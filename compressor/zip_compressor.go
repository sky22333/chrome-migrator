package compressor

import (
	"archive/zip"
	"chrome-migrator/config"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ZipCompressor struct {
	OutputPath   string
	TempDir      string
	BrowserName  string
	ProgressCallback func(current, total int64, message string)
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
	}
}

// simplifyBrowserName
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

func (c *ZipCompressor) CompressData() error {
	if err := c.ensureOutputDir(); err != nil {
		return fmt.Errorf("创建输出目录失败: %v", err)
	}

	// 计算总文件数
	var totalFiles int64
	filepath.Walk(c.TempDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalFiles++
		}
		return nil
	})

	zipFile, err := os.Create(c.OutputPath)
	if err != nil {
		return fmt.Errorf("创建zip文件失败: %v", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	var processedFiles int64
	err = filepath.Walk(c.TempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		processedFiles++
		if c.ProgressCallback != nil {
			message := fmt.Sprintf("正在压缩 %s 数据...", c.BrowserName)
			c.ProgressCallback(processedFiles, totalFiles, message)
		}

		relPath, err := filepath.Rel(c.TempDir, path)
		if err != nil {
			return nil
		}

		relPath = strings.ReplaceAll(relPath, "\\", "/")

		if err := c.addFileToZip(zipWriter, path, relPath); err != nil {
			return nil
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("压缩数据失败: %v", err)
	}

	return nil
}

func (c *ZipCompressor) ensureOutputDir() error {
	outputDir := filepath.Dir(c.OutputPath)
	return os.MkdirAll(outputDir, 0755)
}

func (c *ZipCompressor) addFileToZip(zipWriter *zip.Writer, filePath, zipPath string) error {
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

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
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
