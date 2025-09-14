package detector

import (
	"chrome-migrator/config"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Windows API constants
const (
	TH32CS_SNAPPROCESS = 0x00000002
	PROCESS_TERMINATE  = 0x0001
)

// Windows API functions
var (
	kernel32                    = windows.NewLazyDLL("kernel32.dll")
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW         = kernel32.NewProc("Process32FirstW")
	procProcess32NextW          = kernel32.NewProc("Process32NextW")
	procCloseHandle             = kernel32.NewProc("CloseHandle")
	procOpenProcess             = kernel32.NewProc("OpenProcess")
	procTerminateProcess        = kernel32.NewProc("TerminateProcess")
)

// PROCESSENTRY32 structure
type PROCESSENTRY32 struct {
	dwSize              uint32
	cntUsage            uint32
	th32ProcessID       uint32
	th32DefaultHeapID   uintptr
	th32ModuleID        uint32
	cntThreads          uint32
	th32ParentProcessID uint32
	pcPriClassBase      int32
	dwFlags             uint32
	szExeFile           [260]uint16
}

type BrowserInfo struct {
	BrowserType config.BrowserType
	Name        string
	InstallPath string
	UserDataDir string
	Profiles    []string
	IsRunning   bool
	ProcessName string
}

func (bi *BrowserInfo) KillProcesses() (int, error) {
	return killProcessesByNameWithCount(bi.ProcessName)
}

type BrowserDetector interface {
	Detect() (*BrowserInfo, error)
	KillProcesses() error
}

type ChromeDetector struct{}
type EdgeDetector struct{}

func NewBrowserDetector(browserType config.BrowserType) BrowserDetector {
	switch browserType {
	case config.BrowserChrome:
		return &ChromeDetector{}
	case config.BrowserEdge:
		return &EdgeDetector{}
	default:
		return &ChromeDetector{}
	}
}

// Chrome检测实现
func (cd *ChromeDetector) Detect() (*BrowserInfo, error) {
	info := &BrowserInfo{
		BrowserType: config.BrowserChrome,
		Name:        "Google Chrome",
		ProcessName: "chrome.exe",
	}

	installPath, err := cd.getInstallPath()
	if err != nil {
		return nil, fmt.Errorf("无法检测Chrome安装路径: %v", err)
	}
	info.InstallPath = installPath

	userDataDir, err := cd.getUserDataDir()
	if err != nil {
		return nil, fmt.Errorf("无法获取Chrome用户数据目录: %v", err)
	}
	info.UserDataDir = userDataDir

	profiles, err := getBrowserProfiles(userDataDir)
	if err != nil {
		return nil, fmt.Errorf("无法获取Chrome配置文件: %v", err)
	}
	info.Profiles = profiles

	info.IsRunning = isProcessRunning("chrome.exe")

	return info, nil
}

func (cd *ChromeDetector) getInstallPath() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Google\Chrome\BLBeacon`, registry.QUERY_VALUE)
	if err != nil {
		key, err = registry.OpenKey(registry.CURRENT_USER, `SOFTWARE\Google\Chrome\BLBeacon`, registry.QUERY_VALUE)
		if err != nil {
			return "", fmt.Errorf("Chrome未安装或无法访问注册表")
		}
	}
	defer key.Close()

	path, _, err := key.GetStringValue("version")
	if err != nil {
		return "", err
	}

	return filepath.Dir(path), nil
}

func (cd *ChromeDetector) getUserDataDir() (string, error) {
	// 使用环境变量的标准路径
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		userDataDir := filepath.Join(localAppData, "Google", "Chrome", "User Data")
		if _, err := os.Stat(userDataDir); err == nil {
			return userDataDir, nil
		}
	}

	// 通过App Paths注册表检测
	if userDataDir, err := getUserDataDirFromAppPaths("chrome"); err == nil {
		return userDataDir, nil
	}

	// 通过Uninstall注册表检测
	if userDataDir, err := getUserDataDirFromUninstall("chrome"); err == nil {
		return userDataDir, nil
	}

	// 文件系统fallback检测
	if userDataDir, err := getUserDataDirFromFileSystem("chrome"); err == nil {
		return userDataDir, nil
	}

	return "", fmt.Errorf("无法检测到Chrome用户数据目录，请确保Chrome已正确安装")
}

func (cd *ChromeDetector) KillProcesses() error {
	return killProcessesByName("chrome.exe")
}

func (ed *EdgeDetector) Detect() (*BrowserInfo, error) {
	info := &BrowserInfo{
		BrowserType: config.BrowserEdge,
		Name:        "Microsoft Edge",
		ProcessName: "msedge.exe",
	}

	installPath, err := ed.getInstallPath()
	if err != nil {
		return nil, fmt.Errorf("无法检测Edge安装路径: %v", err)
	}
	info.InstallPath = installPath

	userDataDir, err := ed.getUserDataDir()
	if err != nil {
		return nil, fmt.Errorf("无法获取Edge用户数据目录: %v", err)
	}
	info.UserDataDir = userDataDir

	profiles, err := getBrowserProfiles(userDataDir)
	if err != nil {
		return nil, fmt.Errorf("无法获取Edge配置文件: %v", err)
	}
	info.Profiles = profiles

	info.IsRunning = isProcessRunning("msedge.exe")

	return info, nil
}

func (ed *EdgeDetector) getInstallPath() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Edge\BLBeacon`, registry.QUERY_VALUE)
	if err != nil {
		key, err = registry.OpenKey(registry.CURRENT_USER, `SOFTWARE\Microsoft\Edge\BLBeacon`, registry.QUERY_VALUE)
		if err != nil {
			return "", fmt.Errorf("Edge未安装或无法访问注册表")
		}
	}
	defer key.Close()

	path, _, err := key.GetStringValue("version")
	if err != nil {
		return "", err
	}

	return filepath.Dir(path), nil
}

