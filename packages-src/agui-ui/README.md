# agui-ui

Agui 的精品 UI 组件包，基于 0.9.5 原生自绘按钮（圆角、描边、悬停/按下色、阴影）。

## 安装

```powershell
agui install agui-ui
```

要求 Agui >= 0.9.5。

## 使用

```agui
import "agui-ui"

win = window("示例", 640, 420, bg="#0B1220")

card = ui_card("账户设置", "卡片容器")
save = ui_primary_button("保存更改", parent=card)
cancel = ui_secondary_button("取消", parent=card)

badge = ui_badge("NEW", "#7C3AED")
ghost = ui_ghost_button("了解更多")
icon = ui_icon_button("+", "新建项目")
```

## API

| 函数 | 说明 |
|---|---|
| `ui_primary_button(text, parent=null)` | 主操作按钮，蓝色圆角 |
| `ui_secondary_button(text, parent=null)` | 次要按钮，深灰描边 |
| `ui_ghost_button(text, parent=null)` | 幽灵按钮，透明底 |
| `ui_danger_button(text, parent=null)` | 危险操作按钮 |
| `ui_success_button(text, parent=null)` | 成功/发布按钮 |
| `ui_icon_button(icon, tip="", parent=null)` | 40px 图标按钮 |
| `ui_button(text, bg, fg, border, hover, pressed, radius, shadow, width, tip, parent)` | 通用按钮 |
| `ui_card(title, body="", parent=null)` | 340x190 卡片容器 |
| `ui_badge(text, color, parent=null)` | 彩色徽章标签 |
| `ui_heading(text, parent=null)` | 18px 粗体标题 |
| `ui_muted(text, parent=null)` | 灰色辅助文字 |
| `ui_divider(parent=null)` | 1px 分隔线 |

## 说明

- `parent` 必须是 `panel` / `scroll` / `grid` / `groupbox` 等容器。
- 按钮保留原生键盘焦点与 `click` 事件。
- 完整预览：主仓库 `examples/ui_kit_demo.ag` 与 `examples/ui_kit_package.ag`。
