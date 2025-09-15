package main

import (
	"chrome-migrator/compressor"
	"chrome-migrator/config"
	"chrome-migrator/detector"
	"chrome-migrator/extractor"
	"chrome-migrator/restorer"
	"chrome-migrator/ui"
	"chrome-migrator/utils"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	logger, err := utils.NewLogger()
	if err != nil {
		os.Exit(1)
	}
	defer logger.Close()

	logger.Info("浏览器数据迁移工具启动")
	cfg := config.DefaultConfig()
	
	if err := ensureDirectories(cfg); err != nil {
		logger.Error("创建必要目录失败: %v", err)
		os.Exit(1)
	}

	uiInstance := ui.NewUI()
	uiInstance.ShowWelcome()
	menuChoice := uiInstance.ShowMainMenu()

	switch menuChoice {
	case 1:
		handleBackup(uiInstance, cfg, logger)
	case 2:
		handleRestore(uiInstance, logger)
	case 3:
		fmt.Println("程序已退出")
		return
	}

	uiInstance.WaitForExit()
}


func handleBackup(uiInstance *ui.UI, cfg *config.Config, logger *utils.Logger) {
	browserType := uiInstance.ShowBrowserOptions()
	cfg.BrowserType = browserType

	browsers, err := detector.DetectBrowsers(browserType)
	if err != nil {
		uiInstance.ShowError(fmt.Sprintf("检测浏览器失败: %v", err))
		logger.Error("检测浏览器失败: %v", err)
		return
	}

	if len(browsers) == 0 {
		uiInstance.ShowError("未找到任何浏览器")
		logger.Error("未找到任何浏览器")
		return
	}

	var outputPaths []string

	for _, browser := range browsers {
		uiInstance.ShowBrowserInfo(browser.Name, browser.InstallPath, browser.UserDataDir, browser.Profiles)
		logger.Info("%s检测成功，安装路径: %s", browser.Name, browser.InstallPath)
		logger.Info("用户数据目录: %s", browser.UserDataDir)
		logger.Info("找到配置文件: %v", browser.Profiles)

		if browser.IsRunning {
			if !uiInstance.ConfirmKillProcess(browser.Name) {
				uiInstance.ShowInfo("用户取消操作")
				continue
			}

			logger.Info("检测到%s正在运行，尝试关闭...", browser.Name)
			killedCount, err := browser.KillProcesses()
			if err != nil {
				uiInstance.ShowWarning(fmt.Sprintf("关闭%s进程时出现警告: %v", browser.Name, err))
				logger.Warning("关闭%s进程时出现警告: %v", browser.Name, err)
			} else {
				uiInstance.ShowProcessKilled(browser.Name, killedCount)
			}
			time.Sleep(3 * time.Second)
		}

		outputPath, err := processBrowser(browser, cfg, uiInstance, logger)
		if err != nil {
			uiInstance.ShowError(fmt.Sprintf("处理%s失败: %v", browser.Name, err))
			logger.Error("处理%s失败: %v", browser.Name, err)
			continue
		}

		if outputPath != "" {
			outputPaths = append(outputPaths, outputPath)
		}
	}

	if len(outputPaths) > 0 {
		uiInstance.ShowRestoreInstructions(outputPaths)
		logger.Info("浏览器数据迁移完成！")
	} else {
		uiInstance.ShowError("没有成功备份任何浏览器数据")
		logger.Error("没有成功备份任何浏览器数据")
	}
}


func handleRestore(uiInstance *ui.UI, logger *utils.Logger) {
	browserType := uiInstance.ShowRestoreBrowserOptions()
	dataRestorer := restorer.NewDataRestorer()

	targetDir, err := dataRestorer.GetTargetDirectory(browserType)
	if err != nil {
		logger.Error("获取目标目录失败: %v", err)
		uiInstance.ShowError(fmt.Sprintf("错误: %v", err))
		return
	}

	uiInstance.ShowInfo(fmt.Sprintf("目标还原路径: %s", targetDir))
	backupFilePath := uiInstance.GetBackupFilePath()
	uiInstance.ShowRestoreWarning()

	dataRestorer.SetProgressCallback(func(current int64, message string) {
		uiInstance.ShowRestoreProgress(current, message)
	})

	uiInstance.ShowInfo("开始还原数据...")
	if err := dataRestorer.RestoreData(backupFilePath, browserType, uiInstance); err != nil {
		logger.Error("还原数据失败: %v", err)
		uiInstance.ShowError(fmt.Sprintf("还原失败: %v", err))
		return
	}

	fmt.Println()
	uiInstance.ShowInfo("数据还原完成！")
	uiInstance.ShowInfo("请重新启动浏览器以使用还原的数据。")
	logger.Info("数据还原完成")
}