func (ed *EdgeDetector) getUserDataDir() (string, error) {
	// 使用环境变量的标准路径
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		userDataDir := filepath.Join(localAppData, "Microsoft", "Edge", "User Data")
		if _, err := os.Stat(userDataDir); err == nil {
			return userDataDir, nil
		}
	}

	// 通过App Paths注册表检测
	if userDataDir, err := getUserDataDirFromAppPaths("edge"); err == nil {
		return userDataDir, nil
	}

	// 通过Uninstall注册表检测
	if userDataDir, err := getUserDataDirFromUninstall("edge"); err == nil {
		return userDataDir, nil
	}

	// 文件系统fallback检测
	if userDataDir, err := getUserDataDirFromFileSystem("edge"); err == nil {
		return userDataDir, nil
	}

	return "", fmt.Errorf("无法检测到Edge用户数据目录，请确保Edge已正确安装")
}

func (ed *EdgeDetector) KillProcesses() error {
	return killProcessesByName("msedge.exe")
}

func getBrowserProfiles(userDataDir string) ([]string, error) {
	var profiles []string

	defaultProfile := filepath.Join(userDataDir, "Default")
	if _, err := os.Stat(defaultProfile); err == nil {
		profiles = append(profiles, "Default")
	}

	entries, err := os.ReadDir(userDataDir)
	if err != nil {
		return profiles, nil
	}

	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) > 7 && entry.Name()[:7] == "Profile" {
			profiles = append(profiles, entry.Name())
		}
	}

	if len(profiles) == 0 {
		return nil, fmt.Errorf("未找到任何配置文件")
	}

	return profiles, nil
}

func isProcessRunning(processName string) bool {
	handle, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if handle == uintptr(syscall.InvalidHandle) {
		return false
	}
	defer procCloseHandle.Call(handle)

	var pe PROCESSENTRY32
	pe.dwSize = uint32(unsafe.Sizeof(pe))

	ret, _, _ := procProcess32FirstW.Call(handle, uintptr(unsafe.Pointer(&pe)))
	if ret == 0 {
		return false
	}

	for {
		currentProcessName := syscall.UTF16ToString(pe.szExeFile[:])
		if currentProcessName == processName {
			return true
		}

		ret, _, _ := procProcess32NextW.Call(handle, uintptr(unsafe.Pointer(&pe)))
		if ret == 0 {
			break
		}
	}

	return false
}

func killProcessesByName(processName string) error {
	_, err := killProcessesByNameWithCount(processName)
	return err
}

