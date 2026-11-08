# Agui 宣传方案

> 目标：让「Windows 上想做个带界面的小工具」的人，在 3 分钟内从看到帖子到跑起来。
> 现状（2026-11-07）：GitHub 0 star / 0 fork，无 Release、无在线试玩、无英文说明。**问题不在语言，在漏斗。**

---

## 一、定位：别卖「语言」，卖「最短路径」

新语言最贵的成本是「让人相信它不会半路死掉」。所以宣传语不要从语言设计讲起，从**用户手里那件具体的活儿**讲起。

三个版本，按场合用：

| 场合 | 一句话 |
| --- | --- |
| 主定位 | **写 Windows 小工具，最短的一条路：10 行脚本，双击 exe。** |
| 对 Python 用户 | 你的脚本缺个界面？tkinter 丑、PyQt 打包 60MB、Electron 更重——Agui 打包出来 12MB，原生控件。 |
| 对非程序员 | 不用装环境、不用编译，改一行字就能改界面，做出来的就是一个 exe，发给谁都能用。 |

**核心记忆点（只讲一个）**：`10 行脚本 → 12MB 单文件 exe → 双击就跑，零依赖。`
其他特性（Game Kit、中文关键字、Android）是第二层信息，第一次接触不讲。

### 竞品坐标（必须提前想好，评论区一定会问）

| | Agui | aardio | 易语言 | Python+PyQt | Electron |
| --- | --- | --- | --- | --- | --- |
| 学习曲线 | 类 Python 缩进 | 中 | 低（中文） | 低 | 中 |
| 打包体积 | **~12 MB** | ~2–5 MB | ~5–10 MB | 40–80 MB | 120–200 MB |
| 原生控件 | ✅ Win32 | ✅ | ✅ | ✅（Qt 非原生但好看） | ❌ 网页 |
| 开源 | ✅ MIT | 免费闭源 | 商业 | ✅ | ✅ |
| 生态/库 | ❌ 弱 | ✅ 强 | ✅ 中 | ✅ 极强 | ✅ 极强 |
| 跨平台 | ❌（WASM 实验） | ❌ | ❌ | ✅ | ✅ |

结论：**不要跟 aardio 比功能，不要跟 Python 比生态。** 只打三个点——语法现代（像 Python）、真开源 MIT、打包链路简单到一条命令。任何「Agui 不如 X」的评论都回答：「对，所以它只做这一件事。」

---

## 二、先修漏斗（0 转化，发再多帖也白费）

按投入产出排序，P0 全部可在一天内完成：

| 优先级 | 现状 | 要改成 | 状态（2026-11-08） |
| --- | --- | --- | --- |
| P0 | 想跑必须先装 Go 1.22+ 再 `build.bat` | **GitHub Releases 放 `agui.exe` + 示例 exe**，README 顶部一个大按钮「下载即用」 | ✅ v0.9.1 发布 3 个资产；README 首屏已加「下载即用」（check/run/build 三条命令），「快速开始」下沉为「从源码构建」 |
| P0 | 仓库描述是「…制作于 deepseek flash 4.1」 | 中英双语，点明「12 MB 单文件 exe / 零运行时」，**不要提 AI 生成** | ✅ 已改（中文 + English），topics 10 个已加 |
| P0 | 无在线试玩 | `web/`（WASM）走 GitHub Pages | ✅ <https://gumeng23.github.io/AGUI-Windows/> 已上线，workflow 见 `.github/workflows/pages.yml`；2026-11-08 首页改为项目官网（特性/截图/对比/上手），游戏移到 `/demo.html` |
| P0 | 无「值得点开」的官网 | Pages 只放游戏加载页 | ✅ 已补：hero + 三十秒上手 + 9 条特性 + 8 张截图 + 四方案对比 + Android 限制说明 |
| P0 | 无主页链接 | 仓库右侧 Homepage 填 Pages 地址 | ⚠️ **仍未填**（Pages 已通，填上等于多一个入口） |
| P1 | `aguicode.zip`（66MB）提交在仓库里 | 从 HEAD 移除（历史可保留） | ✅ 已移除 |
| P1 | README 很长但首屏没有「30 秒看懂」 | 首屏：一句话 + GIF + 下载/试玩 + 最小示例 | 🟡 一句话、下载、试玩、示例都齐了，**GIF 仍缺**（静态图讲不出「脚本→窗口」的落差） |
| P1 | 只有中文 README | 加 `README.en.md`，首屏同结构 | ⬜ 未做 |
| P2 | 无 issue 模板、无 CI 徽章 | 加 `go test` CI 徽章、issue 模板、`CONTRIBUTING.md` | ⬜ 未做 |
| P2 | VS Code 扩展未上 Marketplace | 发布到 Marketplace 并写进 README | ⬜ 未做（本地已有 `agui-0.1.0.vsix`） |

