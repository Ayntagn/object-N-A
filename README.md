# object-N/A v0.2.0

object-N/A 是用 Go 和 MyGo 实现的本地 Markdown 工作区。打开一个 Vault 后，可以从文件树编辑笔记、并排查看 Markdown 预览、浏览链接图谱，或在多文件视图中同时查看几份稿件。

应用内显示 `object-N/A`。GitHub 仓库、macOS Bundle 和 Windows 快捷方式用 `object-N-A`，命令名为 `object-n-a`。本源码快照包含独立实现和自写的使用指南。构建用的字体和渲染资源由 `go run ./cmd/assets` 按锁定版本取回。 [第三方依赖](docs/THIRD_PARTY.md)

## 取得源码

仓库已公开，可直接克隆版本标签。也可以从 GitHub 发布页下载 `Source code (zip)`，解压后进入包含 `go.mod` 的目录。 [v0.2.0](https://github.com/Ayntagn/object-N-A/releases/tag/v0.2.0)

```sh
git clone --branch v0.2.0 --depth 1 https://github.com/Ayntagn/object-N-A.git
cd object-N-A
```

## 准备构建

安装 Go 1.27.3。在源码根目录运行资源命令，然后按下方对应平台的步骤构建。Go 模块固定 MyGo v0.8.3。首次准备资源及下载 Go 模块需要联网。字体和渲染资源的上游文件合计约 185 MB。 [Go 下载](https://go.dev/dl/) · [MyGo 平台要求](https://github.com/egoist/mygo/blob/v0.8.3/docs/README.md#requirements)

```sh
go version
go run ./cmd/assets
```

## macOS

使用 macOS 13 或更新版本，预览使用系统 WKWebView。以下命令构建当前 Mac 架构的可执行文件。 [Go 1.27 平台要求](https://go.dev/doc/go1.27)

```sh
mkdir -p dist
CGO_ENABLED=0 go build -trimpath -o dist/object-n-a ./cmd/app
./dist/object-n-a -version
```

安装脚本创建用户级 `~/Applications/object-N-A.app`，并使用 ad-hoc 签名。运行 `open` 后可从 Finder 的该目录或 Spotlight 启动应用。 [Apple 应用 Bundle 结构](https://developer.apple.com/library/archive/documentation/CoreFoundation/Conceptual/CFBundles/BundleTypes/BundleTypes.html)

```sh
sh packaging/install-macos.sh dist/object-n-a
open "$HOME/Applications/object-N-A.app"
```

首次从 Bundle 启动时，也可把现有 Vault 的目录传给应用。

```sh
open "$HOME/Applications/object-N-A.app" --args -vault "$HOME/Notes"
```

## Windows

使用 Windows 10 或 11。Markdown 预览和 PDF 阅读需要 Microsoft Edge WebView2 Evergreen Runtime，Windows 11 已包含。如果系统缺少该运行时，可从微软下载。 [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/)

在 PowerShell 中构建、运行。

```powershell
New-Item -ItemType Directory -Force dist | Out-Null
$env:CGO_ENABLED = '0'
go build -trimpath -ldflags="-H=windowsgui" -o dist/object-n-a.exe ./cmd/app
.\dist\object-n-a.exe
```

安装脚本把程序复制到当前用户的 `%LOCALAPPDATA%\Programs\object-N-A`，并创建开始菜单快捷方式 `object-N-A`。

```powershell
powershell.exe -NoProfile -ExecutionPolicy RemoteSigned -File .\packaging\install-windows.ps1 -Binary .\dist\object-n-a.exe
```

从命令行打开指定 Vault。

```powershell
.\dist\object-n-a.exe -vault "C:\path\to\Vault"
```

## Linux

在图形桌面安装 GTK 3 和 WebKitGTK 4.1。Debian 和 Ubuntu 可用以下命令。Fedora 包名为 `gtk3`、`webkit2gtk4.1`，Arch Linux 为 `gtk3`、`webkit2gtk-4.1`。 [MyGo 平台要求](https://github.com/egoist/mygo/blob/v0.8.3/docs/README.md#requirements)

```sh
sudo apt install libwebkit2gtk-4.1-0
```

```sh
mkdir -p dist
CGO_ENABLED=0 go build -trimpath -o dist/object-n-a ./cmd/app
./dist/object-n-a -version
```

安装脚本把程序放在 `~/.local/bin/object-n-a`，并在 `$XDG_DATA_HOME/applications` 注册 `object-n-a.desktop`。未设置该变量时使用 `~/.local/share/applications`。安装后可从应用菜单打开 `object-N/A`。 [Desktop Entry 规范](https://specifications.freedesktop.org/desktop-entry/latest/) · [XDG 目录规范](https://specifications.freedesktop.org/basedir-spec/latest/)

```sh
sh packaging/install-linux.sh dist/object-n-a
"$HOME/.local/bin/object-n-a" -vault "$HOME/Notes"
```

## 打开 Vault

把示例路径换成已有的笔记目录。启动时省略 `-vault`，可在工作区点击 `Open Vault` 或 `Create Vault`。从系统启动器进入应用后，也可从应用内选择文件。

## 切换设置与帮助语言

在 **Settings → General → Settings & Help Language** 点击 **简体中文**。设置页、帮助菜单、功能指南、键盘快捷键和欢迎页会即时切换。回到 **设置 → 通用 → 设置与帮助语言** 点击 **English** 可切回英文。语言选择会在重启后保留。
