# Agui 语言与 GUI 参考

本文档对应 Agui 0.4。示例默认使用英文关键字；中文关键字别名在 [2.3](#23-关键字与中文别名) 中列出。

---

## 1. 运行模型

- 一个 `.ag` 文件就是一个程序，可以用 `agui run app.ag` 运行，也可以打包成独立 exe。
- 解释器在**主线程**上执行脚本，并把该线程锁定为 GUI 线程；所有控件和事件都在同一线程上处理。
- `window()` 和控件函数在脚本执行期间创建窗口。窗口在 `run()`（或脚本结束后的自动运行）时才显示。
- 如果脚本结束时已经创建了窗口、且没有调用过 `run()`，命令行入口会自动启动消息循环。因此 `run()` 通常可以省略。
- 只支持一个主窗口。
- GUI 版解释器没有控制台，`print` 的输出不可见；需要看输出请用 `agui.exe` 运行。

一个最小的完整程序：

```agui
win = window("Hello", w=480, h=300)
label("你好，世界")
run()
```

---

## 2. 词法

### 2.1 编码、注释与换行

- 源文件使用 UTF-8。
- `#` 到行尾是注释；`//` 到行尾也是注释。
- 字符串可以跨行吗？不可以。字符串中的换行请写 `\n`。
- 语句以换行结束；也可以用分号 `;` 分隔同一行的多条语句：

```agui
a = 1; b = 2
print(a + b)
```

- 在 `(`、`[`、`{` 内部，换行会被忽略，允许把长表达式拆成多行：

```agui
items = [
    "苹果",
    "香蕉",
]
```

### 2.2 缩进

Agui 使用 Python 风格的缩进表示代码块。`if`、`while`、`for`、`func`、`on` 后面必须跟 `:`，下一条非空行必须增加缩进：

```agui
if score >= 60:
    print("及格")
else:
    print("不及格")
```

- 空格和 Tab 都可以，Tab 按 4 个空格计算。
- 空行和注释行不影响缩进。
- 缩进必须回到某个已存在的层级，不能出现“半格”缩进。

### 2.3 关键字与中文别名

| 英文 | 中文 | 说明 |
| --- | --- | --- |
| `if` / `elif` / `else` | `如果` / `否则如果` / `否则` | 条件分支 |
| `while` | `当` | 循环 |
| `for ... in ...` | `对于 ... 在 ...` | 遍历 |
| `func` | `函数` | 定义函数 |
| `return` | `返回` | 返回 |
| `break` / `continue` | `中断` / `继续` | 跳出 / 下一轮 |
| `on` | `监听` | 注册事件 |
| `try` / `catch` / `finally` | `尝试` / `捕获` / `最终` | 异常捕获与收尾 |
| `raise` | `抛出` | 主动抛出错误 |
| `class` | `类` | 轻量类 |
| `import` | `导入` | 导入其他脚本 |
| `lambda` | `匿名` | 匿名函数 |
| `pass` | `略过` | 空语句 |
| `and` / `or` / `not` | `并且`（或 `且`）/ `或者`（或 `或`）/ `非` | 逻辑运算 |
| `true` / `false` / `null` | `真` / `假` / `空` | 字面量 |

中文示例：

```agui
如果 真:
    略过
```

### 2.4 标识符

标识符由 Unicode 字母、数字和下划线组成，不能以数字开头。中文标识符合法：

```agui
用户名 = "小明"
print(用户名)
```

关键字不能作为变量名。

### 2.5 字面量

```agui
n1 = 42
n2 = 3.14
n3 = 0xFF      # 十六进制
s1 = "文本"
s2 = '单引号也可以'
b1 = true
b2 = false
nothing = null
list1 = [1, 2, 3]
map1 = {"name": "Agui", "year": 2026}
```

字符串转义支持：`\n`、`\t`、`\r`、`\b`、`\f`、`\v`、`\0`、`\\`、`\"`、`\'`、`\xNN`、`\uNNNN`、`\u{1F600}`。

数字在运行时统一为浮点数。打印整数值时不显示 `.0`。没有专门的整数除法：`7 / 2` 等于 `3.5`；需要整数商可以用 `int(7 / 2)`（向零截断）或 `floor(7 / 2)`。

---

## 3. 类型

| 类型 | `type()` 返回值 | 说明 |
| --- | --- | --- |
| `null` | `"null"` | 空值 |
| 布尔 | `"bool"` | `true` / `false` |
| 数字 | `"number"` | 统一为 64 位浮点 |
| 字符串 | `"string"` | 不可变，按 Unicode 字符（rune）索引 |
| 列表 | `"list"` | 可变、有序 |
| 字典 | `"map"` | 插入有序、键为字符串 |
| 函数 | `"function"` | 用户函数或内置函数 |
| 控件 | `"widget"` | GUI 控件 |
| 字体 | `"font"` | `font()` 返回的字体 |

### 3.1 真值

以下值为假，其余为真：

- `null`
- `false`
- 数字 `0`
- 空字符串 `""`
- 空列表 `[]`
- 空字典 `{}`

### 3.2 相等

`==` 和 `!=` 进行深比较：列表逐项比较，字典逐键比较。不同类型通常不相等（数字和字符串不会自动转换）。

### 3.3 索引

```agui
items = [10, 20, 30]
print(items[0])     # 10
print(items[-1])    # 30（负数从末尾数）
print("中文"[1])     # 文

person = {"name": "小明"}
print(person["name"])
```

列表下标越界、字典键不存在、字符串下标越界都会报运行时错误。字典的 `get(key, 默认值)` 可以在键不存在时返回默认值。

---

## 4. 表达式与运算符

优先级从低到高：

| 优先级 | 运算符 | 说明 |
| --- | --- | --- |
| 1 | `or` | 逻辑或，短路，返回布尔值 |
| 2 | `and` | 逻辑与，短路，返回布尔值 |
| 3 | `not` | 逻辑非 |
| 4 | `==` `!=` `<` `<=` `>` `>=` `in` | 比较与成员判断 |
| 5 | `+` `-` | 加减、字符串/列表/字典拼接 |
| 6 | `*` `/` `%` | 乘除、取余、字符串/列表重复 |
| 7 | `-` `+`（一元） | 取负、取正 |
| 8 | `()` `[]` `.` | 调用、下标、属性 |

### 4.1 算术

```agui
print(1 + 2 * 3)     # 7
print(10 / 4)        # 2.5
print(10 % 3)        # 1
print("ab" * 3)      # ababab
print([1, 2] * 2)    # [1, 2, 1, 2]
```

- `+`：数字相加；任一边是字符串时拼接（自动把另一边转成字符串）；列表与列表连接；字典与字典合并（右边覆盖左边）。
- `*`：数字相乘；字符串或列表乘以整数表示重复。
- `/`：浮点除法，除数为 0 报错。
- `%`：浮点取余，除数为 0 报错。

### 4.2 比较与成员

```agui
print(3 < 4)                 # true
print("abc" < "abd")         # true
print(2 in [1, 2, 3])        # true
print("name" in {"name": 1}) # true
print("gu" in "Agui")        # true
```

- 大小比较只支持数字与数字、字符串与字符串。
- `in` 对列表做相等判断，对字典判断键，对字符串判断子串。
- 没有 `not in` 语法，可以写 `not (x in items)`。

### 4.3 赋值运算

```agui
n = 1
n += 2      # 3
n *= 4      # 12
```

左值可以是变量、控件属性或下标：

```agui
label1.text = "新文字"
items[0] = 99
person["age"] = 13
```

对未定义变量使用 `+=` 等复合赋值会报错。

---

## 5. 变量与作用域

- 第一次赋值在当前作用域创建变量。
- 之后的赋值会优先更新**已存在的**外层变量；如果都没有，则在当前作用域创建。
- 这让事件处理器可以直接修改脚本顶层的计数器，不需要 `global` 关键字。

```agui
count = 0

func inc():
    count += 1   # 更新顶层 count

on btn.click:
    inc()
    display.text = str(count)
```

函数参数和局部变量在函数调用结束后消失；闭包会保留它们：

```agui
func make_counter():
    n = 0
    func next():
        n += 1
        return n
    return next

c = make_counter()
print(c(), c())   # 1 2
```

---

## 6. 语句

### 6.1 if / elif / else

```agui
if score >= 90:
    print("A")
elif score >= 60:
    print("B")
else:
    print("C")
```

### 6.2 while

```agui
i = 0
while i < 3:
    print(i)
    i += 1
```

### 6.3 for

```agui
for item in ["a", "b"]:
    print(item)

for ch in "中文":
    print(ch)

for key in {"a": 1, "b": 2}:
    print(key)

for i in 3:          # 0, 1, 2
    print(i)

for i in range(1, 10, 2):
    print(i)
```

`for` 可以遍历列表、字符串、字典（键）和数字；数字 `n` 表示 `0..n-1`。

`break` 跳出当前循环，`continue` 进入下一轮。循环变量在循环结束后仍然存在。

### 6.4 pass

```agui
func todo():
    pass
```

### 6.5 return

```agui
func max2(a, b):
    if a > b:
        return a
    return b
```

没有显式返回值时函数返回 `null`。在函数外使用 `return` 会报错。

### 6.6 on

见 [第 8 节](#8-事件)。

### 6.7 语句分隔

换行分隔语句，分号也可以。多条语句不能写在同一个缩进块的第一行（`if x: y = 1` 不支持），必须在冒号后换行并缩进。

---

## 7. 函数

```agui
func name(a, b = 2):
    return a + b
```

- 支持默认参数；默认值在调用时求值。
- 调用时可以使用关键字参数：

```agui
print(name(1))
print(name(1, b=10))
print(name(a=1, b=10))
```

- 位置参数必须写在关键字参数前面。
- 不接受可变参数（`*args`）和解包（`**kwargs`）。
- 支持匿名函数 `lambda`（单表达式或多行块）；详见 16.2。

```agui
func tick():
    print(now())

every(1000, tick)
```

函数是值，可以存入变量、列表或字典，也可以作为参数传递。递归最大深度约 512 层。

重复定义同名函数会报错（同一作用域）。

---

## 8. 事件

语法：

```agui
on 控件.事件:
    ...
```

例如：

```agui
btn = button("点我")
on btn.click:
    msgbox("被点击了")
```

- `on` 后面必须是 `控件.事件` 形式。
- 事件处理在 GUI 线程上同步执行。
- 一个事件可以注册多个处理器，按注册顺序执行。
- 处理器里可以访问定义处的作用域。
- 处理器抛出的运行时错误会显示错误对话框（包含行号和调用栈），程序继续运行。

### 8.1 支持的事件

| 控件 | 事件 | 触发时机 |
| --- | --- | --- |
| `window` | `close` | 用户关闭窗口或调用 `win.close()` 后、窗口销毁前 |
| `window` | `resize` | 窗口尺寸变化后 |
| `label` | `click` / `dblclick` | 点击 / 双击标签 |
| `button` | `click` | 按钮被点击 |
| `checkbox` | `click` / `change` | 勾选状态变化 |
| `edit` | `change` | 文本发生变化 |
| `listbox` | `select` / `dblclick` | 选中项变化 / 双击条目 |
| `combo` | `select` / `change` / `dblclick` | 选择变化 / 编辑内容变化 / 双击 |
| `slider` | `change` | 滑块位置变化 |
| `menuitem` | `click` | 菜单项被点击 |
| `progress` | 无 | — |

### 8.2 程序化触发

```agui
btn.click()   # 发送 BM_CLICK，同步执行 click 处理器
```

这在自动化测试里很有用：脚本可以在 `run()` 之前连续点击按钮，验证界面状态。

### 8.3 关闭行为

- 点击窗口右上角关闭按钮：先执行 `on win.close`，再销毁窗口并结束消息循环。
- `win.close()`：同上。
- `quit()`：直接结束消息循环，不触发 `close` 事件。

关闭事件目前**不能取消**。

---

## 9. GUI 模型与布局

### 9.1 创建窗口

```agui
win = window("标题", w=900, h=640, bg="#F7F8FA", resizable=true)
```

- 不调用 `window()` 而直接创建控件时，会自动创建一个 900×640、标题为“Agui 应用”的窗口。
- 每次调用 `window()` 都会创建一个新的顶层窗口；用 `win.use()` / `win.activate()` 切换后续控件默认放入哪个窗口。
- `w` / `h` 是**客户区**尺寸（不含标题栏和边框）。
- 坐标和尺寸使用逻辑像素（96 DPI 基准）；在高 DPI 屏幕上 Agui 会按系统 DPI 缩放。
- Windows 10 1703+ 默认启用 Per-Monitor V2 DPI 感知；窗口拖到不同缩放的显示器时会收到 `WM_DPICHANGED`，重新创建默认字体并重排子控件。自定义字体不会自动重建。
- 窗口默认使用系统默认位置，可以用 `win.center()` 居中。

### 9.2 自动布局

不指定坐标时：

- 左边距 20，控件间距 10。
- 控件按创建顺序从上到下排列。
- 标签、输入框、复选框、列表、进度条、滑块默认占满可用宽度。
- 按钮默认宽 150，下拉框默认宽 260。
- 默认高度：标签 24、按钮 34、输入框 30（多行 120）、复选框 28、列表 140、下拉框 30（下拉部分 220）、进度条 18、滑块 36。

指定 `x` / `y` 即可精确放置；指定 `w` / `h` 即可精确控制尺寸。窗口缩放时，自动布局的控件会重新计算，显式坐标的控件保持原样。

```agui
window("混合布局", w=640, h=480)
label("标题")                                  # 自动
edit("自动占满宽度")                            # 自动
button("固定", x=20, y=200, w=150, h=36)       # 精确
```

#### 容器、锚点与网格

```agui
card = panel(x=20, y=20, w=360, h=220)
label("账户", parent=card)
edit("用户名", parent=card)
checkbox("记住我", parent=card)

side = scroll(x=400, y=20, w=300, h=220)
for i in range(20):
    label("列表项 " + str(i), parent=side)

keys = grid(cols=3, x=20, y=260, w=420, h=160)
for k in ["7", "8", "9", "4", "5", "6"]:
    button(k, parent=keys)

ok = button("确定", anchor="right bottom", x=20, y=20, w=120, h=36)
```

- `panel()` 是无滚动容器；`scroll()` 带垂直滚动条，支持滚轮；`grid(cols=2)` 按列数排列子控件。
- 子控件传 `parent=容器对象`；容器可以嵌套，groupbox 内也可以再放 panel/grid。
- 不写 `x/y` 时，子控件从容器左上角按 `margin`、`gap` 纵向排列；容器自身的 `x/y/w/h` 仍按父级规则。
- `anchor` 可为 `left`、`right`、`top`、`bottom` 的组合。`right` 贴右、`bottom` 贴底；同时 `left right` 横向拉伸、`top bottom` 纵向拉伸。
- `visible=false` 的控件不显示，也不占用布局位置；重新设为 `true` 会触发父容器重排。
- `scroll_y` / `scroll_height` 可读写滚动位置和内容总高度。

### 9.3 字体和颜色

```agui
big = font("Microsoft YaHei UI", 24, bold=true)
title = label("大标题", font=big, fg="#1D4ED8")
win.bg = "#F0F4FF"
entry.bg = "#FFFFFF"
```

- `font(family, size, bold=false, italic=false)`，`size` 单位是磅。
- 颜色支持 `#RRGGBB`、`#RGB` 和 `white`、`black`、`red`、`green`、`blue`、`gray`、`orange`、`yellow`。
- `fg` 是文字颜色，`bg` 是背景颜色。
- 给按钮设置 `fg` / `bg` 会自动切换为自绘按钮，颜色生效；其他原生控件（复选框、列表等）仍以系统主题为主。

### 9.4 窗口生命周期

```agui
win = window("生命周期", w=480, h=300)
label("内容")

on win.close:
    print("关闭前保存数据")

run()        # 显示窗口并进入消息循环
print("窗口已关闭")
```

- `run()` 会显示窗口并阻塞，直到窗口关闭或调用 `quit()`。
- 脚本需要先注册事件，再进入消息循环。
- 窗口关闭后，`run()` 返回，脚本继续执行。
- 不能重复调用 `run()`。

### 9.5 截图

```agui
after(800, capture_and_quit)

func capture_and_quit():
    screenshot("shots/app.png")
    quit()
```

`screenshot()` 捕获主窗口（含边框）并保存为 PNG。窗口需要已经显示并完成绘制。

---

## 10. 控件参考

所有控件都支持：

- 创建参数：`x`, `y`, `w`, `h`, `fg`, `bg`, `font`, `enabled`, `visible`。
- 属性：`kind`, `x`, `y`, `w`, `h`（`width` / `height` 是别名）, `enabled`, `visible`, `font`。
- 方法：`focus()`, `show()`, `hide()`。

### 10.1 window

```agui
win = window("标题", w=900, h=640, bg="#F7F8FA", resizable=true)
```

| 属性 | 说明 |
| --- | --- |
| `title` / `text` | 标题 |
| `bg` | 背景色 |
| `w` / `h` | 客户区尺寸 |
| `x` / `y` | 窗口在屏幕上的位置 |

方法：`close()`, `center()`, `minimize()`, `maximize()`, `restore()`, `show()`, `hide()`。

属性：`dropped` / `dropped_files`（最近拖入的文件路径列表）。

事件：`close`、`resize`、`drop`。

### 10.2 label

```agui
label("文字", fg="#333333", font=big)
```

属性：`text`、`fg`、`bg`。
事件：`click`、`dblclick`。

### 10.3 button

```agui
btn = button("确定", w=120)
on btn.click:
    ...
```

属性：`text`、`enabled`。
方法：`click()`、`focus()`。
事件：`click`。

### 10.4 edit

```agui
name = edit("", w=300)
notes = edit("", multiline=true, h=120)
secret = edit("", password=true)
```

| 属性 | 说明 |
| --- | --- |
| `text` | 内容 |
| `readonly` | 只读 |
| `password` | 密码模式 |
| `multiline` | 只能在创建时指定 |

方法：`clear()`、`append(text)`、`focus()`。
事件：`change`。

### 10.5 checkbox

```agui
agree = checkbox("我同意", checked=true)
print(agree.checked)
```

属性：`text`、`checked`。
事件：`click`、`change`。

### 10.6 listbox

```agui
list = listbox(["第一项", "第二项"], h=180)
list.add_item("第三项")
print(list.count, list.selected)
```

属性：`items`（列表，可赋值整体替换）、`selected`（下标，-1 表示无）、`count`（只读）、`text`（当前选中文本，只读）。
方法：`add_item(text)`, `remove_item(index)`, `select(index)`, `clear()`。
事件：`select`、`dblclick`。

### 10.7 combo

```agui
plan = combo(["免费版", "专业版", "企业版"])
plan.selected = 1
print(plan.text)
```

属性、方法、事件与 `listbox` 基本相同。下拉框高度在创建时固定为 30（下拉列表另算）。

### 10.8 progress

```agui
bar = progress(30, max=100)
bar.value = 75
```

属性：`value`（0..max）、`max`。
没有事件。

### 10.9 slider

```agui
s = slider(30, min=0, max=100)
on s.change:
    bar.value = s.value
```

属性：`value`、`min`、`max`。
方法：`set_range(min, max)`。
事件：`change`。

### 10.10 menu / menuitem

```agui
win = window("菜单示例", w=600, h=360)

file = menu("文件(&F)")
open = file.item("打开(&O)...")
file.separator()
file.item("退出(&X)", quit)

help = menu("帮助(&H)")
about = help.item("关于 Agui")

on open.click:
    msgbox("打开文件")

on about.click:
    msgbox("关于这个程序")
```

| 调用 / 属性 | 说明 |
| --- | --- |
| `menu(title)` | 在窗口菜单栏创建一级菜单 |
| `file.item(text, callback?, accel=, checkable=, checked=, enabled=)` | 添加菜单项，返回菜单项对象 |
| `file.submenu(text)` | 添加子菜单，返回 `Menu` |
| `file.separator()` | 添加分隔线 |
| `menuitem.click()` | 以程序方式触发点击处理器 |
| `menuitem.text` / `menuitem.enabled` | 菜单项文字 / 是否可用 |
| `menuitem.accel` | 真实快捷键，例如 `"Ctrl+O"` / `"F5"` / `"Ctrl+Shift+S"` |
| `menuitem.icon` / `menuitem.has_icon` | 菜单项图标路径 / 图标是否加载 |
| `menuitem.checkable` / `menuitem.checked` | 是否可勾选 / 当前勾选状态 |
| `menuitem` 的 `click` 事件 | 菜单项被点击；可勾选项会自动翻转 |

- 菜单栏会自动占用客户区高度，自动布局的控件会相应下移。
- 标题中的 `&` 表示 Alt 访问键，例如 `"文件(&F)"`。
- `accel` 会注册真正的键盘快捷键；菜单项 `icon` 用 png/jpg 图片。快捷键表在窗口消息循环里通过 `TranslateAcceleratorW` 分发。
- 子菜单通过 `sub.item(...)` 继续添加条目，目前支持两级。

---

## 11. 内置函数库

### 11.1 输出与转换

| 函数 | 说明 |
| --- | --- |
| `print(...)` | 用空格连接参数并输出到标准输出（GUI 版不可见） |
| `warn(...)` | 输出到标准错误 |
| `str(v)` | 转成字符串 |
| `num(v)` | 转成数字，失败报错 |
| `int(v)` | 向零截断 |
| `bool(v)` | 真值转换 |
| `type(v)` | 类型名 |
| `len(v)` | 字符串字符数、列表长度、字典键数 |
| `format(tpl, ...)` | `{}` 顺序占位、`{name}` 关键字占位 |

```agui
print(format("你好，{name}！", name="小明"))
```

### 11.2 字符串

`upper`, `lower`, `trim`, `split(s, sep)`（不传 sep 按空白切分）, `join(list, sep)`, `replace(s, old, new)`, `contains(s, sub)`, `startswith`, `endswith`, `substr(s, start, len)`, `find(s, sub)`

同样可以作为字符串方法调用：

```agui
"  Agui  ".trim().lower()
"a,b,c".split(",")
```

### 11.3 列表

`list(...)`, `append(list, ...)`, `pop(list, index=-1)`, `insert(list, index, value)`, `remove(list, value)`, `sort(list)`, `reverse(list)`, `join(list, sep)`

列表方法：`push` / `append`, `pop`, `insert`, `remove`, `clear`, `sort`, `reverse`, `index`, `join`, `len`, `slice`, `map(fn)`, `filter(fn)`, `each(fn)`。

```agui
nums = [3, 1, 2]
nums.sort()
nums.push(4)
print(nums.slice(1, 3))
```

### 11.4 字典

`dict(name=..., ...)`, `keys(map)`, `values(map)`, `items(map)`, `has(container, value)`, `get(map, key, default)`, `set(map, key, value)`

字典方法：`keys`, `values`, `items`, `has`, `get`, `set`, `remove`, `clear`, `len`。

```agui
person = dict(name="小明", age=12)
for k in person.keys():
    print(k, person[k])
```

### 11.5 数学与时间

`range(start, stop, step)`, `abs`, `round`（四舍五入）, `floor`, `ceil`, `sqrt`, `pow(a, b)`, `min`, `max`, `sum`, `random()`, `randint(a, b)`, `now()`, `timestamp()`, `sleep(ms)`

- `range` 返回数字列表。
- `random()` 返回 `[0, 1)`。
- `randint(a, b)` 返回 `[a, b]` 内的整数。
- `now()` 返回 `"2026-10-31 16:00:21"`。
- `sleep(ms)` 最多 60 秒，会阻塞 UI。

### 11.6 文件与目录

| 函数 | 说明 |
| --- | --- |
| `read_file(path)` | 读取文本（UTF-8） |
| `write_file(path, text)` | 覆盖写入 |
| `append_file(path, text)` | 追加 |
| `file_exists(path)` | 是否存在 |
| `delete_file(path)` | 删除 |
| `list_dir(path)` | 目录项名称列表 |
| `mkdir(path)` | 递归创建目录 |
| `basename(path)` / `dirname(path)` | 路径拆分 |
| `join_path(...)` / `abs_path(path)` | 路径处理 |
| `cwd()` | 当前工作目录 |

路径可以是相对路径（相对于启动程序的工作目录）或绝对路径。

### 11.7 对话框

```agui
msgbox("内容", title="提示", kind="info")   # info / warn / error / question
ok = confirm("确定退出？", title="确认")
name = input("请输入姓名：", default="")
path = open_file("文本文件|*.txt|所有文件|*.*")
path = save_file("文本文件|*.txt", default="新文件.txt")
```

- `input` 返回字符串；用户取消时返回 `null`。
- `open_file` / `save_file` 返回路径；取消时返回 `null`。
- 文件过滤器格式：`描述|模式|描述|模式|...`。

### 11.8 定时器

```agui
func tick():
    print("一秒过去了")

id = every(1000, tick)
after(500, tick)
cancel_timer(id)
```

- 回调必须是函数值（具名函数）。
- 回调在 GUI 线程执行。
- `after` 只执行一次；`every` 重复执行。
- 返回的 id 可以传给 `cancel_timer`。

### 11.9 其他 GUI 函数

| 函数 | 说明 |
| --- | --- |
| `run()` | 显示窗口并进入消息循环 |
| `quit()` | 结束消息循环 |
| `screenshot(path="screenshot.png")` | 截取主窗口 |
| `set_clipboard(text)` / `get_clipboard()` | 剪贴板 |
| `screen_width()` / `screen_height()` | 屏幕逻辑尺寸 |
| `dpi()` | 当前 DPI（96 为 100%） |
| `args` | 运行参数列表（全局变量） |
| `script_path` | 脚本路径（全局变量） |

---

## 12. 错误处理

- 语法错误会指出文件名、行号和列号。
- 运行时错误包含出错行、错误信息、附近的源代码以及调用栈。
- 顶层错误（启动阶段）会打印到 stderr；打包后的 GUI 程序会用消息框显示。
- 事件处理器中的错误会用消息框显示，程序继续运行。
- 目前没有 `try/catch`。

示例错误输出：

```text
test.ag 第 3 行

错误: 不能除以零

    2 | on w.click:
 >  3 |     x = 1 / 0
     4 |
```

调试内部错误时可以设置环境变量 `AGUI_DEBUG=1`，解释器会把 Go 层的 panic 堆栈打印到 stderr。

---

## 13. 命令行与打包

```text
agui run  <脚本.ag> [--debug] [--shot[=文件.png]] [参数...]
agui check <脚本.ag>
agui init [目录] [--name 名称]
agui build <脚本.ag> [-o 输出.exe] [-console] [--icon 图标] [版本参数...]
agui version
```

### 13.1 运行

```powershell
.\agui.exe run examples\hello.ag
.\agui.exe run examples\console.ag
```

`--shot[=文件.png]` 会在窗口显示 1 秒后截图并退出，适合无人值守的界面检查：

```powershell
.\agui.exe run examples\hello.ag --shot=shots\hello.png
```

### 13.2 语法检查

```powershell
.\agui.exe check examples\hello.ag
```

只做词法和语法分析，不执行脚本，也不会创建窗口。

### 13.3 打包

```powershell
.\agui.exe build examples\hello.ag -o hello.exe
```

- 打包会把脚本追加到运行器 exe 末尾，并在启动时读取。生成的 exe 不依赖外部脚本文件。
- 默认优先使用同目录下的 `aguiw.exe`（无控制台）。如果没有，则使用当前 `agui.exe` 并提示仍会带控制台。
- `-console` 强制使用当前控制台版。
- 输出目录不存在时会自动创建。
- `--icon` 嵌入 exe 图标；`--product`、`--company`、`--description`、`--copyright`、`--file-version` 写入版本资源。
- 例如：`.\agui.exe build examples\hello.ag -o hello.exe --icon assets\app.ico --product "我的工具" --company "Agui" --file-version 1.2.3.4`
- 运行打包后的 exe 时，`args` 是命令行参数列表，`script_path` 为 `"内置脚本"`。
- 打包后的 GUI 程序没有控制台，`print` 输出不可见；出错时用消息框提示。

### 13.4 SmartScreen 与签名

未签名的 exe 首次运行可能被 SmartScreen 拦截。可以点击“更多信息 → 仍要运行”，或对正式分发的程序进行代码签名。

本机测试可以运行：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\sign.ps1 -Exe hello.exe
```

它会创建/复用一个自签名代码签名证书并签名。自签名只能用于本地信任；SmartScreen 信誉需要可信证书和时间积累。详细步骤见 [signing.md](signing.md)。

### 13.5 包管理

```powershell
agui install https://example.com/mathx.ag mathx
agui install .\mypkg mathx
agui list
agui remove mathx
```

安装到项目根 `packages/`，元数据为 `packages/<name>.json`；`import "mathx"` 会自动在 `packages/` 中查找。详见 16.26。

---

## 14. 与 Python 的差异

Agui 借鉴了 Python 的缩进和部分语法，但不是 Python：

- 有轻量 `class`，但没有继承、装饰器、生成器、列表推导式、类变量/静态方法。
- 异常有 `try/catch/finally` 和 `raise`，没有 `else`；捕获变量是格式化后的错误文本，不是异常对象。
- `lambda` 支持单表达式和多行块，但不能直接写在函数调用的圆括号参数里。
- `import` 是文本级导入：在**当前作用域**执行另一个 `.ag` 文件，支持从 `packages/` 解析已安装包，但不是 Python 的模块对象；没有命名空间、`as`、`from ... import`。
- 切片支持 `[lo:hi:step]`，包括负步长；没有多维切片。
- 没有三元表达式；用 `if` 语句代替。
- `and` / `or` 返回布尔值，而不是其中一个操作数。
- 数字统一是浮点数，`/` 总是浮点除法。
- `for` 只能遍历列表、字符串、字典键或数字范围；不能解包元组。
- 字典保持插入顺序，键最终以字符串保存（数字键会转成字符串）。
- 字符串按 Unicode 码点索引和计算长度。
- 函数参数不支持 `*args` / `**kwargs`；关键字参数是显式支持的。

---

## 15. 已知限制与路线图

当前版本：

- `class` 没有继承、私有成员、类变量、静态方法、运算符重载。
- 多行 lambda 不能直接写在函数调用的圆括号里；先赋值再传。
- `import` 的运行时错误仍显示主文件行号；错误消息包含导入文件路径，但没有独立源码映射。
- 菜单支持两级子菜单、真实快捷键和图标；没有动态插入/删除整段菜单的重建 API。
- 富文本支持追加、字符/段落格式、选区、超链接和插图；插图通过 RTF/剪贴板完成，部分 RichEdit 版本的图片显示可能不稳定。
- 表格支持编辑、排序和编辑事件，但没有虚拟化、多选、列宽拖拽保存、分组。
- treeview 没有拖拽节点、多选；canvas 没有贝塞尔曲线、图片、路径填充。
- 自绘按钮支持纯色背景/文字色，没有渐变、圆角和图标。
- 布局容器支持纵向、网格、锚点和滚动，但没有百分比尺寸、flex 权重和自动换行的水平流式布局。
- 异步任务是协作式的：CPU 密集脚本会在语句间让出，但单条不可中断的原生调用返回前无法响应取消。
- `screenshot()` 默认截活动窗口，多窗口时需要传窗口对象。
- SQLite 依赖 Windows 自带的 `winsqlite3.dll`；包管理器没有版本依赖求解。
- 托盘依赖 Explorer；受限进程可能得到“拒绝访问”，此时 `tray.visible=false` 且 `last_error` 记录原因。
- 自签名 exe 只能建立本机信任；SmartScreen 声誉需要正式证书和时间。

---

## 16. 扩展语法与新控件

### 16.1 切片

```agui
l = [1, 2, 3, 4, 5]
print(l[1:3])       # [2, 3]
print(l[:2])        # [1, 2]
print(l[3:])        # [4, 5]
print(l[-2:])       # [4, 5]
print(l[:])         # [1, 2, 3, 4, 5]
print(l[::-1])      # [5, 4, 3, 2, 1]
print(l[1:5:2])     # [2, 4]
print("abcdef"[1:4]) # bcd
print("abcdef"[::2]) # ace
```

切片完整形式是 `[lo:hi:step]`，`lo` / `hi` / `step` 都可以省略。负数下标从末尾计算，越界会被钳制到合法范围；`hi < lo` 返回空列表/空字符串；负步长从右向左取。切片支持列表和字符串。

### 16.2 lambda

```agui
double = lambda x: x * 2
add = lambda x, y=10: x + y
print(double(21), add(5), add(5, 1))
```

`lambda` 支持单表达式体和多行代码块；支持 `lambda x, y: ...` 和 `lambda(x, y): ...` 两种写法：

```agui
double = lambda x: x * 2
block = lambda x:
    y = x + 1
    y * 2
print(double(4), block(3))
```

单表达式体自动作为返回值；多行块中最后一条表达式语句也会自动返回。块 lambda 必须换行缩进；由于词法规则在括号内会忽略换行，不能直接把块 lambda 写在函数调用的圆括号参数里——先赋值给变量，再传给 `after`、`every`、`async` 或菜单回调：

```agui
show = lambda: msgbox("半秒到了")
after(500, show)
```

### 16.3 try / catch

```agui
try:
    print(1 / 0)
catch err:
    print("捕获：", err)

try:
    risky()
catch:
    print("默认变量名是 err")
```

`catch` 后面的变量名可省略，默认 `err`；捕获到的是格式化后的错误文本（包含行号）。`return`、`break`、`continue` 不会被 `catch` 拦截。`try` 后面必须至少有一个 `catch` 或 `finally` 块；两者可以同时出现，也可以只写 `finally`。

```agui
try:
    raise "自定义错误"
catch err:
    print("捕获：", err)
finally:
    print("无论如何都会执行")

try:
    open_file()
finally:
    close_file()
```

`raise 表达式` 抛出运行时错误，错误文本就是表达式的字符串值；`finally` 在正常结束、异常、`return`/`break`/`continue` 离开时都会执行。`finally` 中如果再次抛出错误，会覆盖原来的异常。

### 16.4 import

```agui
import "lib/math"
print(square(7))
```

- 相对路径基于**当前脚本所在目录**；没有扩展名时自动补 `.ag`。
- 同一个绝对路径只执行一次。
- 导入文件里的函数、变量直接进入当前作用域，不会创建命名空间。
- 导入失败会抛运行时错误，可被 `try/catch` 捕获。

### 16.5 多窗口

```agui
main = window("主窗口", w=560, h=420)
label("主窗口内容")

func open_child():
    child = window("子窗口", w=360, h=240)
    label("子窗口内容", x=20, y=20)
    button("关闭", x=20, y=70).click()

btn = button("打开子窗口", x=20, y=20)
on btn.click:
    open_child()
```

每次 `window()` 都创建新的顶层窗口。后续创建的控件默认进入最近创建/激活的窗口；用 `win.use()` 或 `win.activate()` 切换当前窗口。每个窗口有自己的控件树、菜单栏和背景色。默认是独立窗口；给 `window(..., parent=main)` 传入另一个窗口可创建从属窗口（跟随所有者最小化、保持在所有者之上）。

| 方法 / 属性 | 说明 |
| --- | --- |
| `win.use()` / `win.activate()` | 设为后续控件的目标窗口 |
| `win.close()` | 关闭窗口；最后一个窗口关闭时退出消息循环 |
| `win.center()` / `minimize()` / `maximize()` / `restore()` | 窗口状态 |
| `win.icon(path)` | 设置窗口图标（png/jpg/ico） |
| `window(..., icon="path")` | 创建窗口时直接设置图标 |

### 16.6 图片与窗口图标

```agui
window("图片", w=520, h=360, icon="assets/app.ico")
image("assets/agui.png", w=320, h=200, fit=true)
```

`image(src, x=, y=, w=, h=, fit=true, bg=, fg=)` 使用 GDI+ 解码 png/jpg。`fit=true` 保持比例居中；`fit=false` 拉伸填满。属性 `src` / `path` 可读写，方法 `load()`、`set_src(path)` 重新加载。支持 `click` / `dblclick` 事件。

窗口图标通过 `GdipCreateHICONFromBitmap` 从图片生成 HICON，同时设置大小图标；打包时也可以用 `--icon` 嵌入 exe 图标。

### 16.7 富文本

```agui
r = richedit("", h=240)
r.append("标题", bold=true, color="#1D4ED8", size=22)
r.append("正文", italic=true, underline=true)
r.append("居中段落", align="center")
r.format(0, 2, bold=true, color="#DC2626")
```

| 方法 | 说明 |
| --- | --- |
| `append(text, bold=, italic=, underline=, color=, size=, font=, align=)` | 在文末追加并格式化新文本 |
| `format(start, end, ...)` | 格式化指定字符范围 |
| `select_all()` / `clear()` | 全选 / 清空 |
| `select_range(start, end)` / `selection()` | 选中范围 / 读取范围 |
| `selected_text()` / `insert_text(text, replace=false)` | 读取选中文字 / 插入文字 |
| `replace_selection(text)` / `scroll_caret()` | 替换选区 / 滚动到光标 |
| `link(text, url)` / `insert_image(path_or_bytes, w=, h=)` | 插入超链接 / 图片 |
| `selection_start` / `selection_end` / `link_count` | 选区起点、终点、链接数量 |
| `text` / `readonly` / `fg` / `bg` | 文本、只读、文字色、背景色 |

`align` 支持 `left` / `center` / `right`。事件：`change`、`click`。链接点击会打开系统浏览器；插图会覆盖系统剪贴板，详见 16.27。

### 16.8 表格

```agui
rows = [["张三", "90"], ["李四", "85"]]
t = table(["姓名", "分数"], rows, h=180)

t.add_row(["王五", "88"])
t.set_cell(0, 1, "95")
print(t.get_cell(0, 1), t.row_count, t.column_count)
t.select(1)
```

| 方法 / 属性 | 说明 |
| --- | --- |
| `add_row(values)` / `append_row(values)` | 追加一行 |
| `set_cell(row, col, text)` / `get_cell(row, col)` | 读写单元格 |
| `remove_row(index)` / `clear()` | 删除行 / 清空 |
| `select(index)` | 选中行，`-1` 取消选中 |
| `selected` / `row_count` / `column_count` / `rows` | 选中行 / 行数 / 列数 / 全部数据 |
| `headers` | 表头列表，可读写 |

表格支持单元格编辑和排序：

```agui
t = table(["名称", "数量"], [["苹果", "12"], ["香蕉", "3"]], h=180, editable=true)
t.sort(1)                 # 按第 2 列升序
t.sort(1, false)          # 降序
t.edit_cell(0, 1)         # 打开第 1 行第 2 列的编辑框
t.set_edit_text("99")
t.end_edit(true)          # 提交；false 取消
```

| 方法 / 属性 | 说明 |
| --- | --- |
| `sort(col, asc=true)` | 按列排序；数字列按数值，其他按字符串 |
| `edit_cell(row, col)` / `editing` | 打开编辑框 / 是否正在编辑 |
| `set_edit_text(text)` / `edit_text` | 设置/读取编辑框文字 |
| `end_edit(save=true)` | 提交或取消当前编辑 |
| `editable` | 是否允许双击编辑 |

单击列头会按该列排序；双击单元格进入编辑，Enter 或失焦提交，Esc 取消。编辑框会被裁剪在表格可见区域内，行号越界时 `edit_cell` 直接返回。修改 `headers` 会保留已有行数据。

事件：`select`、`click`、`dblclick`、`edit`。`edit` 在单元格保存后触发，可以用 `t.selected` 和 `t.get_cell(...)` 读取结果。

数据量较大时使用虚拟表格，只渲染当前可见行：

```agui
rows = []
for i in range(100000):
    rows.append([str(i), "第 " + str(i) + " 行"])
t = table(["编号", "内容"], rows, virtual=true, h=420)
```

虚拟模式下 `row_count` 仍是完整行数，滚动条按总行数计算；`set_cell`、`sort`、`rows` 直接操作内存中的行数据，不创建对应数量的 Win32 列表项。虚拟表格仍可选中和双击编辑，但编辑框定位以当前可见区域为准。

### 16.9 视觉样式与自绘按钮

Agui 启动时通过 activation context 激活 Common Controls v6，按钮、复选框、滚动条等跟随系统现代主题，不需要额外清单文件。

普通按钮的 `fg` / `bg` 在原生 Win32 下几乎无效，因此设置任一颜色会自动切换为 owner-drawn：

```agui
button("保存", fg="#FFFFFF", bg="#2563EB", w=140)
button("删除", fg="#FFFFFF", bg="#DC2626", w=140)
```

自绘按钮保留键盘焦点、默认按钮外框和禁用态；目前只支持纯色背景，不支持渐变/圆角。

### 16.10 后台任务、取消与递归上限

```agui
func work():
    sleep(1)
    return "done"

func done():
    label_status.text = "加载完成"

t = async(work, then=done)
print(t.status)       # running
print(t.wait(2))      # true
print(t.result)       # done
```

`async(work, then=callback)` 返回 `task` 对象：

- `work` 在后台 goroutine 中**协作式**执行；每执行若干语句会主动让出解释器锁，因此后台做计算时 UI 事件仍能响应。
- 完成后 `then` 被投递回 UI 线程；出错时报告错误，`then` 不执行。
- 不要在 `work` 里直接修改控件；更新界面放在 `then`。
- `task.status`：`running`、`done`、`failed`、`cancelled`。
- `task.wait(timeout=5)`：等待完成，返回是否已完成；`0` 表示无限等待。
- `task.cancel()`：请求取消；在 `sleep`、文件、HTTP 等可让出位置会尽快抛出“已取消”。
- `task.result` / `task.error`：结果或错误信息。

> 注意：单条不可中断的原生调用（例如一个很慢的同步函数）只有在返回后才会响应取消；程序退出时 Agui 会取消所有未完成任务，最多等待约 2 秒。

递归深度可调：

```agui
print(get_recursion_limit())   # 默认 512
set_recursion_limit(200)       # 范围 8..100000，返回旧值
```

超过限制会抛出“函数调用层数太深（超过 N 层），可能是无限递归”，可以配合 `try/catch` 捕获。

### 16.11 调试、init 与打包资源

```powershell
agui init demo                 # 生成 demo/main.ag 模板
agui run app.ag --debug        # 打印事件日志，出错时输出全局变量
agui build app.ag -o app.exe --icon assets\app.ico --asset assets --asset data\config.json `
  --product "我的工具" --company "Agui" --description "示例" --file-version 1.2.3.4
```

- `--asset` 可重复；目录以目录名为虚拟前缀挂载，入口脚本同目录的 `.ag` 会自动包含。
- 脚本里的 `import`、`read_file`、`image`、窗口图标、菜单图标优先读内嵌资源，再读磁盘。

- `--debug` 设置 `AGUI_DEBUG=1`，`Widget.emit` 输出 `[debug] 事件 kind.event`；运行错误时把全局变量（排除原生函数）打印到 stderr。
- 脚本里可以调用 `debug_vars()` 获取全局变量字典。
- `agui build` 使用 `internal/winres` 直接写 PE `.rsrc` 节，嵌入 RT_GROUP_ICON/RT_ICON 和 VS_VERSIONINFO，不依赖 windres 或外部工具。
- 签名见 [signing.md](signing.md)。


### 16.12 轻量 class

```agui
class Counter:
    func init(self, start=0):
        self.n = start

    func add(self, x):
        self.n += x
        return self.n

c = Counter(10)
print(c.add(5), c.n)      # 15 15
```

- `class Name:` 的块里只能放 `func` 方法（可以写 `pass` 占位）。
- 实例化 `Name(...)` 时自动调用 `init`；没有 `init` 且传了参数会报错。
- 方法第一个参数可以写 `self`；写 `self` 时会自动绑定实例，不写也可以访问实例字段。
- `self.field` 读写实例字段；实例字段保存在内部的 Map 中。
- 方法调用使用点语法：`obj.method(...)`；`type(obj)` 返回 `object`，`type(Class)` 返回 `class`。
- 这是轻量对象模型：没有继承、私有成员、类变量、静态方法或运算符重载。

### 16.13 菜单快捷键与图标

```agui
f = menu("文件")
open_item = f.item("打开", accel="Ctrl+O", icon="assets/open.png")
save_item = f.item("保存", accel="Ctrl+S")
sub = f.submenu("最近打开")
sub.item("a.ag")

on open_item.click:
    msgbox("打开", "Ctrl+O 触发了菜单项")
```

- `accel` 是**真实快捷键**，不是只显示在菜单文字里；支持 `Ctrl` / `Shift` / `Alt` 组合、`F1`–`F24` 以及 Enter、Esc、Tab、Space、方向键等。
- 菜单项图标接受 png/jpg（内部用 GDI+ 生成 HBITMAP，再 `SetMenuItemBitmaps`）；可勾选项保留勾选标记。
- `item(...)` 还支持 `checkable=true`、`checked=true`、`enabled=false`。
- 属性 `accel` / `icon` / `checked` / `enabled` 可在运行时修改；修改 `accel` 或 `enabled` 会重建快捷键表。
- 新增只读属性 `has_icon` 可用于检查图标是否加载成功。

### 16.14 groupbox 分组容器

```agui
win = window("分组", 640, 520)
g = groupbox("账户", h=230)
label("用户名", parent=g)
name = edit("", parent=g)
label("密码", parent=g)
edit("", parent=g, password=true)

opts = groupbox("选项", h=120)
checkbox("启用通知", parent=opts, checked=true)
```

- `groupbox(text, ...)` 创建一个带标题的分组框。
- 组内控件传 `parent=groupbox对象`；坐标相对于 groupbox 客户区，自动布局从标题下方开始。
- groupbox 本身仍按窗口的自动纵向布局排列；窗口缩放时会重新计算组内控件。
- `parent` 目前只接受 groupbox，传其他控件会报错。

### 16.15 treeview

```agui
t = tree([
    ["src", ["main.go", ["gui", ["window.go"]]]],
    ["docs", ["language.md"]],
    "README.md",
], h=280)

print(t.count, t.selected)
t.select("window.go")
on t.select:
    print("选中", t.selected)
on t.expand:
    print("展开/收起")

t.add_child("docs", "新文档.md")
print(t.has_node("新文档.md"))
```

树节点可以写成字符串、`[文字, 子节点列表]` 或带 `text` / `children` 的 Map。方法：`add(text)` / `add_root(text)` / `add_child(parent, text)`、`select(文字或句柄)`、`expand_all()` / `collapse_all()`、`clear()`；属性 `selected` / `count`；只读方法 `has_node(text)`。事件：`select`、`expand`、`dblclick`。

### 16.16 canvas 画布

```agui
c = canvas(w=520, h=320, bg="#FFFFFF")
c.rect(20, 20, 180, 100, color="#2563EB", fill=true)
c.circle(350, 90, 60, color="#0F766E", fill=false, width=3)
c.line(20, 220, 480, 220, color="#DC2626", width=2)
c.text(30, 250, "Agui Canvas", color="#111827", size=20)
on c.click:
    print("点击坐标", c.mouse_x, c.mouse_y)
```

方法：`line(x1,y1,x2,y2,color=,width=)`、`rect(x,y,w,h,color=,fill=,width=)`、`circle(x,y,r,color=,fill=,width=)`、`text(x,y,内容,color=,size=,font=)`、`redraw()`、`clear()`。属性：`mouse_x` / `mouse_y` / `count`；事件：`click`。坐标使用逻辑像素，绘制内容在高 DPI 下自动缩放。

### 16.17 文件拖拽

```agui
win = window("拖拽", 480, 320)
status = label("把文件拖进来")
on win.drop:
    print("收到", len(win.dropped), "个文件")
    status.text = win.dropped[0]
```

窗口自动接受文件拖放；拖入时触发 `drop` 事件，`win.dropped` / `win.dropped_files` 返回路径列表。自动化测试可以调用 `win.simulate_drop([...])` 模拟一次拖放（会走完整的 HDROP 解析路径）。

### 16.18 JSON、base64 与 HTTP

```agui
data = {"name": "Agui", "items": [1, 2, 3]}
s = json_encode(data)
print(s)
back = json_decode(s)
print(back.name, back.items[1])

print(base64_encode("hello"))
print(base64_decode("aGVsbG8="))

r = http_get("https://example.com", timeout=10)
print(r.status, r.ok, len(r.body))
if r.ok:
    print(r.headers["Content-Type"])
```

| 函数 | 说明 |
| --- | --- |
| `json_encode(value, pretty=false)` | 编码为 JSON 字符串；Map 键按字母序输出 |
| `json_decode(text)` | 解码 JSON；对象→dict、数组→list、数字→数字，拒绝尾随垃圾 |
| `base64_encode(text, urlsafe=false)` | Base64 编码；`urlsafe=true` 用 URL-safe 变体 |
| `base64_decode(text, urlsafe=false)` | Base64 解码，标准解码失败时回退 Raw Standard |
| `http_get(url, headers=, timeout=15)` | GET，返回 `{status, ok, body, headers, url}` |
| `http_post(url, body="", headers=, timeout=15)` | POST |
| `http_request(method, url, body="", headers=, timeout=15)` | 任意方法 |

HTTP 响应最多读取 8 MB；网络错误会作为运行时错误抛出，可被 `try/catch` 捕获。更完整的 HTTP 客户端（PUT/PATCH/DELETE、下载/上传、cookies、重试、代理、`raise=false`）见 16.21。

### 16.19 布局容器：panel、scroll、grid

```agui
card = panel(x=20, y=20, w=360, h=220)
label("账户", parent=card)
edit("用户名", parent=card)
button("保存", parent=card)

list = scroll(x=400, y=20, w=300, h=220)
for i in range(20):
    label("第 " + str(i) + " 项", parent=list)

pad = grid(cols=3, x=20, y=260, w=420, h=160)
for k in ["7", "8", "9", "4", "5", "6"]:
    button(k, parent=pad)

ok = button("确定", anchor="right bottom", x=20, y=20, w=120, h=36)
```

- `panel()`：无滚动容器，适合卡片、分区。
- `scroll()`：带垂直滚动条，支持鼠标滚轮和 `scroll_y` 属性；内容超出高度时自动出现滚动条。
- `grid(cols=2)`：按列数从左到右、从上到下排列子控件，行高取该行最高控件。
- 容器可嵌套：panel 里可以放 groupbox、scroll、grid，groupbox 里也可以放 panel。
- `anchor` 支持 `left`、`right`、`top`、`bottom` 及其组合；`right bottom` 贴右下，`left right` 横向拉伸，`top bottom` 纵向拉伸。
- `visible=false` 的控件不显示也不占位；恢复显示会重新布局父容器。
- 修改容器尺寸或窗口缩放会触发递归重排。

常用复合容器：

```agui
bar = toolbar(h=44)
button("新建", w=90, parent=bar)
button("打开", w=90, parent=bar)

sp = splitter(vertical=false, h=240)
left = sp.pane(0)
right = sp.pane(1)
label("左侧", parent=left)
label("右侧", parent=right)

tabs = tabs(h=240)
p1 = tabs.add_page("概览")
label("第一页", parent=p1)
p2 = tabs.add_page("详情")
label("第二页", parent=p2)
tabs.active = 1

pg = propertygrid([["名称", "Agui"], ["版本", "0.9"]], h=120)
```

- `toolbar()`：横向排列子控件，适合按钮条。
- `splitter(vertical=false)`：左右或上下分栏，拖动中间 6px 分隔条调整比例；`pane(0)` / `pane(1)` 取得两栏容器，`ratio` 读写比例（0.1–0.9）。
- `tabs()`：`add_page(title)` 创建页面容器并返回，`active` 切换当前页；页面内控件用 `parent=页面对象`。
- `propertygrid(rows)`：两列表格封装，`set_prop(name, value)` / `get_prop(name)` 读写属性行。
- 这些容器都可以嵌套 panel、groupbox、grid 等。

### 16.20 GUI 补充控件

```agui
win = window("补充控件", 720, 620, center=true, opacity=1.0)
statusbar("就绪\tDPI 自动\tAgui 0.5")

radio("标准", checked=true)
radio("紧凑")

date("2026-11-01")
time("15:30:00")

button("悬停查看提示", tooltip="这是一个原生 tooltip")
```

- `radio(text, checked=false, ...)`：单选按钮；同组单选需要自行用 `parent` 放进同一个容器，事件为 `click` / `change`。
- `statusbar(text)`：底部状态栏，用 `\t` 分段，自动停靠在窗口底部；属性 `text` / `parts`，事件 `click` / `dblclick`。
- `date(value="")` / `time(value="")`：原生日期/时间选择器；日期接受 `YYYY-MM-DD`，时间接受 `HH:MM` 或 `HH:MM:SS`；只写时间时日期补为今天。属性 `text` / `value`，方法 `set_value()` / `today()`，事件 `change`。
- `tooltip` 可作为任意控件的通用属性（`button(..., tooltip="提示")` 或 `w.tooltip = "提示"`），使用原生 tooltips_class32；自动化测试可调用 `w.hover()` 显示提示，并用 `w.tooltip_visible` 断言。
- 窗口属性：`center=true` 启动时居中；`topmost` 置顶；`opacity` 0.0–1.0；`state` 为 `normal` / `maximized` / `minimized`；方法 `center()`、`flash()`。
- 状态栏、日期、时间、单选按钮同样参与自动布局；状态栏不占纵向流。

**IME 与无障碍名称**：`edit`、`richedit` 使用原生 Win32 编辑控件，中文输入法组合输入由系统处理，无需额外配置。所有控件支持 `name` / `caption` 通用属性，写入窗口文本：

```agui
e = edit("", w=400)
e.name = "姓名输入框"

c = canvas(w=400, h=200)
c.name = "游戏画布"
```

普通 Win32 控件（按钮、编辑框、表格等）由系统提供 UIA 名称；自绘的 `canvas`、`image` 通过 `name` 暴露名称，UIA 会显示为 `Pane`。当前没有为自绘控件实现完整的 `IAccessible` 子节点/角色，`canvas` 内部图形仍不能逐个被读屏软件枚举。

### 16.21 HTTP 客户端增强

```agui
r = http_get("https://example.com", timeout=10, retry=2, raise=false)
print(r.status, r.ok, r.content_type, r.length)

r = http_json("https://api.example.com/items", {"name": "Agui"})
print(r.json)

http_download("https://example.com/logo.png", "logo.png")
http_upload("https://example.com/upload", files={"file": "logo.png"})

print(url_parse("https://example.com/a?x=1"))
```

- 方法：`http_get`、`http_post`、`http_put`、`http_patch`、`http_delete`、`http_head`、`http_request`；`http_json(url, data, method="POST")` 自动 JSON 编码。
- 通用关键字参数：`headers`、`timeout`（秒，默认 15）、`cookies`、`auth=[用户名, 密码]`、`follow_redirects`、`max_redirects`、`proxy`、`insecure`、`retry`、`retry_delay`、`raise`。
- 请求体可以用 `body`、`json`、`form` 或 `files`；`http_download` 把响应原样写入文件，`http_upload` 以 multipart POST 上传。
- 返回字典：`status`、`ok`、`body`、`headers`、`url`、`content_type`、`length`、`elapsed`、`cookies`、`json`。HTTP 4xx/5xx 默认抛错；`raise=false` 时返回带 `error` 的字典。
- 响应体上限 32 MB；请求会释放解释器锁，放在 `async` 中不会冻结界面。
- URL 工具：`url_encode`、`url_decode`、`query_encode(dict)`、`url_parse(url)`。

### 16.22 WebSocket 客户端

```agui
ws = ws_connect("ws://127.0.0.1:8765", timeout=5)
print(ws.connected, ws.url)

on ws.message:
    print("收到", ws.message, "类型", ws.type)
    ws.close()

on ws.close:
    print("连接关闭")

ws.send("hello")
t = async(lambda: sleep(3000))
t.wait(5)
```

- `ws_connect(url, headers=, timeout=15, protocols=, origin=, insecure=false)` 建立 RFC6455 客户端连接，支持 `ws://` 与 `wss://`。
- 事件：`open`、`message`、`close`、`error`；`message` 事件里读 `ws.message` / `ws.text` / `ws.data`，`ws.type` 为 `text` 或 `binary`。
- 方法：`send()` / `send_text()`、`send_binary()`、`recv(timeout=5)`（同步等待一条消息）、`ping()`、`close()`。
- 属性：`url`、`protocol`、`connected`、`closed`、`message`、`type`、`count`、`last_error`。
- 无窗口（命令行）脚本也能收到事件；GUI 脚本的事件回调会回到 UI 线程。
- 服务端示例可用 `scripts/ws_echo.py`（依赖 Python websockets）或任意 echo 服务。

### 16.23 bytes 字节数组

`bytes` 是独立的二进制值类型，不会像字符串那样做编码转换；适合文件、网络、图片和 SQLite BLOB。

```agui
b = bytes("Agui")          # UTF-8 字节
b = b + "!"                # bytes + 字符串
print(len(b), b.hex())     # 5 4167756921
print(b[0])                # 65
print(b.slice(0, 2).to_string())
b.push(33)
write_bytes("demo.bin", b)
c = read_bytes("demo.bin")
print(c.to_string())
```

- 构造/转换：`bytes(x)`、`to_bytes(x)`、`bytes_to_string(b)`、`hex_encode(b)`、`hex_decode(s)`。
- 文件：`read_bytes(path)`、`write_bytes(path, b)`、`append_bytes(path, b)`。
- 方法：`len()`、`push(...)` / `append(...)`、`pop()`、`clear()`、`slice(start, end)`、`to_string()` / `string()`、`hex()`。
- `bytes` 支持 `+`、`[]` 下标、切片（含步长）、`for` 遍历、`in`。
- WebSocket 二进制帧和 HTTP 的 `body_bytes` 使用 `bytes`；`http_download` 写入原始字节。

### 16.24 托盘、剪贴板图片与音频

```agui
t = tray(icon="assets/app.ico", tooltip="Agui")
t.item("打开", lambda: msgbox("打开"))
t.item("退出", lambda: quit())
t.notify("Agui", "已启动")

png = clipboard_image()       # bytes（PNG）
if png != nil:
    write_bytes("clip.png", png)

play_wav("assets/beep.wav")   # 异步播放
stop_wav()
```

- `tray(icon=, tooltip=)`：属性 `visible/added/last_error/tooltip/icon`；方法 `item/separator/notify/set_icon/set_tooltip/minimize_to_tray/remove/hide/show`；事件 `click/dblclick`。
- 托盘依赖 Windows Explorer 的消息窗口；某些受限进程会得到“拒绝访问”，此时对象仍可用，`last_error` 记录原因，`visible=false`。
- `clipboard_image()` 读取 CF_DIBV5/CF_DIB 并编码为 PNG；没有图片时返回 `nil`；支持 24/32 位未压缩和 32 位 BITFIELDS。
- `play_wav(path, loop=false)`、`stop_wav()` 使用 winmm；只支持 WAV。

### 16.25 SQLite

Windows 10/11 自带 `winsqlite3.dll`，不需要额外安装。

```agui
db = sqlite_open(":memory:")
db.exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, score REAL)")
r = db.execute("INSERT INTO users(name, score) VALUES (?, ?)", "Alice", 9.5)
print(r["last_id"], r["changes"])

rows = db.query("SELECT id, name, score FROM users")
for row in rows:
    print(row["id"], row["name"], row["score"])

print(db.tables)
db.close()
```

- `sqlite_open(path, readonly=false, create=true)`；路径 `":memory:"` 为内存库。
- `db.exec(sql)` 执行无返回语句，返回 changes；`db.execute(sql, ...)` 绑定参数并返回 `{changes, last_id}`；`db.query(sql, ...)` 返回 `List[Map]`。
- 属性：`tables`、`last_error`、`path`、`closed`；方法：`exec/execute/query/close`。
- 参数支持字符串、数字、bytes、nil；浮点通过文本绑定并依赖 SQLite 的 REAL affinity。
- SQLite 调用是阻塞的；需要不卡 UI 时放进 `async`。

### 16.26 包管理器

```powershell
agui install https://example.com/mathx.ag mathx
agui install .\mypkg mathx
agui list
agui remove mathx
```

包安装在项目根的 `packages/` 下，元数据写入 `packages/<name>.json`。安装目录时，入口优先 `<name>.ag`、`<name>/<name>.ag`、`index.ag`、`main.ag`。

```agui
import "mathx"
print(add(2, 3))
```

- 包是文本级包含：导入后定义直接进入当前环境，没有命名空间；推荐用唯一前缀避免冲突。
- 支持 HTTP(S) 直接 `.ag` 文件（20MB 上限、60 秒超时）和本地文件/目录。
- `list` 显示名称、版本和入口。

### 16.27 RichEdit 选区、链接与图片

```agui
r = richedit("Agui", h=220)
r.append(" 富文本", bold=true, color="#1D4ED8")
r.link("官网", "https://example.com")
r.insert_image("assets/agui.png", w=160, h=100)

r.select_range(0, 4)
print(r.selected_text)       # Agui
print(r.selection_start, r.selection_end)
print(r.link_count)
```

- 方法：`select_range(start, end)`、`selection()` 返回 `[start,end]`、`selected_text()`、`insert_text(text, replace=false)`、`replace_selection(text)`、`insert_image(path_or_bytes, w=, h=)`、`link(text, url)`、`scroll_caret()`。
- 属性：`selection_start`、`selection_end`、`selected_text`、`link_count`、`selection`（可写 `[start,end]`）。
- 链接点击用 `ShellExecuteW` 打开系统浏览器。
- 图片通过“RTF + 剪贴板 + WM_PASTE”插入，会覆盖系统剪贴板；部分 RichEdit 版本对图片显示支持有限，建议插入后 `redraw()` 或重新聚焦。
- 支持 PNG；内部转成 JPEG RTF，宽高按传入值缩放。

### 16.28 Game Kit：2D 小游戏运行时

Game Kit 是建立在窗口、canvas 和 Win32 定时器上的轻量 2D 运行时，不是完整引擎。
它提供固定步长循环、键盘/手柄、相机、精灵、粒子、碰撞、音频、全屏和 JSON 存档。

最小示例：

```agui
win = window("小游戏", 640, 480, bg="#0b1220")
c = canvas(w=600, h=380, bg="#0f172a")

px = 100
py = 100

func update(dt):
    if key_down("right"):
        px = px + 220 * dt
    if key_down("left"):
        px = px - 220 * dt
    c.batch_begin()
    c.clear()
    c.circle(px, py, 18, color="#38bdf8", fill=true)
    c.text(12, 12, "FPS " + str(round(1 / dt)), color="#e2e8f0", screen=true)
    c.batch_end()

game_loop(update, fps=60)
```

#### 16.28.1 循环与时间

| API | 说明 |
| --- | --- |
| `game_loop(update, draw=null, fps=60)` | 启动固定步长循环。`update(dt)` 每步调用；`draw(alpha)` 每帧调用；`fps` 必须是关键字或第三个参数 |
| `game_stop()` / `game_pause()` / `game_resume()` | 停止 / 暂停 / 恢复 |
| `game_set_time_scale(v)` / `time_scale(v)` | 0 暂停时间，1 正常，2 加速 |
| `game_running()` / `game_paused()` | 查询状态 |
| `dt` / `game_time` / `game_frame` | 当前固定步长、累计游戏时间、帧号 |

固定步长默认 1/60 秒；每帧最多补 8 步，避免卡顿后追帧过久。
`update` 内不要做阻塞 IO；长任务用 `async` 或拆到多帧。

#### 16.28.2 键盘、快捷键与手柄

```agui
bind("left", "a", "left")
bind("jump", "space", "w", "up")

if action_down("left"):
    px = px - 200 * dt
if action_pressed("jump"):
    vy = -500
```

- `key_down(name)` / `key_pressed(name)` / `key_released(name)`：单键查询；
- `bind(action, key1, key2, ...)` 与 `action_down/action_pressed`：动作映射；
- `keys_down()`：当前按下的键列表；
- 窗口事件：`on win.keydown:`、`on win.keyup:`，`win.key` 是按键名；
- 常用键名：`a`..`z`、`0`..`9`、`space`、`enter`、`escape`、`left/right/up/down`、`shift/ctrl/alt`、`f1`..`f12`。
- 手柄：`gamepad_connected()`、`gamepad_button("a")`、`gamepad_axis("left_x")`；支持 XInput 1.4/1.3/9.1.0。

#### 16.28.3 相机

```agui
camera(px - 300, py - 200, 1.5)
camera_bounds(0, 0, 2000, 1200)
camera_shake(8, 0.2)
```

`camera(x, y, zoom)` 设置世界坐标原点、缩放；无参返回 `{x,y,zoom}`。
`screen(x, y)` 把世界坐标转换为屏幕坐标。`camera_shake(幅度, 秒)` 产生短震动。
相机变换只影响 canvas 绘制，不影响窗口控件。

#### 16.28.4 canvas 绘制与精灵

```agui
c.clear()
c.rect(20, 20, 100, 40, color="#2563eb", fill=true)
c.circle(200, 80, 24, color="#f59e0b", fill=true)
c.line(0, 200, 600, 200, color="#64748b", width=2)
c.text(20, 250, "Hello", size=20, outline="#000000", outline_width=2)

# 整张图片
c.image("assets/player.png", 100, 100, w=64, h=64)

# 精灵表：4 列 2 行，取第 3 帧
c.sprite("assets/hero.png", 100, 100, w=48, h=48, frames=8, cols=4, frame=3)

# 二维 tilemap：0 表示空，其余为图块索引
c.tilemap(level, "assets/tiles.png", 32, 32)
```

- `batch_begin()` / `batch_end()`：一帧内多次绘制只触发一次重绘；`flip()` 等价于 `batch_end()`；
- 绘制先 `clear()`，再画，最后 `batch_end()`；
- 文字支持 `outline`/`outline_width`；字体按 family/size/bold/italic 缓存；
- 图片支持 png/jpg/bmp/gif；`screen=true` 的文字不参与相机变换；
- `tilemap(tiles, path, tile_w, tile_h, x=, y=, ...)` 绘制二维列表。

精灵表动画与帧事件：

```agui
walk = anim(8, fps=10, loop=true)   # 8 帧，循环
on walk.frame:
    c.text(12, 40, "frame " + str(walk.frame), screen=true)

func update(dt):
    walk.update(dt)                  # 只在帧变化时触发 walk.frame
    c.batch_begin()
    c.clear()
    c.sprite("assets/hero.png", 100, 100, w=48, h=48,
             frames=8, cols=4, frame=walk.frame)
    c.batch_end()
```

`anim(frames, fps=12, loop=true)` 返回动画对象；`update(dt)` 推进，`frame/fps/playing/done` 可读，`reset()/play()/pause()/stop()` 可调用。

#### 16.28.5 粒子

```agui
particles_emit(x, y, count=12, speed=120, life=0.5, size=3,
               color="#facc15", gravity=200)
particles_update(dt)
particles_draw(c)
```

`particles_draw` 可省略 canvas 参数，自动使用第一个 canvas。`particles_clear()` 清空。
粒子按生命周期对颜色做简单的背景混合（alpha fade），不是 GPU 混合模式。

#### 16.28.6 碰撞与数学

| API | 说明 |
| --- | --- |
| `aabb(ax, ay, aw, ah, bx, by, bw, bh)` | 轴对齐矩形相交 |
| `ray_rect(x, y, dx, dy, rx, ry, rw, rh)` | 射线与矩形，返回 `{hit,t,nx,ny}` |
| `tilemap_collide(tiles, x, y, tile_w, tile_h)` | 点是否落在非 0 图块 |

#### 16.28.7 音频

```agui
audio_play("assets/bgm.mp3", channel="bgm", loop=true, volume=60)
audio_play("assets/jump.wav", channel="sfx", volume=100)
audio_stop("sfx")
audio_stop_all()
if audio_playing("bgm"):
    ...
```

使用 Windows MCI，支持系统编解码器可播放的 WAV/MP3/OGG；多声道按名称隔离。

#### 16.28.8 全屏、无边框、存档

```agui
fullscreen(true)
borderless(true)

save_json("save.json", {"level": 3, "hp": 80})
data = load_json("save.json", null)
```

窗口方法也提供 `win.fullscreen(true)` / `win.borderless(true)`。
`save_json` 原子写入（临时文件 + rename），`load_json` 文件不存在时返回默认值。

#### 16.28.9 示例

- `examples/game_platformer.ag`：平台跳跃，含相机、AABB、粒子、存档；
- `examples/game_cards.ag`：回合制卡牌，含按钮、能量、随机伤害、快捷键、存档；
- `examples/game_smoke.ag`：最小循环与绘制自检。

运行：

```powershell
.\agui.exe run examples\game_platformer.ag
.\agui.exe run examples\game_cards.ag
```

已知边界：没有完整物理引擎、地图编辑器或资源管线；MP3/OGG 依赖系统 MCI 编解码器；全屏/手柄在无设备环境未做真机验证。

### 16.29 单文件 Android 构建

同一个 `.ag` 文件可以面向 Windows 或 Android。只在入口文件开头写一次 `#target`，`agui build` 会按平台产出 `.exe` 或 `.apk`：

```agui
#target android
#name 星野养成
#package com.agui.star
#version 0.1.0
#icon assets/icon.png

win = window("星野养成", w=420, h=640)
```

- `#target windows` 为缺省值；只允许一个 `#target`，且必须在代码之前。
- `#name` 是应用显示名；`#package` 是 Android 包名；`#version` 是版本名；`#icon` 支持 PNG 路径（相对脚本目录）或 data URI；`#permission android.permission.CAMERA` 可重复，声明运行时权限。
- `agui build app.ag` 生成 `app.exe` 或 `app.apk`；`agui check app.ag` 会校验文件头与包名。
- Android 端使用原生 View，不使用 WebView；支持 `window/actionbar/panel/label/button/edit/checkbox/progress/image`，事件 `click/change/close`，以及 `after/every/cancel_timer/quit/toast/alert`。
- `actionbar(title)` 创建顶部应用栏，`bar.add_action("刷新")` 返回一个可监听 `click` 的菜单按钮；适合放置返回、刷新、关于等常用操作。
- 系统能力：`share(text)`、`open_url(url)`、`vibrate(毫秒)`、`notify(title, text)`、`clipboard()`、`set_clipboard(text)`；通知和震动需要在文件头用 `#permission` 声明 `POST_NOTIFICATIONS`/`VIBRATE`（Android 13+ 通知还需运行时授权）。
- `table(headers, rows)` 在 Android 上使用原生 `TableLayout`，适合几十行以内的配置/结果展示；不是虚拟列表，大数据请使用 `panel` + 分页或 `canvas`。
- `tabs()` 创建标签页容器，`p = t.add_page("标题")` 返回页面容器，页面内控件用 `parent=p`；`t.active = 1` 切换页面。
- `pager()` 创建可左右滑动的页面容器，`p = pg.add_page("标题")` 返回页面容器；`pg.active = 1` 切换页面，左右滑动也会更新 `active` 并触发 `change` 事件。Android 端用 `ViewFlipper` + 手势实现，不依赖 AndroidX。
- `refresh()` 创建下拉刷新容器，子控件用 `parent=容器`；下拉超过阈值后触发 `refresh` 事件。示例：
  ```agui
  rf = refresh()
  label("下拉刷新", parent=rf)
  on rf.refresh:
      status.text = "已刷新"
  ```
- `tree(items)` 在 Android 上用缩进标签模拟树：`[["父节点", [["子节点", []]]]]`，点击节点触发 `select` 事件，`tree.selected` 是扁平索引；不支持展开/折叠和拖拽。
- `rich("...")` 在 Android 上渲染只读富文本，支持 `[b]粗体[/b]`、`[i]斜体[/i]`、`[color=#RRGGBB]颜色[/color]`；不支持编辑和超链接。
- `request_permissions("android.permission.CAMERA", "android.permission.RECORD_AUDIO")` 可一次申请一组运行时权限；每个结果通过窗口 `permission` 事件返回，`win.permission_name` / `win.permission_granted` 读取最近一次结果。权限必须在脚本头部用 `#permission` 声明。
- `permission_rationale(name)` 判断是否适合再次弹系统授权框；`open_settings()` 打开当前应用设置页，用于用户永久拒绝后的引导。
- `intent(action, data="", mime="")` 启动任意 Android Intent，例如 `intent("android.settings.SETTINGS")`、`intent("android.intent.action.VIEW", "https://example.com")`。`open_url` 是 VIEW 的便捷封装。
- `foreground(title, text)` / `stop_foreground()` 启动/停止 Android 前台服务，通知栏会显示常驻通知；Android 14 使用 `dataSync` 类型，脚本需声明 `FOREGROUND_SERVICE` 与 `FOREGROUND_SERVICE_DATA_SYNC`。
- 深链：清单注册 `agui://` scheme；应用收到链接时触发窗口 `intent` 事件，`win.intent` 读取 URL。启动 Activity 时和 `onNewIntent` 都会分发。
- `camera()` 调用系统相机，返回缩略图并保存到缓存 JPEG，随后触发窗口 `file` 事件，路径在 `win.last_file`。
- `location()` 返回最后已知位置 `"纬度,经度"`，无缓存定位时返回空字符串；需要 `ACCESS_FINE_LOCATION` 或 `ACCESS_COARSE_LOCATION`。
- `start_record()` / `stop_record()` 使用麦克风录音，停止后触发窗口 `file` 事件，路径在 `win.last_file`；需要 `RECORD_AUDIO`。
- `sensor("accelerometer")` 返回最近一次加速度 `x,y,z`；也支持 `"gyroscope"`、`"light"`。首次调用时注册监听，无数据返回空字符串。
- `list(items)` 在 Android 上使用 `ListView` + `ArrayAdapter`，行视图会复用，适合上千行长列表；目前只读，没有选中事件和自定义行布局。
- `device_info()` 返回厂商/型号/Android 版本；`battery()` 返回电量百分比（-1 表示不可用）；`network()` 返回 `wifi` / `cellular` / `ethernet` / `online` / `offline`。
- `confirm(title, message)` 显示确定/取消对话框，结果在窗口 `confirm` 事件中读取 `win.confirm`（true=确定，false=取消）。`alert(title, message)` 只显示信息。
- `save_file(name, content)` 打开系统“另存为”选择器；保存完成触发窗口 `saved` 事件，`win.saved` 是目标文档 URI。写入使用 SAF，不需要存储权限。
- 构建 Android 需要 `android/runtime/agui.aar`（由 `gomobile bind ... ./mobile` 生成）以及 SDK/JDK；`--asset` 可把额外资源打进 APK。
- 当前 Native 后端只支持 arm64-v8a；Windows 专属的 `menu/tray` 等能力未移植，但 ActionBar、表格、tabs、tree、富文本、列表和主要系统 API 已有原生实现。
- 原生 Canvas/Game Kit 子集已支持：`canvas`、`game_loop`、触摸、`camera`、`aabb`、`save_json/load_json`；完整 Game Kit 仍以 Windows 为准。
- release 构建：`agui build app.ag --release --ks release.keystore --ks-pass ... --key-alias agui --key-pass ... --version-code 2`。
- 生命周期：`onPause/onResume` 转发到解释器，返回键触发窗口 `close`；`share(text)` 调系统分享。
- 系统文件选择：`pick_file("text/plain")` 打开 SAF 选择器；结果复制到应用缓存后触发窗口 `file` 事件，用 `win.file` 读取路径（不需要存储权限）。
- 运行时权限：`#permission` 声明权限后，调用 `request_permission("android.permission.CAMERA")`；结果触发窗口 `permission` 事件，用 `win.permission` / `win.permission_granted` 读取。


### 16.30 工具链：watch / LSP / debug

```powershell
.\agui.exe watch examples\hello.ag
.\agui.exe lsp
.\agui.exe debug examples\hello.ag
```

- `watch`：350ms 轮询入口文件 mtime，变更后终止并重启脚本；Ctrl+C 退出。
- `lsp`：stdio 上的 JSON-RPC，提供 `textDocumentSync=1`、基础补全和 `publishDiagnostics`；适合编辑器做最简接入。
- `debug`：设置 `AGUI_TRACE=1`，每条语句执行前打印 `[debug] 文件:行 源码`；不是断点式调试器。

### 16.31 包命名空间与 lockfile

`import "mathx"` 会执行模块并把新增顶层定义绑定到同名命名空间，因此可以写 `mathx.add(2, 3)`；为兼容旧脚本，直接 `add(2, 3)` 仍然可用。

`agui install` 在写 `packages/<name>.json` 的同时维护当前目录的 `agui.lock`（包名、版本、来源、入口、SHA-256）。`agui verify` 校验内容是否被修改；`agui remove` 同时删除 lock 项。

### 16.32 Windows 深色主题与通知

```agui
win = window("暗色", 560, 360, theme="dark")
notify("Agui", "构建完成")
```

`theme="dark"` 使用深色窗口背景并为未显式设置颜色的控件提供浅色前景。`notify` 通过托盘图标显示气泡；若 Explorer/进程权限拒绝，会返回错误，可用 `tray.last_error` 诊断。

### 16.33 Windows 绿色包、安装脚本与自动更新

```powershell
.\agui.exe dist examples\hello.ag -o dist
.\agui.exe update check dist\Agui-应用\update.json
.\agui.exe update apply dist\Agui-应用\update.json --current "C:\Apps\Agui-应用\Agui-应用.exe"
```

`dist` 生成：

- `<名称>-<版本>-win64.zip`；
- `<名称>/<名称>.exe`；
- `install.ps1`：复制到 `%LOCALAPPDATA%\AguiApps\<名称>` 并创建桌面快捷方式；
- `uninstall.ps1`：删除安装目录与快捷方式；
- `update.json`：`name/version/url/sha256/notes`。

`update check` 只读取并显示清单；`update apply` 校验 SHA-256，下载 `url`。若 URL 以 `.zip` 结尾，会从 zip 中取最大的 `.exe`，写到 `<当前exe>.new`，再由一个短暂 PowerShell 脚本在 600ms 后替换并重启。因此更新时应用会退出一次；替换脚本依赖 PowerShell。

`dist --url <地址>` 可把下载地址写入清单；也可以手工编辑 `update.json` 指向 GitHub Releases 等 HTTPS 地址。

### 16.34 Android App Bundle（AAB）

```powershell
.\agui.exe build examples\android_star.ag --aab --version-code 2 -o build\android\star.aab
```

`--aab` 复用 APK 构建流程，然后用 `aapt2 link --proto-format` 生成 proto 资源，并重新打包为：

- `BundleConfig.pb`
- `base/manifest/AndroidManifest.xml`
- `base/resources.pb`
- `base/res/...`
- `base/assets/app.ag`
- `base/dex/classes.dex`
- `base/lib/arm64-v8a/libgojni.so`

AAB 不签名；上传 Google Play 时由 Play App Signing 签名。需要本地校验时可安装 bundletool：

```powershell
java -jar bundletool.jar validate --bundle build\android\star.aab
```

当前构建器输出的是最小 bundle 结构，未包含密度/语言拆分；未用 bundletool 最终校验前，建议同时保留 APK 作为可直接安装的产物。