func killProcessesByNameWithCount(processName string) (int, error) {
	handle, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if handle == uintptr(syscall.InvalidHandle) {
		return 0, fmt.Errorf("无法创建进程快照")
	}
	defer procCloseHandle.Call(handle)

	var pe PROCESSENTRY32
	pe.dwSize = uint32(unsafe.Sizeof(pe))

	ret, _, _ := procProcess32FirstW.Call(handle, uintptr(unsafe.Pointer(&pe)))
	if ret == 0 {
		return 0, fmt.Errorf("无法枚举进程")
	}

	var killedCount int
	for {
		currentProcessName := syscall.UTF16ToString(pe.szExeFile[:])
		if currentProcessName == processName {
			processHandle, _, _ := procOpenProcess.Call(PROCESS_TERMINATE, 0, uintptr(pe.th32ProcessID))
			if processHandle != 0 {
				procTerminateProcess.Call(processHandle, 0)
				procCloseHandle.Call(processHandle)
				killedCount++
			}
		}

		ret, _, _ := procProcess32NextW.Call(handle, uintptr(unsafe.Pointer(&pe)))
		if ret == 0 {
			break
		}
	}

	return killedCount, nil
}

// 检测多个浏览器
func DetectBrowsers(browserType config.BrowserType) ([]*BrowserInfo, error) {
	var browsers []*BrowserInfo

	switch browserType {
	case config.BrowserChrome:
		detector := NewBrowserDetector(config.BrowserChrome)
		info, err := detector.Detect()
		if err != nil {
			return nil, err
		}
		browsers = append(browsers, info)

	case config.BrowserEdge:
		detector := NewBrowserDetector(config.BrowserEdge)
		info, err := detector.Detect()
		if err != nil {
			return nil, err
		}
		browsers = append(browsers, info)

	case config.BrowserBoth:
		// 检测Chrome
		chromeDetector := NewBrowserDetector(config.BrowserChrome)
		if chromeInfo, err := chromeDetector.Detect(); err == nil {
			browsers = append(browsers, chromeInfo)
		}

		// 检测Edge
		edgeDetector := NewBrowserDetector(config.BrowserEdge)
		if edgeInfo, err := edgeDetector.Detect(); err == nil {
			browsers = append(browsers, edgeInfo)
		}

		if len(browsers) == 0 {
			return nil, fmt.Errorf("未检测到任何浏览器")
		}
	}

	return browsers, nil
}

// 通过App Paths注册表检测浏览器用户数据目录
func getUserDataDirFromAppPaths(browserName string) (string, error) {
	var exeName string
	switch browserName {
	case "chrome":
		exeName = "chrome.exe"
	case "edge":
		exeName = "msedge.exe"
	default:
		return "", fmt.Errorf("不支持的浏览器: %s", browserName)
	}

	// 尝试从HKLM App Paths获取
	if path, err := getAppPathFromRegistry(registry.LOCAL_MACHINE, exeName); err == nil {
		if userDataDir := deriveUserDataDirFromExePath(path, browserName); userDataDir != "" {
			return userDataDir, nil
		}
	}

	// 尝试从HKLM WOW6432Node App Paths获取（32位应用）
	if path, err := getAppPathFromRegistry(registry.LOCAL_MACHINE, exeName, "WOW6432Node"); err == nil {
		if userDataDir := deriveUserDataDirFromExePath(path, browserName); userDataDir != "" {
			return userDataDir, nil
		}
	}

	return "", fmt.Errorf("无法从App Paths注册表获取%s用户数据目录", browserName)
}