**关键判断**：一个陌生访客的耐心约 30 秒。当前路径是「clone → 装 Go → 编译 → 找示例」，流失率 >95%。修完 P0，同样一篇帖子带来的留存能翻几倍。

---

## 三、三个传播资产（比十篇帖子有用）

### 1. 30 秒 GIF（最重要）

一屏到底，无剪辑字幕，节奏：

```
0–3s   空白编辑器输入 12 行（提速 2×，能看清结构）
3–6s   按 F5，窗口弹出，点按钮，数字跳动
6–8s   终端敲 agui build app.ag -o app.exe
8–10s  文件管理器里出现 app.exe，11.4 MB
10–14s 双击 app.exe → 窗口秒开（对比：旁边 Electron 应用还在转圈）
14–18s 把 exe 拖进微信发给「另一个窗口」，对方双击直接跑
```

最后 4 秒是全片的价值主张：**发给别人就能用**。GIF 控制在 5MB 内，首帧就是成品窗口，别放 logo。

### 2. 一张对比图（数据要真实）

需要实测后填：打包体积、冷启动时间、内存占用。

- 体积：Agui `out/hello.exe` = **11,409,888 字节（10.9 MB）**（已实测）
- 待测：Agui 冷启动（目标 <300ms）、同功能 PyQt5 打包版体积/启动、Electron 版体积/启动
- 图的形式：横向条形图，Agui 一条短到几乎看不见，标题「同样的窗口，体积差 10 倍」

### 3. 三个「能用的成品」（不是 demo）

语言项目最有效的传播物是「别人真的会用的工具」。每个都要能一键下载 exe：

1. **批量重命名 / 文件整理器** —— 打工人刚需，截图传播力强
2. **Excel/CSV 小工具**（去重、合并、转 JSON）—— 办公场景，受众最广
3. **串口/HTTP 调试小工具** —— 程序员场景，证明「不是玩具」

每个配一句「源码 60 行，见 examples/xxx.ag」。**能被改造，才会有人留下来。**

---

## 四、渠道与节奏

不要一次全发。先小范围验证文案，再放大。

### 第一周：中文技术社区（验证）

| 顺序 | 渠道 | 写法要点 |
| --- | --- | --- |
| 1 | **V2EX / 分享创造** | 标题带结果：`写了个 Windows 小工具语言，10 行代码打包成 12MB 的 exe`。正文自嘲 + 求骂，V2EX 吃这套；**必须贴在线试玩和 exe 下载** |
| 2 | **掘金 / 知乎** | 文章体：《我为什么又造了一门语言：用 Go + Win32 做 GUI 脚本》。讲取舍（为什么不用 Electron、为什么缩进、为什么中文关键字），开发者爱看「决策过程」 |
| 3 | **少数派** | 面向普通用户：《不装任何环境，做一个自己的 Windows 小工具》。配 GIF，讲成品，不讲实现 |
| 4 | **B 站** | 3–5 分钟：从空白到 exe，全程录屏无剪辑。标题 `10 行代码做个 Windows 软件` |

### 第二周：英文圈（放大）

| 渠道 | 写法要点 |
| --- | --- |
| **Show HN** | 标题 `Show HN: A scripting language that compiles to a 12 MB native Windows exe`。首发要有 playground 链接（HN 用户不下载东西）。准备好回答「为什么不用 C#/WinForms」「为什么不用 Tauri」 |
| **Reddit r/golang** | 角度换成实现：`I wrote a tree-walking interpreter + Win32 GUI bindings in pure Go, no cgo`。Go 社区对「无 cgo 的 Win32 绑定」真的感兴趣 |
| **Reddit r/programming** | 慎重，氛围偏苛刻。等有 100+ star 再发 |
| **Lobsters** | 小而精，适合「语言设计取舍」类文章 |

### 第三周起：长尾

- 在「Python 打包太大怎么办」「tkinter 太丑」类问题下做**诚实回答**（先答问题，末尾提一句自己的项目，别硬广）
- 给 `awesome-windows` / `awesome-programming-languages` 提 PR
- 尝试 winget / scoop 收录（`winget install agui` 是极强的「正经项目」信号）
- 每月一篇「用 Agui 做了 X」的短文，形成连载

