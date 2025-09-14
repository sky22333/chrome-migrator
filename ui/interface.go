package ui

import (
	"chrome-migrator/config"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/schollz/progressbar/v3"
)

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Margin(1, 0)

	optionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Padding(0, 1)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Padding(0, 1)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFAA00")).
			Padding(0, 1)
)

type UI struct {
	progressBar *progressbar.ProgressBar
}

func NewUI() *UI {
	return &UI{}
}

func (ui *UI) ShowWelcome() {
	fmt.Println(titleStyle.Render("Chrome/Edge 数据备份迁移工具"))
	fmt.Println()
	fmt.Println("本工具可以帮助您备份浏览器数据，包括：")
	fmt.Println("• 浏览历史记录")
	fmt.Println("• 书签和收藏夹")
	fmt.Println("• 保存的密码")
	fmt.Println("• Cookie和网站数据")
	fmt.Println("• 扩展程序")
	fmt.Println("• 用户偏好设置")
	fmt.Println()
}

func (ui *UI) ShowBrowserOptions() config.BrowserType {
	fmt.Println(optionStyle.Render("请选择要备份的浏览器："))
	fmt.Println()
	fmt.Println("1. Chrome 和 Edge都备份")
	fmt.Println("2. 备份 Microsoft Edge")
	fmt.Println("3. 备份 Google Chrome")
	fmt.Println()

	for {
		fmt.Print("请输入选项 (1-3): ")
		var input string
		fmt.Scanln(&input)

		switch strings.TrimSpace(input) {
		case "1":
			return config.BrowserBoth
		case "2":
			return config.BrowserEdge
		case "3":
			return config.BrowserChrome
		default:
			fmt.Println(errorStyle.Render("无效选项，请输入 1、2 或 3"))
			continue
		}
	}
}

func (ui *UI) ShowBrowserInfo(browserName, installPath, userDataDir string, profiles []string) {
	fmt.Printf("\n%s\n", successStyle.Render(fmt.Sprintf("检测到 %s:", browserName)))
	fmt.Printf("安装路径: %s\n", installPath)
	fmt.Printf("用户数据目录: %s\n", userDataDir)
	fmt.Printf("找到配置文件: %v\n", profiles)
}

func (ui *UI) ConfirmKillProcess(browserName string) bool {
	fmt.Printf("\n%s\n", warningStyle.Render(fmt.Sprintf("检测到 %s 正在运行", browserName)))
	fmt.Println("为了确保数据完整性，程序会自动关闭浏览器进程，备份期间请不要打开浏览器。")
	fmt.Print("是否关闭浏览器？按回车键关闭，输入 'n' 取消: ")

	var input string
	fmt.Scanln(&input)

	return strings.ToLower(strings.TrimSpace(input)) != "n"
}

func (ui *UI) ShowProcessKilled(browserName string, count int) {
	if count > 0 {
		fmt.Printf("%s\n", successStyle.Render(fmt.Sprintf("已关闭 %d 个 %s 进程", count, browserName)))
	} else {
		fmt.Printf("%s\n", successStyle.Render(fmt.Sprintf("%s 进程已关闭", browserName)))
	}
}

func (ui *UI) ShowError(message string) {
	fmt.Printf("%s\n", errorStyle.Render(fmt.Sprintf("错误: %s", message)))
}

func (ui *UI) ShowWarning(message string) {
	fmt.Printf("%s\n", warningStyle.Render(fmt.Sprintf("警告: %s", message)))
}

func (ui *UI) ShowSuccess(message string) {
	fmt.Printf("%s\n", successStyle.Render(message))
}

func (ui *UI) ShowInfo(message string) {
	fmt.Println(message)
}

func (ui *UI) CreateProgressBar(max int64, description string) {
	ui.progressBar = progressbar.NewOptions64(max,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWidth(50),
		progressbar.OptionUseANSICodes(true),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSetRenderBlankState(false),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "█",
			SaucerHead:    "█",
			SaucerPadding: "░",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
}

func (ui *UI) UpdateProgress(current int64, message string) {
	if ui.progressBar != nil {
		ui.progressBar.Set64(current)
	}
}

func (ui *UI) FinishProgress() {
	if ui.progressBar != nil {
		ui.progressBar.Finish()
		ui.progressBar = nil
	}
}

func (ui *UI) ShowDiskSpaceInfo(required, available int64) {
	fmt.Printf("\n磁盘空间检查:\n")
	fmt.Printf("需要空间: %s\n", formatBytes(required))
	fmt.Printf("可用空间: %s\n", formatBytes(available))
	if available >= required {
		fmt.Printf("%s\n", successStyle.Render("磁盘空间充足"))
	} else {
		fmt.Printf("%s\n", errorStyle.Render("磁盘空间不足"))
	}
}

func (ui *UI) ShowCompressionInfo(inputPath, outputPath string, originalSize, compressedSize int64) {
	fmt.Printf("\n压缩完成:\n")
	fmt.Printf("输出文件: %s\n", outputPath)
	fmt.Printf("原始大小: %s\n", formatBytes(originalSize))
	fmt.Printf("压缩后大小: %s\n", formatBytes(compressedSize))
	if originalSize > 0 {
		compressionRatio := float64(compressedSize) / float64(originalSize) * 100
		fmt.Printf("压缩率: %.1f%%\n", compressionRatio)
	}
}

func (ui *UI) ShowRestoreInstructions(outputPaths []string) {
	fmt.Printf("\n%s\n", titleStyle.Render("备份完成！"))
	fmt.Println()
	fmt.Println("备份文件已保存到:")
	for _, path := range outputPaths {
		fmt.Printf("• %s\n", path)
	}
	fmt.Println()
	fmt.Println(successStyle.Render("数据还原说明:"))
	fmt.Println("1. 将压缩的备份文件发送到新设备")
	fmt.Println("2. 将文件解压到以下目录并覆盖:")
	
	// 创建路径高亮样式
	pathStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#0066CC")).
		Padding(0, 1).
		Margin(0, 1)
	
	fmt.Printf("   Chrome: %s\n", pathStyle.Render("%LOCALAPPDATA%\\Google\\Chrome\\User Data"))
	fmt.Printf("   Edge: %s\n", pathStyle.Render("%LOCALAPPDATA%\\Microsoft\\Edge\\User Data"))
	fmt.Println("3. 重新启动浏览器")
	fmt.Println()
	fmt.Println(warningStyle.Render("注意: 请确认所有数据都恢复成功后再清理旧设备"))
	fmt.Println()
	fmt.Print("按任意键退出...")
	fmt.Scanln()
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func (ui *UI) WaitForExit() {
	fmt.Print("\n按任意键退出...")
	var input string
	fmt.Scanln(&input)
}
