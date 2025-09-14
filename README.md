## Chrome/Edge 浏览器数据迁移工具

一个用于备份和迁移 Chrome/Edge 浏览器数据的 Windows 轻量级工具。

## 功能特性

- 支持 Chrome 和 Microsoft Edge 浏览器
- 自动检测浏览器安装路径和用户数据
- 备份书签、历史记录、密码、Cookie 等数据
- 还原登录状态
- 压缩备份文件，节省存储空间
- 实时进度显示

## 使用方法

1. 下载并运行 `chrome-migrator.exe`
2. 选择要备份的浏览器（Chrome/Edge）
3. 确认关闭浏览器进程
4. 等待备份完成
5. 将备份的文件发送到新设备
6. 按照提示手动解压到指定目录即可

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
- 足够的磁盘空间

## 注意事项

- 备份前请关闭目标浏览器
- 确保有足够的磁盘空间
- 备份文件包含敏感信息，请妥善保管

## 免责声明

- 本项目及相关工具仅用于个人数据备份、迁移和管理目的，不得用于任何非法用途。使用本项目可能涉及个人隐私或敏感数据，请确保遵守当地法律法规。
- 作者对因使用本项目造成的任何直接或间接损失概不负责。使用者应自行承担使用风险，并确保数据安全。亦不承担因操作不当引起的任何法律责任。
- This project and its related tools are intended solely for personal data backup, migration, and management purposes and must not be used for any illegal activities. Using this project may involve personal or sensitive data; please ensure compliance with applicable laws and regulations.
- The author is not responsible for any direct or indirect losses resulting from the use of this project. Users assume all risks and are responsible for ensuring data security. The author also disclaims any legal liability arising from improper use.