func processBrowser(browser *detector.BrowserInfo, cfg *config.Config, uiInstance *ui.UI, logger *utils.Logger) (string, error) {
	browserTempDir := filepath.Join(cfg.TempDir, browser.Name)
	if err := os.MkdirAll(browserTempDir, 0755); err != nil {
		return "", fmt.Errorf("创建临时目录失败: %v", err)
	}

	dataExtractor := extractor.NewDataExtractor(
		browser.UserDataDir,
		browserTempDir,
		browser.Profiles,
		browser.Name,
	)

	// 一次性获取数据大小和文件数量
	dataSize, totalFiles, err := dataExtractor.GetDataSizeAndCount()
	if err != nil {
		logger.Warning("无法计算%s数据信息: %v", browser.Name, err)
		dataSize = 1024 * 1024 * 1024
		totalFiles = 100
	}

	requiredSpace := dataSize * int64(config.RequiredDiskSpaceMultiplier)
	availableSpace, err := utils.GetAvailableDiskSpace(cfg.OutputDir)
	if err == nil {
		uiInstance.ShowDiskSpaceInfo(requiredSpace, availableSpace)
		if availableSpace < requiredSpace {
			return "", fmt.Errorf("磁盘空间不足")
		}
	}

	compressor := compressor.NewZipCompressor(browserTempDir, browser.Name)

	uiInstance.CreateProgressBar(totalFiles, fmt.Sprintf("正在拷贝 %s 数据...", browser.Name))

	dataExtractor.SetProgressCallback(func(current, total int64, message string) {
		uiInstance.UpdateProgress(current, message)
	})

	logger.Info("开始提取%s数据，预计大小: %s，文件数: %d", browser.Name, utils.FormatBytes(dataSize), totalFiles)

	if err := dataExtractor.ExtractAllData(); err != nil {
		uiInstance.FinishProgress()
		return "", fmt.Errorf("数据提取失败: %v", err)
	}

	uiInstance.FinishProgress()
	logger.Info("%s数据提取完成，开始压缩...", browser.Name)

	compressFiles, err := compressor.CountFilesToCompress()
	if err != nil {
		logger.Warning("无法计算压缩文件数量: %v", err)
		compressFiles = totalFiles
	}

	uiInstance.CreateProgressBar(compressFiles, fmt.Sprintf("正在压缩 %s 数据...", browser.Name))

	compressor.SetProgressCallback(func(current, total int64, message string) {
		uiInstance.UpdateProgress(current, message)
	})

	if err := compressor.CompressData(); err != nil {
		uiInstance.FinishProgress()
		return "", fmt.Errorf("数据压缩失败: %v", err)
	}

	uiInstance.FinishProgress()

	compressedSize, err := compressor.GetCompressedSize()
	if err == nil {
		uiInstance.ShowCompressionInfo(browserTempDir, compressor.GetOutputPath(), dataSize, compressedSize)
		logger.Info("%s压缩完成，输出文件: %s，大小: %s",
			browser.Name,
			compressor.GetOutputPath(),
			utils.FormatBytes(compressedSize))
	}

	if err := compressor.CleanupTemp(); err != nil {
		logger.Warning("清理%s临时文件失败: %v", browser.Name, err)
	}

	return compressor.GetOutputPath(), nil
}

func ensureDirectories(cfg *config.Config) error {
	dirs := []string{
		cfg.OutputDir,
		cfg.TempDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %v", dir, err)
		}
	}

	return nil
}