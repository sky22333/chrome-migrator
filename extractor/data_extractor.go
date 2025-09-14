package extractor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type DataExtractor struct {
	UserDataDir   string
	OutputDir     string
	Profiles      []string
	BrowserName   string
	ProgressCallback func(current, total int64, message string)
	totalFiles    int64
	processedFiles int64
	workerCount   int
	progressMutex sync.Mutex
	lastProgressUpdate time.Time
}

// FileTask 表示一个文件复制任务
type FileTask struct {
	SrcPath     string
	DstPath     string
	Size        int64
	BaseMessage string
}

// 文件大小阈值：1MB
const LargeFileThreshold = 1024 * 1024

var (
	kernel32         = windows.NewLazyDLL("kernel32.dll")
	procCopyFileW    = kernel32.NewProc("CopyFileW")
	procCreateDirectoryW = kernel32.NewProc("CreateDirectoryW")
	procGetLastError = kernel32.NewProc("GetLastError")
)

var criticalFiles = []string{
	"History",
	"Bookmarks",
	"Login Data",
	"Cookies",
	"Preferences",
	"Current Session",
	"Current Tabs",
	"Last Session",
	"Last Tabs",
	"Web Data",
	"Favicons",
	"Top Sites",
	"Network Action Predictor",
	"Shortcuts",
	"TransportSecurity",
}

var criticalDirs = []string{
	"Extensions",
	"Local Storage",
	"Session Storage",
	"IndexedDB",
}

func NewDataExtractor(userDataDir, outputDir string, profiles []string, browserName string) *DataExtractor {
	// 根据CPU核心数设置工作线程数，最大不超过8个
	workerCount := runtime.NumCPU()
	if workerCount > 8 {
		workerCount = 8
	}
	if workerCount < 2 {
		workerCount = 2
	}
	
	return &DataExtractor{
		UserDataDir: userDataDir,
		OutputDir:   outputDir,
		Profiles:    profiles,
		BrowserName: browserName,
		workerCount: workerCount,
	}
}

func (e *DataExtractor) SetProgressCallback(callback func(current, total int64, message string)) {
	e.ProgressCallback = callback
}

// CountTotalFiles 计算需要处理的总文件数
func (e *DataExtractor) CountTotalFiles() (int64, error) {
	var totalFiles int64
	
	for _, profile := range e.Profiles {
		profileDir := filepath.Join(e.UserDataDir, profile)
		
		for _, file := range criticalFiles {
			filePath := filepath.Join(profileDir, file)
			if _, err := os.Stat(filePath); err == nil {
				totalFiles++
			}
		}
		
		for _, dir := range criticalDirs {
			dirPath := filepath.Join(profileDir, dir)
			count, _ := e.countFilesInDir(dirPath)
			totalFiles += count
		}
	}
	
	for _, file := range criticalFiles {
		filePath := filepath.Join(e.UserDataDir, file)
		if _, err := os.Stat(filePath); err == nil {
			totalFiles++
		}
	}
	
	e.totalFiles = totalFiles
	return totalFiles, nil
}

// countFilesInDir 计算目录中的文件数量
func (e *DataExtractor) countFilesInDir(dir string) (int64, error) {
	var count int64
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误，继续计数
		}
		if !info.IsDir() && !e.shouldSkipFile(path) {
			count++
		}
		return nil
	})
	return count, err
}

// updateProgress 更新进度
func (e *DataExtractor) updateProgress(message string) {
	atomic.AddInt64(&e.processedFiles, 1)
	e.updateProgressWithThrottle(message)
}

// updateProgressWithThrottle 带频率限制的进度更新
func (e *DataExtractor) updateProgressWithThrottle(message string) {
	e.progressMutex.Lock()
	defer e.progressMutex.Unlock()
	
	now := time.Now()
	if now.Sub(e.lastProgressUpdate) < 100*time.Millisecond {
		return
	}
	e.lastProgressUpdate = now
	
	if e.ProgressCallback != nil {
		current := atomic.LoadInt64(&e.processedFiles)
		e.ProgressCallback(current, e.totalFiles, message)
	}
}

// forceUpdateProgress 强制更新进度
func (e *DataExtractor) forceUpdateProgress(message string) {
	e.progressMutex.Lock()
	defer e.progressMutex.Unlock()
	
	if e.ProgressCallback != nil {
		current := atomic.LoadInt64(&e.processedFiles)
		e.ProgressCallback(current, e.totalFiles, message)
	}
	e.lastProgressUpdate = time.Now()
}