// 从注册表App Paths获取应用程序路径
func getAppPathFromRegistry(baseKey registry.Key, exeName string, subPaths ...string) (string, error) {
	keyPath := `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\` + exeName
	if len(subPaths) > 0 {
		keyPath = `SOFTWARE\` + subPaths[0] + `\Microsoft\Windows\CurrentVersion\App Paths\` + exeName
	}

	key, err := registry.OpenKey(baseKey, keyPath, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()

	path, _, err := key.GetStringValue("")
	if err != nil {
		return "", err
	}

	return path, nil
}

// 通过Uninstall注册表检测浏览器用户数据目录
func getUserDataDirFromUninstall(browserName string) (string, error) {
	var displayNames []string
	switch browserName {
	case "chrome":
		displayNames = []string{"Google Chrome", "Chrome"}
	case "edge":
		displayNames = []string{"Microsoft Edge", "Edge"}
	default:
		return "", fmt.Errorf("不支持的浏览器: %s", browserName)
	}

	// 检查HKLM Uninstall
	if installPath, err := getInstallPathFromUninstall(registry.LOCAL_MACHINE, displayNames); err == nil {
		if userDataDir := deriveUserDataDirFromInstallPath(installPath, browserName); userDataDir != "" {
			return userDataDir, nil
		}
	}

	// 检查HKLM WOW6432Node Uninstall（32位应用）
	if installPath, err := getInstallPathFromUninstall(registry.LOCAL_MACHINE, displayNames, "WOW6432Node"); err == nil {
		if userDataDir := deriveUserDataDirFromInstallPath(installPath, browserName); userDataDir != "" {
			return userDataDir, nil
		}
	}

	// 检查HKCU Uninstall
	if installPath, err := getInstallPathFromUninstall(registry.CURRENT_USER, displayNames); err == nil {
		if userDataDir := deriveUserDataDirFromInstallPath(installPath, browserName); userDataDir != "" {
			return userDataDir, nil
		}
	}

	return "", fmt.Errorf("无法从Uninstall注册表获取%s用户数据目录", browserName)
}

// 从Uninstall注册表获取安装路径
func getInstallPathFromUninstall(baseKey registry.Key, displayNames []string, subPaths ...string) (string, error) {
	uninstallPath := `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`
	if len(subPaths) > 0 {
		uninstallPath = `SOFTWARE\` + subPaths[0] + `\Microsoft\Windows\CurrentVersion\Uninstall`
	}

	key, err := registry.OpenKey(baseKey, uninstallPath, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return "", err
	}
	defer key.Close()

	subKeys, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return "", err
	}

	for _, subKey := range subKeys {
		subKeyPath := uninstallPath + `\` + subKey
		appKey, err := registry.OpenKey(baseKey, subKeyPath, registry.QUERY_VALUE)
		if err != nil {
			continue
		}

		displayName, _, err := appKey.GetStringValue("DisplayName")
		if err != nil {
			appKey.Close()
			continue
		}

		// 检查是否匹配目标浏览器
		for _, targetName := range displayNames {
			if strings.Contains(strings.ToLower(displayName), strings.ToLower(targetName)) {
				installLocation, _, err := appKey.GetStringValue("InstallLocation")
				if err == nil && installLocation != "" {
					appKey.Close()
					return installLocation, nil
				}
				uninstallString, _, err := appKey.GetStringValue("UninstallString")
				if err == nil && uninstallString != "" {
					appKey.Close()
					return filepath.Dir(uninstallString), nil
				}
				break
			}
		}
		appKey.Close()
	}

	return "", fmt.Errorf("未找到匹配的应用程序")
}

// 从可执行文件路径推导用户数据目录
func deriveUserDataDirFromExePath(exePath, browserName string) string {
	if exePath == "" {
		return ""
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return ""
	}

	var userDataDir string
	switch browserName {
	case "chrome":
		userDataDir = filepath.Join(localAppData, "Google", "Chrome", "User Data")
	case "edge":
		userDataDir = filepath.Join(localAppData, "Microsoft", "Edge", "User Data")
	default:
		return ""
	}

	if _, err := os.Stat(userDataDir); err == nil {
		return userDataDir
	}

	return ""
}

// 从安装路径推导用户数据目录
func deriveUserDataDirFromInstallPath(installPath, browserName string) string {
	return deriveUserDataDirFromExePath(installPath, browserName)
}

// 文件系统fallback检测方案
func getUserDataDirFromFileSystem(browserName string) (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return "", fmt.Errorf("无法获取LOCALAPPDATA环境变量")
	}

	var candidatePaths []string
	switch browserName {
	case "chrome":
		candidatePaths = []string{
			filepath.Join(localAppData, "Google", "Chrome", "User Data"),
			filepath.Join(localAppData, "Chromium", "User Data"),
			filepath.Join(localAppData, "Google(x86)", "Chrome", "User Data"),
		}
	case "edge":
		candidatePaths = []string{
			filepath.Join(localAppData, "Microsoft", "Edge", "User Data"),
			filepath.Join(localAppData, "Microsoft", "Edge Dev", "User Data"),
			filepath.Join(localAppData, "Microsoft", "Edge Beta", "User Data"),
		}
	default:
		return "", fmt.Errorf("不支持的浏览器: %s", browserName)
	}

	// 检查每个候选路径
	for _, path := range candidatePaths {
		if _, err := os.Stat(path); err == nil {
			if isValidUserDataDir(path) {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("无法通过文件系统检测找到%s用户数据目录", browserName)
}

// 验证是否为有效的用户数据目录
func isValidUserDataDir(path string) bool {
	defaultProfile := filepath.Join(path, "Default")
	if _, err := os.Stat(defaultProfile); err == nil {
		return true
	}

	localState := filepath.Join(path, "Local State")
	if _, err := os.Stat(localState); err == nil {
		return true
	}

	return false
}