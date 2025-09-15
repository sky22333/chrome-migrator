package restorer

import (
	"fmt"
	"os"
	"strings"
	"time"

	"chrome-migrator/compressor"
	"chrome-migrator/config"
	"chrome-migrator/detector"
)

type UIInterface interface {
	ConfirmKillBrowser(browserName string) bool
	ShowInfo(message string)
}

type DataRestorer struct {
	compressor       *compressor.ZipCompressor
	progressCallback func(int64, string)
}

func NewDataRestorer() *DataRestorer {
	return &DataRestorer{
		compressor: compressor.NewZipCompressor("", ""),
	}
}

func (dr *DataRestorer) SetProgressCallback(callback func(int64, string)) {
	dr.progressCallback = callback
}

func (dr *DataRestorer) RestoreData(backupFilePath string, browserType config.BrowserType, uiInstance UIInterface) error {
	if err := dr.validateBackupFile(backupFilePath); err != nil {
		return fmt.Errorf("备份文件验证失败: %v", err)
	}

	browserDetector := dr.getBrowserDetector(browserType)
	browserInfo, err := browserDetector.Detect()
	if err != nil {
		return fmt.Errorf("浏览器检测失败: %v", err)
	}

	dataDir := browserInfo.UserDataDir
	if dataDir == "" {
		return fmt.Errorf("无法获取浏览器数据目录")
	}

	if browserInfo.IsRunning {
		if !uiInstance.ConfirmKillBrowser(browserInfo.Name) {
			return fmt.Errorf("用户取消操作，浏览器仍在运行")
		}
		
		killedCount, err := browserInfo.KillProcesses()
		if err != nil {
			return fmt.Errorf("终止浏览器进程失败: %v", err)
		}
		
		if killedCount > 0 {
			uiInstance.ShowInfo(fmt.Sprintf("已终止 %d 个浏览器进程", killedCount))
			time.Sleep(2 * time.Second)
		}
	}

	if err := dr.compressor.ExtractZip(backupFilePath, dataDir, func(current, total int, message string) {
		if dr.progressCallback != nil {
			dr.progressCallback(int64(current), message)
		}
	}); err != nil {
		return fmt.Errorf("解压备份文件失败: %v", err)
	}

	return nil
}

func (dr *DataRestorer) validateBackupFile(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("备份文件路径不能为空")
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在: %s", filePath)
	}

	if !strings.HasSuffix(strings.ToLower(filePath), ".zip") {
		return fmt.Errorf("备份文件必须是ZIP格式")
	}

	return nil
}

func (dr *DataRestorer) getBrowserDetector(browserType config.BrowserType) detector.BrowserDetector {
	return detector.NewBrowserDetector(browserType)
}

func (dr *DataRestorer) GetTargetDirectory(browserType config.BrowserType) (string, error) {
	browserDetector := dr.getBrowserDetector(browserType)
	browserInfo, err := browserDetector.Detect()
	if err != nil {
		return "", fmt.Errorf("检测浏览器失败: %v", err)
	}

	return browserInfo.UserDataDir, nil
}