func (e *DataExtractor) ExtractAllData() error {
	if err := e.createOutputDir(); err != nil {
		return fmt.Errorf("创建输出目录失败: %v", err)
	}

	e.processedFiles = 0

	for _, profile := range e.Profiles {
		profileDir := filepath.Join(e.UserDataDir, profile)
		outputProfileDir := filepath.Join(e.OutputDir, profile)

		if err := e.createDir(outputProfileDir); err != nil {
			return fmt.Errorf("创建配置文件输出目录失败: %v", err)
		}

		if err := e.extractProfileData(profileDir, outputProfileDir, profile); err != nil {
			return fmt.Errorf("提取配置文件 %s 数据失败: %v", profile, err)
		}
	}

	if err := e.extractGlobalData(); err != nil {
		return fmt.Errorf("提取全局数据失败: %v", err)
	}

	return nil
}

func (e *DataExtractor) createOutputDir() error {
	return e.createDir(e.OutputDir)
}

func (e *DataExtractor) createDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		dirPtr, err := syscall.UTF16PtrFromString(dir)
		if err != nil {
			return err
		}

		ret, _, _ := procCreateDirectoryW.Call(uintptr(unsafe.Pointer(dirPtr)), 0)
		if ret == 0 {
			errno, _, _ := procGetLastError.Call()
			return fmt.Errorf("创建目录失败，错误代码: %d", errno)
		}
	}
	return nil
}

func (e *DataExtractor) extractProfileData(profileDir, outputDir, profileName string) error {
	// 复制关键文件
	for _, filename := range criticalFiles {
		srcPath := filepath.Join(profileDir, filename)
		dstPath := filepath.Join(outputDir, filename)

		if _, err := os.Stat(srcPath); err == nil {
			if err := e.copyFileWithRetry(srcPath, dstPath); err == nil {
				e.updateProgress(fmt.Sprintf("正在复制%s配置文件: %s", e.BrowserName, filename))
			}
		}
	}

	// 复制关键目录
	for _, dirname := range criticalDirs {
		srcDir := filepath.Join(profileDir, dirname)
		dstDir := filepath.Join(outputDir, dirname)

		if _, err := os.Stat(srcDir); err == nil {
			if err := e.copyDirRecursiveWithProgress(srcDir, dstDir, fmt.Sprintf("正在复制%s配置文件目录: %s", e.BrowserName, dirname)); err != nil {
				continue
			}
		}
	}

	return nil
}

func (e *DataExtractor) extractGlobalData() error {
	globalFiles := []string{
		"Local State",
		"First Run",
		"chrome_shutdown_ms.txt",
	}

	// 复制全局文件
	for _, filename := range globalFiles {
		srcPath := filepath.Join(e.UserDataDir, filename)
		dstPath := filepath.Join(e.OutputDir, filename)

		if _, err := os.Stat(srcPath); err == nil {
			if err := e.copyFileWithRetry(srcPath, dstPath); err == nil {
				e.updateProgress(fmt.Sprintf("正在复制%s全局文件: %s", e.BrowserName, filename))
			}
		}
	}

	globalDirs := []string{
		"CertificateTransparency",
		"InterventionPolicyDatabase",
		"OptimizationHints",
	}

	// 复制全局目录
	for _, dirname := range globalDirs {
		srcDir := filepath.Join(e.UserDataDir, dirname)
		dstDir := filepath.Join(e.OutputDir, dirname)

		if _, err := os.Stat(srcDir); err == nil {
			e.copyDirRecursiveWithProgress(srcDir, dstDir, fmt.Sprintf("正在复制%s全局目录: %s", e.BrowserName, dirname))
		}
	}

	return nil
}

func (e *DataExtractor) copyFileWithRetry(src, dst string) error {
	const maxRetries = 3
	const retryDelay = time.Second

	for i := 0; i < maxRetries; i++ {
		if err := e.copyFile(src, dst); err == nil {
			return nil
		}

		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return e.fallbackCopy(src, dst)
}

func (e *DataExtractor) copyFile(src, dst string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("源文件不存在: %s", src)
	}

	srcPtr, err := syscall.UTF16PtrFromString(src)
	if err != nil {
		return err
	}

	dstPtr, err := syscall.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}

	ret, _, _ := procCopyFileW.Call(
		uintptr(unsafe.Pointer(srcPtr)),
		uintptr(unsafe.Pointer(dstPtr)),
		0,
	)

	if ret == 0 {
		errno, _, _ := procGetLastError.Call()
		return fmt.Errorf("CopyFile失败，错误代码: %d", errno)
	}

	return nil
}

func (e *DataExtractor) fallbackCopy(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	return os.Chmod(dst, sourceInfo.Mode())
}

