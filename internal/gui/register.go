package gui

import (
	"fmt"

	"agui/internal/lang"
)

var commonWidgetKw = []string{
	"x", "y", "w", "h", "width", "height",
	"fg", "bg", "font", "enabled", "visible", "parent",
}

// Register wires the GUI built-ins into an interpreter and returns the app that
// owns all created windows. Call it once per program.
func Register(i *lang.Interp) *App {
	app := NewApp(i)
	reg := func(name string, fn func(*lang.Interp, []lang.Value, map[string]lang.Value) (lang.Value, error)) {
		i.RegisterNative(name, fn)
	}

	reg("window", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("window", args, 3); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, "title", "w", "width", "h", "height", "bg", "resizable", "icon", "parent"); err != nil {
			return nil, err
		}
		var parent *Widget
		if v, ok := kwargs["parent"]; ok {
			pw, ok := v.(*Widget)
			if !ok || pw.kind != "window" {
				return nil, fmt.Errorf("window 的 parent 必须是另一个窗口")
			}
			parent = pw
		}
		title := lang.ToString(lang.Named(args, kwargs, 0, "title", "Agui 应用"))
		width, ok, err := pickNum(args, kwargs, 1, "w", "width")
		if err != nil {
			return nil, err
		}
		if !ok {
			width = 900
		}
		height, ok, err := pickNum(args, kwargs, 2, "h", "height")
		if err != nil {
			return nil, err
		}
		if !ok {
			height = 640
		}
		bg := kwStr(kwargs, "bg", "#F7F8FA")
		resizable := kwBool(kwargs, "resizable", true)
		w, err := app.createWindow(title, int(width), int(height), bg, resizable, parent)
		if err != nil {
			return nil, err
		}
		if icon := kwStr(kwargs, "icon", ""); icon != "" {
			if err := w.setWindowIcon(icon); err != nil {
				return nil, err
			}
		}
		return w, nil
	})

	reg("menu", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("menu", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, "title"); err != nil {
			return nil, err
		}
		title := lang.ToString(lang.Named(args, kwargs, 0, "title", "菜单"))
		m, err := app.newMenu(title)
		if err != nil {
			return nil, err
		}
		return m, nil
	})

	reg("label", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("label", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("text")...); err != nil {
			return nil, err
		}
		text := lang.ToString(lang.Named(args, kwargs, 0, "text", ""))
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createLabel(text, x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("button", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("button", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("text")...); err != nil {
			return nil, err
		}
		text := lang.ToString(lang.Named(args, kwargs, 0, "text", "按钮"))
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createButton(text, x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("groupbox", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("groupbox", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("text")...); err != nil {
			return nil, err
		}
		text := lang.ToString(lang.Named(args, kwargs, 0, "text", "分组"))
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createGroupBox(text, x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("checkbox", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("checkbox", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("text", "checked")...); err != nil {
			return nil, err
		}
		text := lang.ToString(lang.Named(args, kwargs, 0, "text", "选项"))
		checked := kwBool(kwargs, "checked", false)
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createCheckbox(text, checked, x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("edit", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("edit", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("text", "multiline", "readonly", "password")...); err != nil {
			return nil, err
		}
		text := lang.ToString(lang.Named(args, kwargs, 0, "text", ""))
		multiline := kwBool(kwargs, "multiline", false)
		readonly := kwBool(kwargs, "readonly", false)
		password := kwBool(kwargs, "password", false)
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createEdit(text, multiline, readonly, password, x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("richedit", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("richedit", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("text", "readonly")...); err != nil {
			return nil, err
		}
		text := lang.ToString(lang.Named(args, kwargs, 0, "text", ""))
		readonly := kwBool(kwargs, "readonly", false)
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createRichEdit(text, readonly, x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("listbox", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("listbox", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("items")...); err != nil {
			return nil, err
		}
		items, err := toItems(lang.Named(args, kwargs, 0, "items", nil))
		if err != nil {
			return nil, err
		}
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createListbox(items, x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("combo", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("combo", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("items")...); err != nil {
			return nil, err
		}
		items, err := toItems(lang.Named(args, kwargs, 0, "items", nil))
		if err != nil {
			return nil, err
		}
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createCombo(items, x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("progress", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("progress", args, 2); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("value", "max", "min")...); err != nil {
			return nil, err
		}
		value, ok, err := pickNum(args, kwargs, 0, "value")
		if err != nil {
			return nil, err
		}
		if !ok {
			value = 0
		}
		maxVal, ok, err := pickNum(args, kwargs, 1, "max")
		if err != nil {
			return nil, err
		}
		if !ok {
			maxVal = 100
		}
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createProgress(value, maxVal, x, y, w, h)
		if err != nil {
			return nil, err
		}
		if minVal, ok, err := pickNum(nil, kwargs, -1, "min"); err != nil {
			return nil, err
		} else if ok {
			if err := widget.SetProp("min", minVal); err != nil {
				return nil, err
			}
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("slider", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("slider", args, 3); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("value", "min", "max")...); err != nil {
			return nil, err
		}
		value, ok, err := pickNum(args, kwargs, 0, "value")
		if err != nil {
			return nil, err
		}
		if !ok {
			value = 0
		}
		minVal, ok, err := pickNum(args, kwargs, 1, "min")
		if err != nil {
			return nil, err
		}
		if !ok {
			minVal = 0
		}
		maxVal, ok, err := pickNum(args, kwargs, 2, "max")
		if err != nil {
			return nil, err
		}
		if !ok {
			maxVal = 100
		}
		if maxVal <= minVal {
			return nil, fmt.Errorf("slider(): max 必须大于 min")
		}
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createSlider(minVal, maxVal, value, x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("image", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("image", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("src", "path", "fit")...); err != nil {
			return nil, err
		}
		path := lang.ToString(lang.Named(args, kwargs, 0, "src", ""))
		if v, ok := kwargs["path"]; ok {
			path = lang.ToString(v)
		}
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createImage(path, x, y, w, h)
		if err != nil {
			return nil, err
		}
		widget.imgFit = kwBool(kwargs, "fit", true)
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("canvas", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("canvas", args, 0); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("bg")...); err != nil {
			return nil, err
		}
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createCanvas(x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("table", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("table", args, 2); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("headers", "rows", "editable")...); err != nil {
			return nil, err
		}
		headers, err := toItems(lang.Named(args, kwargs, 0, "headers", nil))
		if err != nil {
			return nil, err
		}
		rows, err := toRows(lang.Named(args, kwargs, 1, "rows", nil))
		if err != nil {
			return nil, err
		}
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createTable(headers, rows, x, y, w, h)
		if err != nil {
			return nil, err
		}
		if v, ok := kwargs["editable"]; ok {
			widget.editable = lang.Truthy(v)
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("tree", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("tree", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("items")...); err != nil {
			return nil, err
		}
		items := lang.Named(args, kwargs, 0, "items", nil)
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createTree(items, x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("font", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("font", args, 2); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, "family", "size", "bold", "italic"); err != nil {
			return nil, err
		}
		family := lang.ToString(lang.Named(args, kwargs, 0, "family", "Microsoft YaHei UI"))
		size, ok, err := pickNum(args, kwargs, 1, "size")
		if err != nil {
			return nil, err
		}
		if !ok {
			size = 10
		}
		if size <= 0 {
			return nil, fmt.Errorf("font(): 字号必须大于 0")
		}
		bold := kwBool(kwargs, "bold", false)
		italic := kwBool(kwargs, "italic", false)
		h := createFont(family, size, bold, italic, app.dpi)
		if h == 0 {
			return nil, fmt.Errorf("font(): 创建字体失败")
		}
		app.fonts = append(app.fonts, h)
		return &lang.Font{Family: family, Size: size, Bold: bold, Italic: italic, Handle: h}, nil
	})

	reg("msgbox", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("msgbox", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, "title", "kind"); err != nil {
			return nil, err
		}
		text := lang.ToString(lang.Pos(args, 0, ""))
		title := kwStr(kwargs, "title", "提示")
		kind := kwStr(kwargs, "kind", "info")
		flags := uint32(mbOK)
		switch kind {
		case "info", "information":
			flags |= mbIconInformation
		case "warn", "warning":
			flags |= mbIconWarning
		case "error":
			flags |= mbIconError
		case "question":
			flags |= mbIconQuestion
		default:
			return nil, fmt.Errorf("msgbox(): 未知类型 %q（可用 info/warn/error/question）", kind)
		}
		messageBox(app.hwnd, text, title, flags)
		return nil, nil
	})

	reg("confirm", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("confirm", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, "title"); err != nil {
			return nil, err
		}
		text := lang.ToString(lang.Pos(args, 0, ""))
		title := kwStr(kwargs, "title", "确认")
		r := messageBox(app.hwnd, text, title, mbYesNo|mbIconQuestion)
		return r == idYes, nil
	})

	reg("input", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("input", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, "title", "default"); err != nil {
			return nil, err
		}
		prompt := lang.ToString(lang.Pos(args, 0, "请输入："))
		title := kwStr(kwargs, "title", "输入")
		initial := kwStr(kwargs, "default", "")
		result, ok := app.showInput(title, prompt, initial)
		if !ok {
			return nil, nil
		}
		return result, nil
	})

	reg("open_file", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("open_file", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, "title"); err != nil {
			return nil, err
		}
		filter := lang.ToString(lang.Pos(args, 0, "所有文件 (*.*)|*.*"))
		title := kwStr(kwargs, "title", "打开文件")
		path, ok, err := app.openFile(filter, title)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		return path, nil
	})

	reg("save_file", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("save_file", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, "title", "default"); err != nil {
			return nil, err
		}
		filter := lang.ToString(lang.Pos(args, 0, "所有文件 (*.*)|*.*"))
		title := kwStr(kwargs, "title", "保存文件")
		def := kwStr(kwargs, "default", "")
		path, ok, err := app.saveFile(filter, title, def)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		return path, nil
	})

	reg("after", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		return registerTimer(i, app, args, kwargs, false)
	})

	reg("every", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		return registerTimer(i, app, args, kwargs, true)
	})

	reg("async", func(i *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("async", args, 2); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, "then"); err != nil {
			return nil, err
		}
		work := lang.Pos(args, 0, nil)
		if work == nil {
			return nil, fmt.Errorf("async(函数, 完成后回调?) 需要至少一个函数参数")
		}
		if !callable(work) {
			return nil, fmt.Errorf("async(): 第一个参数必须是函数")
		}
		done := lang.Pos(args, 1, nil)
		if v, ok := kwargs["then"]; ok {
			done = v
		}
		if done != nil && !callable(done) {
			return nil, fmt.Errorf("async(): then 必须是函数")
		}
		go func() {
			err := i.Protect(func() { i.CallValue(work) })
			app.Post(func() {
				if err != nil {
					app.reportError(err)
					return
				}
				if done != nil {
					if e := i.Protect(func() { i.CallValue(done) }); e != nil {
						app.reportError(e)
					}
				}
			})
		}()
		return nil, nil
	})

	reg("cancel_timer", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("cancel_timer", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs); err != nil {
			return nil, err
		}
		id, err := numArg(lang.Pos(args, 0, nil), "定时器 id")
		if err != nil {
			return nil, err
		}
		app.cancelTimer(uintptr(int(id)))
		return nil, nil
	})

	reg("run", func(i *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if len(args) > 0 || len(kwargs) > 0 {
			return nil, fmt.Errorf("run(): 不接受参数")
		}
		if !app.HasWindow() {
			return nil, fmt.Errorf("run(): 还没有创建窗口，请先用 window() 或控件函数")
		}
		if app.Ran() {
			return nil, fmt.Errorf("run(): 消息循环已经结束，不能重复运行")
		}
		depth := i.Suspend()
		defer i.Resume(depth)
		if err := app.Run(); err != nil {
			return nil, err
		}
		return nil, nil
	})

	reg("quit", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if len(args) > 0 || len(kwargs) > 0 {
			return nil, fmt.Errorf("quit(): 不接受参数")
		}
		app.Quit()
		return nil, nil
	})

	reg("screenshot", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("screenshot", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, "path"); err != nil {
			return nil, err
		}
		path := lang.ToString(lang.Named(args, kwargs, 0, "path", "screenshot.png"))
		if err := app.Screenshot(path); err != nil {
			return nil, err
		}
		return nil, nil
	})

	reg("set_clipboard", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("set_clipboard", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs); err != nil {
			return nil, err
		}
		if err := clipboardSet(lang.ToString(lang.Pos(args, 0, ""))); err != nil {
			return nil, err
		}
		return nil, nil
	})

	reg("get_clipboard", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if len(args) > 0 || len(kwargs) > 0 {
			return nil, fmt.Errorf("get_clipboard(): 不接受参数")
		}
		text, err := clipboardGet()
		if err != nil {
			return nil, err
		}
		return text, nil
	})

	reg("screen_width", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		sw, _ := screenSize()
		return float64(sw) / app.scale, nil
	})

	reg("screen_height", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		_, sh := screenSize()
		return float64(sh) / app.scale, nil
	})

	reg("dpi", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		return float64(app.dpi), nil
	})

	return app
}

func registerTimer(i *lang.Interp, app *App, args []lang.Value, kwargs map[string]lang.Value, repeat bool) (lang.Value, error) {
	name := "after"
	if repeat {
		name = "every"
	}
	if err := lang.CheckArity(name, args, 2); err != nil {
		return nil, err
	}
	if err := checkKw(kwargs); err != nil {
		return nil, err
	}
	ms, err := numArg(lang.Pos(args, 0, nil), "毫秒")
	if err != nil {
		return nil, err
	}
	fn := lang.Pos(args, 1, nil)
	if !callable(fn) {
		return nil, fmt.Errorf("%s(): 第二个参数必须是函数", name)
	}
	id := app.addTimer(int(ms), repeat, func() error {
		return i.Protect(func() { i.CallValue(fn) })
	})
	if id == 0 {
		return nil, fmt.Errorf("%s(): 创建定时器失败", name)
	}
	return float64(id), nil
}

func callable(v lang.Value) bool {
	switch v.(type) {
	case *lang.Function, *lang.Native:
		return true
	}
	return false
}

func (a *App) geometry(kwargs map[string]lang.Value) (x, y, w, h *int32, err error) {
	a.pendingParent = nil
	if v, ok := kwargs["parent"]; ok {
		p, ok := v.(*Widget)
		if !ok || p.kind != "groupbox" {
			return nil, nil, nil, nil, fmt.Errorf("parent 必须是 groupbox 控件")
		}
		a.pendingParent = p
	}
	x, err = a.coord(kwargs, "x")
	if err != nil {
		return
	}
	y, err = a.coord(kwargs, "y")
	if err != nil {
		return
	}
	w, err = a.coord(kwargs, "w", "width")
	if err != nil {
		return
	}
	h, err = a.coord(kwargs, "h", "height")
	return
}

func (a *App) coord(kwargs map[string]lang.Value, names ...string) (*int32, error) {
	for _, name := range names {
		v, ok := kwargs[name]
		if !ok {
			continue
		}
		f, err := numArg(v, name)
		if err != nil {
			return nil, err
		}
		p := a.px(f)
		return &p, nil
	}
	return nil, nil
}

func (a *App) applyCommon(w *Widget, kwargs map[string]lang.Value) error {
	for _, name := range []string{"fg", "bg"} {
		if v, ok := kwargs[name]; ok {
			if err := w.SetProp(name, v); err != nil {
				return err
			}
		}
	}
	if v, ok := kwargs["font"]; ok {
		if err := w.SetProp("font", v); err != nil {
			return err
		}
	}
	if v, ok := kwargs["enabled"]; ok {
		if err := w.SetProp("enabled", v); err != nil {
			return err
		}
	}
	if v, ok := kwargs["visible"]; ok {
		if err := w.SetProp("visible", v); err != nil {
			return err
		}
	}
	return nil
}

func toItems(v lang.Value) ([]string, error) {
	if v == nil {
		return nil, nil
	}
	l, ok := v.(*lang.List)
	if !ok {
		return nil, fmt.Errorf("items 必须是列表")
	}
	out := make([]string, 0, len(l.Items))
	for _, item := range l.Items {
		out = append(out, lang.ToString(item))
	}
	return out, nil
}

func toRows(v lang.Value) ([][]string, error) {
	if v == nil {
		return nil, nil
	}
	l, ok := v.(*lang.List)
	if !ok {
		return nil, fmt.Errorf("rows 必须是二维列表")
	}
	out := make([][]string, 0, len(l.Items))
	for _, item := range l.Items {
		row, err := toItems(item)
		if err != nil {
			return nil, fmt.Errorf("rows 的每一行必须是列表")
		}
		out = append(out, row)
	}
	return out, nil
}

func widgetKw(extra ...string) []string {
	out := make([]string, 0, len(commonWidgetKw)+len(extra))
	out = append(out, commonWidgetKw...)
	return append(out, extra...)
}

func checkKw(kwargs map[string]lang.Value, allowed ...string) error {
	for name := range kwargs {
		found := false
		for _, a := range allowed {
			if name == a {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("未知参数 %q", name)
		}
	}
	return nil
}

func numArg(v lang.Value, name string) (float64, error) {
	f, ok := v.(float64)
	if !ok {
		return 0, fmt.Errorf("%s 必须是数字", name)
	}
	return f, nil
}

func pickNum(args []lang.Value, kwargs map[string]lang.Value, pos int, name string, aliases ...string) (float64, bool, error) {
	if v, ok := kwargs[name]; ok {
		f, err := numArg(v, name)
		return f, true, err
	}
	for _, alias := range aliases {
		if v, ok := kwargs[alias]; ok {
			f, err := numArg(v, alias)
			return f, true, err
		}
	}
	if pos >= 0 && pos < len(args) && args[pos] != nil {
		f, err := numArg(args[pos], name)
		return f, true, err
	}
	return 0, false, nil
}

func kwStr(kwargs map[string]lang.Value, name, def string) string {
	if v, ok := kwargs[name]; ok {
		return lang.ToString(v)
	}
	return def
}

func kwBool(kwargs map[string]lang.Value, name string, def bool) bool {
	if v, ok := kwargs[name]; ok {
		return lang.Truthy(v)
	}
	return def
}
