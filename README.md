# object-N/A

**一个基于 Go 和 MyGo 构建的本地 Markdown 工作区。**

object-N/A 是一款面向本地文件的 Markdown 桌面应用。它将笔记管理、文档编辑、实时预览和知识图谱整合在一个简洁的工作区中，让你能够在熟悉的文件系统中自由组织和管理自己的文档。

打开一个 Vault，即可通过文件树浏览笔记、编辑 Markdown 文档、查看实时渲染结果，或通过知识图谱探索文档之间的关联。

所有笔记均以普通文件的形式保存在本地，无需迁移到专有格式，也不依赖特定的云端服务。

## 功能特性

- **文件管理**：通过文件树浏览和管理 Vault 中的文档。
- **Markdown 编辑**：在集成编辑器中编写和修改 Markdown 文档。
- **实时预览**：编辑文档的同时查看渲染后的内容。
- **知识图谱**：以可视化的方式浏览笔记之间的链接与关联。
- **多文档视图**：同时打开和查看多个文档，方便在不同内容之间切换。
- **Vault 管理**：打开已有的笔记目录，或直接创建新的 Vault。
- **PDF 阅读**：在工作区中预览 PDF 文档。
- **双语界面**：支持简体中文和 English，语言设置会在重启后保留。

## 界面与工作区

object-N/A 以 Vault 作为工作区单位。你可以选择已有的笔记目录，也可以创建一个新的 Vault。

应用将文件浏览、文档编辑和内容预览整合在一起，尽量减少在不同工具和窗口之间切换的需要。

## 快速开始

### 环境要求

| 项目 | 要求 |
| --- | --- |
| Go | 1.27.3 |
| MyGo | v0.8.3 |
| macOS | macOS 13 或更新版本 |
| Windows | Windows 10 或更新版本 |
| Linux | GTK 3、WebKitGTK 4.1 |

首次构建需要联网下载 Go 模块以及字体和渲染资源。

### 获取源码

克隆仓库：

```bash
git clone --branch v0.2.0 --depth 1 https://github.com/Ayntagn/object-N-A.git
cd object-N-A
```

也可以从 [GitHub Releases](https://github.com/Ayntagn/object-N-A/releases) 下载对应版本的源码。

### 准备资源

在项目根目录执行：

```bash
go version
go run ./cmd/assets
```

资源脚本会按照项目锁定的版本下载构建所需的字体和渲染资源，上游资源文件合计约 185 MB。

## 构建与安装

### macOS

要求：macOS 13 或更新版本。

构建：

```bash
mkdir -p dist
CGO_ENABLED=0 go build -trimpath -o dist/object-n-a ./cmd/app
```

检查构建结果：

```bash
./dist/object-n-a -version
```

安装应用：

```bash
sh packaging/install-macos.sh dist/object-n-a
```

安装脚本会在当前用户的 `~/Applications` 目录下创建 `object-N-A.app`。

启动：

```bash
open "$HOME/Applications/object-N-A.app"
```

也可以在启动时指定 Vault：

```bash
open "$HOME/Applications/object-N-A.app" --args -vault "$HOME/Notes"
```

### Windows

要求：Windows 10 或更新版本。

Markdown 预览和 PDF 阅读需要 Microsoft Edge WebView2 Evergreen Runtime。Windows 11 已包含该运行时，其他系统可以从 [Microsoft 官方网站](https://developer.microsoft.com/microsoft-edge/webview2/)下载安装。

在 PowerShell 中执行：

```powershell
New-Item -ItemType Directory -Force dist | Out-Null

$env:CGO_ENABLED = '0'

go build -trimpath -ldflags="-H=windowsgui" -o dist/object-n-a.exe ./cmd/app
```

运行：

```powershell
.\dist\object-n-a.exe
```

安装：

```powershell
powershell.exe -NoProfile -ExecutionPolicy RemoteSigned -File .\packaging\install-windows.ps1 -Binary .\dist\object-n-a.exe
```

安装脚本会将程序安装到：

```text
%LOCALAPPDATA%\Programs\object-N-A
```

同时创建开始菜单快捷方式。

指定 Vault 启动：

```powershell
.\dist\object-n-a.exe -vault "C:\path\to\Notes"
```

### Linux

要求：GTK 3 和 WebKitGTK 4.1。

Debian / Ubuntu：

```bash
sudo apt install libwebkit2gtk-4.1-0
```

Fedora：

```bash
sudo dnf install gtk3 webkit2gtk4.1
```

Arch Linux：

```bash
sudo pacman -S gtk3 webkit2gtk-4.1
```

构建：

```bash
mkdir -p dist
CGO_ENABLED=0 go build -trimpath -o dist/object-n-a ./cmd/app
```

运行：

```bash
./dist/object-n-a
```

安装：

```bash
sh packaging/install-linux.sh dist/object-n-a
```

安装脚本会将程序放置在 `~/.local/bin`，并注册对应的桌面入口。

指定 Vault 启动：

```bash
"$HOME/.local/bin/object-n-a" -vault "$HOME/Notes"
```

## Vault 管理

object-N/A 使用普通文件夹作为 Vault，不会要求用户使用特殊的文件格式。

启动应用后，你可以：

- **Open Vault**：打开已有的笔记目录。
- **Create Vault**：创建新的笔记工作区。

也可以通过 `-vault` 参数直接指定目录：

```bash
object-n-a -vault "$HOME/Notes"
```

将示例路径替换为实际的笔记目录即可。

## 界面语言

object-N/A 支持简体中文和 English 两种界面语言。

可以在设置中切换：

- English：Settings → General → Settings & Help Language
- 简体中文：设置 → 通用 → 设置与帮助语言

切换后，设置页面、帮助菜单、功能指南、键盘快捷键和欢迎页面都会同步更新。

语言选择会被保存，应用重启后仍然有效。

## 技术栈

object-N/A 主要使用以下技术构建：

| 技术 | 用途 |
| --- | --- |
| Go | 应用逻辑与后端实现 |
| MyGo | 桌面应用框架与原生窗口管理 |
| WebView | 应用界面渲染 |
| WKWebView | macOS 界面渲染 |
| WebView2 | Windows 界面渲染 |
| WebKitGTK | Linux 界面渲染 |

项目基于 [MyGo](https://github.com/egoist/mygo) 构建，利用 Go 实现应用逻辑，并通过各平台原生 WebView 承载用户界面。

## 相关文档

- [MyGo 官方文档](https://mygo.egoist.dev/)
- [Go 官方文档](https://go.dev/doc/)
- [第三方依赖说明](docs/THIRD_PARTY.md)
- [GitHub Releases](https://github.com/Ayntagn/object-N-A/releases)

## 致谢

感谢 Go、MyGo 以及项目所使用的开源依赖和工具的开发者。


