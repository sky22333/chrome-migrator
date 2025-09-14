package utils

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32DLL                = windows.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpaceExW    = kernel32DLL.NewProc("GetDiskFreeSpaceExW")
)

func CheckDiskSpace(path string, requiredSize int64) error {
	available, err := GetAvailableDiskSpace(path)
	if err != nil {
		return err
	}
	if available < requiredSize {
		return fmt.Errorf("磁盘空间不足，需要 %d MB，可用 %d MB", 
			requiredSize/1024/1024, available/1024/1024)
	}
	return nil
}

func GetAvailableDiskSpace(path string) (int64, error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	var freeBytesAvailable uint64
	var totalNumberOfBytes uint64
	var totalNumberOfFreeBytes uint64

	ret, _, _ := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)

	if ret == 0 {
		return 0, fmt.Errorf("无法获取磁盘空间信息")
	}

	return int64(freeBytesAvailable), nil
}

func FormatBytes(bytes int64) string {
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