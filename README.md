## Chrome/Edge 浏览器数据迁移工具

一个用于备份和迁移 Chrome/Edge 浏览器数据的 Windows 工具。

## 功能特性

- 支持 Chrome 和 Microsoft Edge 浏览器
- 自动检测浏览器安装路径和用户数据
- 备份书签、历史记录、密码、Cookie 等数据
- 压缩备份文件，节省存储空间
- 实时进度显示
- 自动关闭浏览器进程

## 使用方法

1. 下载并运行 `chrome-migrator.exe`
2. 选择要备份的浏览器（Chrome/Edge/Both）
3. 确认关闭浏览器进程
4. 等待备份完成

## 输出文件

备份文件保存在 `C:\chrome-backup\` 目录：
- `chrome_backup_YYYYMMDD_HHMMSS.zip` - Chrome 备份
- `edge_backup_YYYYMMDD_HHMMSS.zip` - Edge 备份

## 编译构建
```
go mod tidy
```
```
go build -ldflags="-s -w" -o chrome-migrator.exe
```


## 系统要求

- Windows 10/11
- 足够的磁盘空间（约为浏览器数据大小的2倍）

## 注意事项

- 备份前请关闭目标浏览器
- 确保有足够的磁盘空间
- 备份文件包含敏感信息，请妥善保管
