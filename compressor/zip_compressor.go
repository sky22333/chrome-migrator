package compressor

import (
	"archive/zip"
	"chrome-migrator/config"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type ZipCompressor struct {
	OutputPath       string
	TempDir          string
	BrowserName      string
	ProgressCallback func(current, total int64, message string)
	workerCount      int
	bufferSize       int
}

func NewZipCompressor(tempDir, browserName string) *ZipCompressor {
	timestamp := time.Now().Format("20060102_150405")
	var outputPath string
	if browserName != "" {
		simpleName := simplifyBrowserName(browserName)
		outputPath = fmt.Sprintf("%s\\%s_backup_%s.zip", config.OutputBaseDir, simpleName, timestamp)
	} else {
		outputPath = fmt.Sprintf("%s\\browser_backup_%s.zip", config.OutputBaseDir, timestamp)
	}
	
	return &ZipCompressor{
		OutputPath:  outputPath,
		TempDir:     tempDir,
		BrowserName: browserName,
		workerCount: runtime.NumCPU(),
		bufferSize:  64 * 1024, // 64KB buffer
	}
}


func simplifyBrowserName(browserName string) string {
	switch {
	case strings.Contains(strings.ToLower(browserName), "chrome"):
		return "chrome"
	case strings.Contains(strings.ToLower(browserName), "edge"):
		return "edge"
	default:
		// 去除空格和特殊字符，转为小写
		name := strings.ToLower(browserName)
		name = strings.ReplaceAll(name, " ", "")
		name = strings.ReplaceAll(name, "-", "")
		name = strings.ReplaceAll(name, "_", "")
		return name
	}
}

func (c *ZipCompressor) SetProgressCallback(callback func(current, total int64, message string)) {
	c.ProgressCallback = callback
}

// CountFilesToCompress 计算需要压缩的文件数量
func (c *ZipCompressor) CountFilesToCompress() (int64, error) {
	var totalFiles int64
	err := filepath.Walk(c.TempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误，继续计数
		}
		if !info.IsDir() {
			totalFiles++
		}
		return nil
	})
	return totalFiles, err
}

type fileTask struct {
	path    string
	relPath string
}

func (c *ZipCompressor) CompressData() error {
	if err := c.ensureOutputDir(); err != nil {
		return fmt.Errorf("创建输出目录失败: %v", err)
	}

	// 收集所有文件
	var files []fileTask
	filepath.Walk(c.TempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(c.TempDir, path)
		if err != nil {
			return nil
		}
		files = append(files, fileTask{
			path:    path,
			relPath: strings.ReplaceAll(relPath, "\\", "/"),
		})
		return nil
	})

	zipFile, err := os.Create(c.OutputPath)
	if err != nil {
		return fmt.Errorf("创建zip文件失败: %v", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 并发处理文件
	return c.compressFilesConcurrently(zipWriter, files)
}

func (c *ZipCompressor) ensureOutputDir() error {
	outputDir := filepath.Dir(c.OutputPath)
	return os.MkdirAll(outputDir, 0755)
}

func (c *ZipCompressor) compressFilesConcurrently(zipWriter *zip.Writer, files []fileTask) error {
	totalFiles := int64(len(files))
	var processedFiles int64
	var mu sync.Mutex

	// 创建工作队列
	fileChan := make(chan fileTask, c.workerCount*2)
	errorChan := make(chan error, c.workerCount)
	var wg sync.WaitGroup

	// 启动工作协程
	for i := 0; i < c.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buffer := make([]byte, c.bufferSize)
			for task := range fileChan {
				if err := c.addFileToZip(zipWriter, task.path, task.relPath, buffer, &mu); err != nil {
					select {
					case errorChan <- err:
					default:
					}
					continue
				}
				
				mu.Lock()
				processedFiles++
				if c.ProgressCallback != nil {
					message := fmt.Sprintf("正在压缩 %s 数据...", c.BrowserName)
					c.ProgressCallback(processedFiles, totalFiles, message)
				}
				mu.Unlock()
			}
		}()
	}

	// 发送任务
	go func() {
		defer close(fileChan)
		for _, file := range files {
			fileChan <- file
		}
	}()

	wg.Wait()
	close(errorChan)

	// 检查是否有错误
	select {
	case err := <-errorChan:
		return err
	default:
		return nil
	}
}

func (c *ZipCompressor) addFileToZip(zipWriter *zip.Writer, filePath, zipPath string, buffer []byte, mu *sync.Mutex) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	header.Name = zipPath
	header.Method = zip.Deflate

	// 写入zip需要加锁
	mu.Lock()
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		mu.Unlock()
		return err
	}

	// 使用缓冲区优化的流式复制
	_, err = io.CopyBuffer(writer, file, buffer)
	mu.Unlock()
	return err
}

func (c *ZipCompressor) CleanupTemp() error {
	return os.RemoveAll(c.TempDir)
}

func (c *ZipCompressor) GetOutputPath() string {
	return c.OutputPath
}

func (c *ZipCompressor) GetCompressedSize() (int64, error) {
	info, err := os.Stat(c.OutputPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// ExtractZip 解压ZIP文件到指定目录
func (c *ZipCompressor) ExtractZip(zipPath, destDir string, progressCallback func(int, int, string)) error {
	// 打开ZIP文件
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("无法打开ZIP文件: %v", err)
	}
	defer reader.Close()

	totalFiles := len(reader.File)
	processedFiles := 0

	// 创建目标目录
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("无法创建目标目录: %v", err)
	}

	// 解压每个文件
	for _, file := range reader.File {
		if progressCallback != nil {
			progressCallback(processedFiles, totalFiles, fmt.Sprintf("正在解压: %s", file.Name))
		}

		if err := c.extractFile(file, destDir); err != nil {
			return fmt.Errorf("解压文件 %s 失败: %v", file.Name, err)
		}

		processedFiles++
	}

	if progressCallback != nil {
		progressCallback(totalFiles, totalFiles, "解压完成")
	}

	return nil
}

// extractFile 解压单个文件
func (c *ZipCompressor) extractFile(file *zip.File, destDir string) error {
	// 构建目标路径
	destPath := filepath.Join(destDir, file.Name)

	// 确保路径安全，防止目录遍历攻击
	if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
		return fmt.Errorf("不安全的文件路径: %s", file.Name)
	}

	// 如果是目录，创建目录
	if file.FileInfo().IsDir() {
		return os.MkdirAll(destPath, file.FileInfo().Mode())
	}

	// 创建父目录
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// 打开ZIP中的文件
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// 创建目标文件
	destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.FileInfo().Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	// 复制文件内容
	_, err = io.Copy(destFile, rc)
	return err
}
