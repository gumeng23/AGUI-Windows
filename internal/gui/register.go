package gui

import (
	"fmt"
	"strings"

	"agui/internal/lang"
)

var commonWidgetKw = []string{
	"x", "y", "w", "h", "width", "height",
	"fg", "bg", "font", "enabled", "visible", "parent", "anchor", "tooltip",
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
		if err := checkKw(kwargs, "title", "w", "width", "h", "height", "bg", "resizable", "icon", "parent",
			"center", "topmost", "opacity", "state", "theme"); err != nil {
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
		theme := strings.ToLower(kwStr(kwargs, "theme", ""))
		bg := kwStr(kwargs, "bg", "#F7F8FA")
		if theme == "dark" {
			if _, explicit := kwargs["bg"]; !explicit {
				bg = "#0F172A"
			}
			app.darkTheme = true
		}
		resizable := kwBool(kwargs, "resizable", true)
		w, err := app.createWindow(title, int(width), int(height), bg, resizable, parent)
		if err != nil {
			return nil, err
		}
		if theme == "dark" {
			_ = w.SetProp("fg", "#E2E8F0")
		}
		if icon := kwStr(kwargs, "icon", ""); icon != "" {
			if err := w.setWindowIcon(icon); err != nil {
				return nil, err
			}
		}
		if kwBool(kwargs, "center", false) {
			w.centerWindow()
		}
		if kwBool(kwargs, "topmost", false) {
			w.setTopmost(true)
		}
		if v, ok := kwargs["opacity"]; ok {
			f, err := numArg(v, "opacity")
			if err != nil {
				return nil, err
			}
			if err := w.setOpacity(f); err != nil {
				return nil, err
			}
		}
		if state := kwStr(kwargs, "state", ""); state != "" {
			if err := w.setWindowState(state); err != nil {
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

	reg("tray", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("tray", args, 0); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, "icon", "tooltip", "text"); err != nil {
			return nil, err
		}
		icon := kwStr(kwargs, "icon", "")
		tooltip := kwStr(kwargs, "tooltip", kwStr(kwargs, "text", "Agui"))
		return app.newTray(icon, tooltip)
	})

	reg("notify", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("notify", args, 2); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, "warning"); err != nil {
			return nil, err
		}
		title := lang.ToString(lang.Named(args, kwargs, 0, "title", "Agui"))
		text := lang.ToString(lang.Named(args, kwargs, 1, "text", ""))
		if app.tray == nil {
			t, err := app.newTray("", "Agui")
			if err != nil {
				return nil, err
			}
			app.tray = t
		}
		if err := app.tray.notify(title, text, kwBool(kwargs, "warning", false)); err != nil {
			return nil, err
		}
		return nil, nil
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

	regContainer := func(name, kind string, extra ...string) {
		reg(name, func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
			if err := lang.CheckArity(name, args, 0); err != nil {
				return nil, err
			}
			if err := checkKw(kwargs, widgetKw(extra...)...); err != nil {
				return nil, err
			}
			cols := 2
			if v, ok := kwargs["cols"]; ok {
				n, err := numArg(v, "cols")
				if err != nil {
					return nil, err
				}
				cols = int(n)
			}
			x, y, w, h, err := app.geometry(kwargs)
			if err != nil {
				return nil, err
			}
			widget, err := app.createPanel(kind, cols, x, y, w, h)
			if err != nil {
				return nil, err
			}
			return widget, app.applyCommon(widget, kwargs)
		})
	}
	regContainer("panel", "panel")
	regContainer("scroll", "scroll")
	regContainer("grid", "grid", "cols")

	reg("toolbar", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("toolbar", args, 0); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw()...); err != nil {
			return nil, err
		}
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createToolbar(x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("splitter", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("splitter", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("vertical")...); err != nil {
			return nil, err
		}
		vertical := kwBool(kwargs, "vertical", false)
		if len(args) > 0 {
			vertical = lang.Truthy(args[0])
		}
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createSplitter(vertical, x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("tabs", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("tabs", args, 0); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw()...); err != nil {
			return nil, err
		}
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createTabs(x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("propertygrid", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("propertygrid", args, 0); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("rows")...); err != nil {
			return nil, err
		}
		rows, err := toRows(lang.Named(args, kwargs, 0, "rows", nil))
		if err != nil {
			return nil, err
		}
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createPropertyGrid(rows, x, y, w, h)
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

	reg("radio", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("radio", args, 1); err != nil {
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
		widget, err := app.createRadio(text, checked, x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("statusbar", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("statusbar", args, 1); err != nil {
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
		widget, err := app.createStatusbar(text, x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("date", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("date", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("value", "text")...); err != nil {
			return nil, err
		}
		value := lang.ToString(lang.Named(args, kwargs, 0, "value", ""))
		if value == "" {
			value = kwStr(kwargs, "text", "")
		}
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createDate("date", value, x, y, w, h)
		if err != nil {
			return nil, err
		}
		return widget, app.applyCommon(widget, kwargs)
	})

	reg("time", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("time", args, 1); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, widgetKw("value", "text")...); err != nil {
			return nil, err
		}
		value := lang.ToString(lang.Named(args, kwargs, 0, "value", ""))
		if value == "" {
			value = kwStr(kwargs, "text", "")
		}
		x, y, w, h, err := app.geometry(kwargs)
		if err != nil {
			return nil, err
		}
		widget, err := app.createDate("time", value, x, y, w, h)
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
		if err := checkKw(kwargs, widgetKw("headers", "rows", "editable", "virtual")...); err != nil {
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
		virtual := kwBool(kwargs, "virtual", false)
		widget, err := app.createTable(headers, rows, x, y, w, h, virtual)
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
		task := lang.NewTask()
		app.addTask(task)
		go func() {
			var result lang.Value
			err := i.ProtectCooperative(task, func() { result = i.CallValue(work) })
			task.Finish(result, err)
			if err != nil {
				if !task.IsCancelled() {
					app.Post(func() { app.reportError(err) })
				}
				return
			}
			if done != nil && !task.IsCancelled() {
				app.Post(func() {
					if e := i.Protect(func() { i.CallValue(done) }); e != nil {
						app.reportError(e)
					}
				})
			}
		}()
		return task, nil
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
		err := app.Run()
		app.shutdownTasks()
		if err != nil {
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

	reg("play_wav", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if err := lang.CheckArity("play_wav", args, 2); err != nil {
			return nil, err
		}
		if err := checkKw(kwargs, "loop"); err != nil {
			return nil, err
		}
		path := lang.ToString(lang.Pos(args, 0, ""))
		if path == "" {
			return nil, fmt.Errorf("play_wav(路径, loop=false) 需要路径")
		}
		real, err := app.interp.AssetPath(path)
		if err != nil {
			return nil, err
		}
		loop := kwBool(kwargs, "loop", false)
		if len(args) == 2 {
			loop = lang.Truthy(args[1])
		}
		if err := playWAV(real, loop); err != nil {
			return nil, err
		}
		return nil, nil
	})
	reg("stop_wav", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if len(args) != 0 || len(kwargs) != 0 {
			return nil, fmt.Errorf("stop_wav(): 不接受参数")
		}
		stopWAV()
		return nil, nil
	})

	reg("clipboard_image", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if len(args) != 0 || len(kwargs) != 0 {
			return nil, fmt.Errorf("clipboard_image(): 不接受参数")
		}
		data, err := clipboardImagePNG()
		if err != nil {
			return nil, err
		}
		return &lang.Bytes{Data: data}, nil
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

	registerGame(i, app)
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
		if !ok || !isContainerKind(p.kind) {
			return nil, nil, nil, nil, fmt.Errorf("parent 必须是 groupbox、panel、scroll 或 grid 控件")
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

func darkBackgroundKind(kind string) bool {
	switch kind {
	case "button", "edit", "checkbox", "radio", "listbox", "combo", "tree",
		"table", "richedit", "tabs", "toolbar", "splitter", "tabpage", "panel":
		return true
	}
	return false
}

func (a *App) applyCommon(w *Widget, kwargs map[string]lang.Value) error {
	if a.darkTheme {
		if _, ok := kwargs["fg"]; !ok && w.kind != "window" {
			if err := w.SetProp("fg", "#E2E8F0"); err != nil {
				return err
			}
		}
		if _, ok := kwargs["bg"]; !ok && darkBackgroundKind(w.kind) {
			if err := w.SetProp("bg", "#1E293B"); err != nil {
				return err
			}
		}
	}
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
	if v, ok := kwargs["anchor"]; ok {
		if err := w.SetProp("anchor", v); err != nil {
			return err
		}
	}
	if v, ok := kwargs["tooltip"]; ok {
		if err := w.SetProp("tooltip", v); err != nil {
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
