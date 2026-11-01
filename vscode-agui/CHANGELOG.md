# Change Log

## 0.1.0

首个版本。

- 注册 `agui` 语言（`.ag`），提供 TextMate 语法高亮、中文关键字别名、代码片段、缩进与括号规则。
- `agui.exe check` 集成：保存 / 输入时自动检查，错误以诊断波浪线呈现，支持未保存内容。
- 命令：运行脚本、语法检查、打包 exe、选择解释器路径；编辑器标题栏提供 ▶ / ✓ / 📦 按钮。
- 运行与打包的输出通过伪终端实时流入 VS Code 内置终端面板（stdout 原样，stderr 红色）。
- 自动补全：控件、内置函数、GUI 函数、事件、属性、方法；悬停展示签名与说明。
- 配置项：`agui.path`、`agui.checkOnSave`、`agui.checkOnType`、`agui.checkDelay`、`agui.buildOutputDir`、`agui.buildOptions`。
