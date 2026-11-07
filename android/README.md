# Agui Android 原生运行时

从 0.8 开始，Android 不再使用 WebView/WASM/HTML。`agui build` 把 `.ag` 单文件打包成 APK：Go 解释器经 gomobile 编译为 `libgojni.so`，Java Activity 通过 `Bridge` 接口创建原生 Android View。

用户目录不需要 `android/` 工程：脚本头部的 `#target android` 触发 APK 分支，其余模板、Manifest、图标、签名都在仓库的运行时目录里。

## 单文件示例

```agui
#target android
#name 星野养成
#package com.agui.star
#version 0.1.0
#icon assets/icon.png

win = window("星野养成")
title = label("星野养成", size=26, fg="#9A3412")
energy = progress(value=70, max=100)
rest = button("休息")

on rest.click:
    energy.value = 100
```

```powershell
.\agui.exe build examples\android_star.ag -o build\android\star.apk
```

产物约 5MB（含 arm64 Go 运行时），已用 debug keystore 签名，可直接：

```powershell
adb install -r build\android\star.apk
```

## 文件头

| 字段 | 说明 |
| --- | --- |
| `#target android` | 目标平台；缺省 `windows` |
| `#name 星野养成` | 应用名，写入 `strings.xml` |
| `#package com.agui.star` | Android 包名 |
| `#version 0.1.0` | versionName |
| `#icon assets/icon.png` | 应用图标，相对脚本目录 |

只允许入口文件开头一个 `#target`；缺少 target 时按 Windows 构建。

## 支持的 GUI 子集

- 控件：`window/panel/label/button/edit/checkbox/progress/image`
- 事件：`click/change/close`
- 平台函数：`toast/after/every/cancel_timer/quit/platform/dpi`
- 资源：`--asset <文件/目录>` 嵌入 APK；`image("oc.png")` 从 assets 读取

没有实现：menu/tray/table/tree/canvas/多窗口/Game Kit/SQLite/HTTP/WebSocket。运行到未注册函数会报“未定义函数”；构建期白名单校验尚未加入。

## 目录

| 路径 | 作用 |
| --- | --- |
| `mobile/` | Go 运行时（`Start`、`Bridge`、原生控件、定时器） |
| `cmd/agui/target.go` | `#target` 文件头解析 |
| `cmd/agui/android.go` | APK 构建流水线 |
| `android/runtime/MainActivity.java.tmpl` | 原生 View Activity 模板 |
| `android/runtime/agui.aar` | gomobile 绑定的 AAR（构建时产物，约 4.9MB） |
| `android/runtime/debug.keystore` | 本地调试签名 |
| `examples/android_star.ag` | 单文件 OC 养成示例 |

## 重新生成 AAR

修改 `mobile/` 后需要重建 AAR，再构建 APK：

```powershell
$env:JAVA_HOME = "C:\path\to\jdk-21"
$env:PATH = "$env:JAVA_HOME\bin;$env:PATH"
$env:ANDROID_HOME = "F:\aguicode\build\sdk"
$env:ANDROID_NDK_HOME = "F:\aguicode\build\sdk\ndk\26.3.11579264"
$env:GOTMPDIR = "F:\aguicode\build\tmp"
& "$(go env GOPATH)\bin\gomobile.exe" bind `
  -target=android/arm64 -androidapi 23 `
  -javapkg=com.agui.runtime `
  -o android/runtime/agui.aar ./mobile
```

`gomobile` 生成的 Java 包实际是 `com.agui.runtime.mobile`，模板已按此导入。

## APK 构建流水线

`cmd/agui/android.go` 自动完成：

1. 在临时目录写 `AndroidManifest.xml`、`assets/app.ag`、`res/` 和 Java 源码；
2. 从 AAR 提取 `classes.jar` 与 `libgojni.so`；
3. `aapt2 compile/link` 生成资源 APK；
4. `javac` 编译 Activity，`jar` 打包 classes，`d8` 生成 dex；
5. Go 重打包：资源 + `classes.dex` + `lib/arm64-v8a/libgojni.so`；
6. `zipalign`、`apksigner` 签名。

需要 SDK：platform 35、build-tools 35.0.0；JDK 17/21。

## 验证

```powershell
$BT = "build\sdk\build-tools\35.0.0"
& "$BT\aapt2.exe" dump badging build\android\star.apk
& "$BT\apksigner.bat" verify --verbose build\android\star.apk
```

已确认：`com.agui.star`、versionName 0.1.0、minSdk 23、targetSdk 34、图标/标签、`assets/app.ag`、`classes.dex`、`lib/arm64-v8a/libgojni.so` 均存在，v1/v2/v3 签名通过。

**未验证**：本环境没有 Android 真机或模拟器，APK 安装后的 Activity 启动、View 渲染和触摸回调仍需设备验证。

## 旧 WebView 实验

`cmd/aguiweb/`、`internal/webui/`、`web/`、`android/app/` 是 0.8 之前的 WebView 路线，保留作历史/浏览器调试，不再是 `agui build` 的 Android 默认实现。


## Release 构建

`--release` 必须显式提供自己的 keystore，工具链不会生成或内置发布密钥：

```powershell
.\agui.exe build examples\android_star.ag -o build\android\star-release.apk `
  --release --ks C:\keys\my-release.keystore --ks-pass 你的密码 `
  --key-alias mykey --key-pass 你的密码 --version-code 2
```

默认 debug 构建仍使用本地 `android/runtime/debug.keystore`（不存在时自动生成）。文件选择用 `pick_file`（SAF，结果触发窗口 `file` 事件）；运行时权限用 `#permission` 声明 + `request_permission`（结果触发窗口 `permission` 事件）。当前版本仍不生成 AAB。