func (e *DataExtractor) copyDirRecursive(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := e.copyDirRecursive(srcPath, dstPath); err != nil {
				continue
			}
		} else {
			if err := e.copyFileWithRetry(srcPath, dstPath); err != nil {
				continue
			}
		}
	}

	return nil
}

// copyDirRecursiveWithProgress 递归复制目录并更新进度
func (e *DataExtractor) copyDirRecursiveWithProgress(src, dst, baseMessage string) error {
	// 收集所有需要复制的文件任务
	var tasks []FileTask
	var dirs []string
	
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if e.shouldSkipFile(path) {
			return nil
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return nil
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			dirs = append(dirs, dstPath)
		} else {
			tasks = append(tasks, FileTask{
				SrcPath:     path,
				DstPath:     dstPath,
				Size:        info.Size(),
				BaseMessage: baseMessage,
			})
		}

		return nil
	})
	
	if err != nil {
		return err
	}
	
	for _, dir := range dirs {
		if err := e.createDir(dir); err != nil {
			return err
		}
	}
	
	return e.copyFilesConcurrently(tasks)
}

func (e *DataExtractor) GetDataSize() (int64, error) {
	var totalSize int64

	for _, profile := range e.Profiles {
		profileDir := filepath.Join(e.UserDataDir, profile)
		size, _ := e.calculateDirSize(profileDir)
		totalSize += size
	}

	globalSize, _ := e.calculateDirSize(e.UserDataDir)
	totalSize += globalSize / 10

	return totalSize, nil
}

func (e *DataExtractor) calculateDirSize(dir string) (int64, error) {
	var size int64

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && !e.shouldSkipFile(path) {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// copyFilesConcurrently 并发复制文件
func (e *DataExtractor) copyFilesConcurrently(tasks []FileTask) error {
	if len(tasks) == 0 {
		return nil
	}
	
	var smallFiles, largeFiles []FileTask
	for _, task := range tasks {
		if task.Size > LargeFileThreshold {
			largeFiles = append(largeFiles, task)
		} else {
			smallFiles = append(smallFiles, task)
		}
	}
	
	var wg sync.WaitGroup
	errorChan := make(chan error, len(tasks))
	
	if len(smallFiles) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := e.processSmallFilesConcurrently(smallFiles, errorChan); err != nil {
				errorChan <- err
			}
		}()
	}
	
	// 串行处理大文件（避免I/O竞争）
	if len(largeFiles) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, task := range largeFiles {
				if err := e.copyLargeFileWithProgress(task); err != nil {
					errorChan <- err
					return
				}
			}
		}()
	}
	
	wg.Wait()
	close(errorChan)
	
	for err := range errorChan {
		if err != nil {
			return err
		}
	}
	
	return nil
}

// processSmallFilesConcurrently 并发处理小文件
func (e *DataExtractor) processSmallFilesConcurrently(tasks []FileTask, errorChan chan<- error) error {
	taskChan := make(chan FileTask, len(tasks))
	var wg sync.WaitGroup
	
	// 启动工作协程
	for i := 0; i < e.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				if err := e.copyFileWithRetry(task.SrcPath, task.DstPath); err != nil {
					errorChan <- fmt.Errorf("复制文件失败 %s: %v", task.SrcPath, err)
					return
				}
				filename := filepath.Base(task.SrcPath)
				e.updateProgress(fmt.Sprintf("%s: %s", task.BaseMessage, filename))
			}
		}()
	}
	for _, task := range tasks {
		taskChan <- task
	}
	close(taskChan)
	
	wg.Wait()
	return nil
}

// copyLargeFileWithProgress 复制大文件并显示字节级进度
func (e *DataExtractor) copyLargeFileWithProgress(task FileTask) error {
	filename := filepath.Base(task.SrcPath)
	message := fmt.Sprintf("%s: %s (大文件)", task.BaseMessage, filename)
	
	e.forceUpdateProgress(fmt.Sprintf("%s - 开始复制", message))
	
	if err := e.copyFileWithRetry(task.SrcPath, task.DstPath); err != nil {
		return fmt.Errorf("复制大文件失败 %s: %v", task.SrcPath, err)
	}
	
	e.updateProgress(fmt.Sprintf("%s - 完成", message))
	return nil
}

func (e *DataExtractor) shouldSkipFile(path string) bool {
	skipPatterns := []string{
		"LOG",
		"LOCK",
		".tmp",
		".log",
		"chrome_debug.log",
	}

	filename := filepath.Base(path)
	for _, pattern := range skipPatterns {
		if strings.Contains(filename, pattern) {
			return true
		}
	}

	return false
}