---

## 五、可直接用的文案

### V2EX（分享创造）

> **写了个 Windows 小工具语言，10 行代码打包成 12MB 的 exe**
>
> 起因很俗：我想给几个小脚本加界面。tkinter 太丑，PyQt 打包 60MB，Electron 更别说了。于是自己写了一门解释型语言，Go + Win32，语法长这样：
>
> ```
> win = window("你好", w=560, h=320)
> label("欢迎")
> name = edit("世界", w=260)
> greet = button("打招呼", w=150)
> on greet.click:
>     msgbox("你好，" + name.text + "！")
> ```
>
> `agui build hello.ag -o hello.exe` 之后得到一个 12MB 的单文件，双击就跑，不需要任何运行时。控件都是原生 Win32，跟随系统主题。
>
> 在线试玩：<链接> ｜ 下载：<Release 链接> ｜ 源码：<GitHub>
>
> 目前 0.7，Windows only，生态基本为零，单人维护。欢迎骂，尤其想知道「你会在什么场景下真的用它」——这决定我下一步做什么。

### 掘金 / 知乎（长文骨架）

1. 引子：一个真实的小需求 + 三张截图（tkinter 版 / PyQt 版体积 / Agui 版体积）
2. 为什么不用现成方案：逐条列，**每条都给数据和场景，不给情绪**
3. 设计取舍：缩进 vs 花括号；动态类型；`on btn.click:` 为什么做成语法；中文关键字为什么是别名而不是主语法
4. 实现：Go 单文件、无 cgo 调 Win32、解释器 3 个阶段、打包 = 追加到 exe 末尾（这个技巧值得单独一节，技术读者会转发）
5. 踩坑：SmartScreen、GDI+ 图片路径、协作式 async
6. 现状与求助：明确列出「最想要的三个贡献」

### Show HN

> **Show HN: A scripting language that builds native Windows GUI apps into one 12 MB exe**
>
> Agui is a tree-walking interpreter written in pure Go (no cgo) that binds Win32 directly. You write indented, Python-like scripts; `agui build app.ag` appends your script and assets to the interpreter binary, producing a single self-contained .exe with no runtime dependency.
>
> Try it in the browser (WASM): <link> — no install.
>
> Why: I wanted a PowerShell-script-to-GUI path that doesn't drag in Electron or ship a 60 MB PyInstaller bundle. Widgets are real Win32 controls, so they follow the system theme and start instantly.
>
> Status: v0.7, Windows-only, single maintainer, MIT. Known gaps: no package ecosystem, no debugger, limited layout engine. Happy to answer anything about the Win32 bindings or the bundling trick.

### 评论区标准应答（提前写好，别现场想）

| 质疑 | 回应 |
| --- | --- |
| 「为什么不用 C#/WinForms？」 | 不用编译、不用 SDK，改一行字就能改界面，而且产物是一个 12MB 的 exe 而不是一堆依赖。目标用户是「不想开 Visual Studio 的人」。 |
| 「AI 写的吧？」 | 是，我主导设计并用 AI 加速实现；测试、示例、API 设计都是人工迭代的。→ 然后立刻把话题拉回「你用不用得上」，不辩解。 |
| 「生态太弱，不如 Python」 | 对，它不打算替代 Python。它只解决「脚本要个界面」这一件事。 |
| 「Windows only 是死路」 | 现阶段是取舍。WASM 版已经能跑，跨平台等有人真的需要再说。 |
| 「中文关键字是噱头」 | 是别名，不是主语法——英文关键字一样能用。给不熟悉英文的人降低门槛而已。 |

---

## 六、30 天里程碑

| 周 | 目标 | 验收 |
| --- | --- | --- |
| W1 | 漏斗修完 | Release 有 exe、Pages 有 playground、英文 README、topics、GIF 上线 |
| W2 | 中文首发 | V2EX + 掘金 + 知乎三连，目标 100 star、20 条有效反馈 |
| W3 | 英文首发 | Show HN + r/golang，目标 300 star |
| W4 | 沉淀 | 把反馈变成 roadmap issue；发第二个成品工具；winget PR |

**唯一要盯的指标**：从帖子点击到「下载/试玩」的转化率，以及**回访**。star 是虚荣指标，能跑起来的人才算。

---

## 七、一句话总结

> 别宣传「一门新语言」，宣传「一个 12MB、双击就能用的小工具」；
> 先把「点进来 → 30 秒内跑起来」这条路修通，再谈传播。
