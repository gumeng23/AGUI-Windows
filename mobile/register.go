package mobile

import (
	"encoding/json"
	"fmt"
	"strings"

	"agui/internal/lang"
)

// registerUI installs the portable Android View subset. Desktop-only controls
// are deliberately absent so a script fails at build time instead of silently
// doing nothing on Android.
func registerUI(i *lang.Interp, a *App) {
	reg := func(name string, fn func(*lang.Interp, []lang.Value, map[string]lang.Value) (lang.Value, error)) {
		i.RegisterNative(name, fn)
	}

	reg("window", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("window", args, 0, 1); err != nil {
			return nil, err
		}
		title := "Agui"
		if len(args) > 0 {
			title = lang.ToString(args[0])
		}
		w := a.addWidget(0, "window", title, 16, kwString(kw, "fg", "#111827"))
		applyKw(a, w, kw)
		return w, nil
	})

	reg("actionbar", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("actionbar", args, 0, 1); err != nil {
			return nil, err
		}
		title := "Agui"
		if len(args) > 0 {
			title = lang.ToString(args[0])
		}
		w := a.addWidget(0, "actionbar", title, 19, "#FFFFFF")
		applyKw(a, w, kw)
		return w, nil
	})

	reg("tabs", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("tabs", args, 0, 0); err != nil {
			return nil, err
		}
		w := a.addWidget(parentID(kw), "tabs", "", 16, "")
		applyKw(a, w, kw)
		return w, nil
	})

	reg("pager", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("pager", args, 0, 0); err != nil {
			return nil, err
		}
		w := a.addWidget(parentID(kw), "pager", "", 16, "")
		applyKw(a, w, kw)
		return w, nil
	})

	reg("rich", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("rich", args, 1, 1); err != nil {
			return nil, err
		}
		w := a.addWidget(parentID(kw), "rich", lang.ToString(args[0]), 56, "")
		applyKw(a, w, kw)
		return w, nil
	})

	reg("tree", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("tree", args, 1, 1); err != nil {
			return nil, err
		}
		w := a.addWidget(parentID(kw), "tree", "", 16, "")
		applyKw(a, w, kw)
		walkTree(w, args[0], 0)
		return w, nil
	})

	reg("table", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("table", args, 2, 2); err != nil {
			return nil, err
		}
		w := a.addWidget(parentID(kw), "table", "", 14, "")
		data, err := tableJSON(args[0], args[1])
		if err != nil {
			return nil, err
		}
		a.bridge.SetTable(w.id, data)
		applyKw(a, w, kw)
		return w, nil
	})

	reg("list", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("list", args, 1, 1); err != nil {
			return nil, err
		}
		w := a.addWidget(parentID(kw), "list", "", 40, "")
		items, ok := args[0].(*lang.List)
		if !ok {
			return nil, fmt.Errorf("list 需要列表参数")
		}
		rows := make([]string, 0, items.Len())
		for _, item := range items.Items {
			rows = append(rows, lang.ToString(item))
		}
		data, err := json.Marshal(rows)
		if err != nil {
			return nil, err
		}
		a.bridge.SetList(w.id, string(data))
		applyKw(a, w, kw)
		return w, nil
	})

	reg("confirm", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("confirm", args, 1, 2); err != nil {
			return nil, err
		}
		title := "确认"
		msg := lang.ToString(args[0])
		if len(args) > 1 {
			title = lang.ToString(args[0])
			msg = lang.ToString(args[1])
		}
		a.bridge.Confirm(title, msg)
		return nil, nil
	})

	reg("panel", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("panel", args, 0, 0); err != nil {
			return nil, err
		}
		parent := parentID(kw)
		w := a.addWidget(parent, "panel", "", 16, "")
		applyKw(a, w, kw)
		return w, nil
	})

	reg("label", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("label", args, 0, 1); err != nil {
			return nil, err
		}
		w := a.addWidget(parentID(kw), "label", textArg(args), int(kwNumber(kw, "size", 16)), kwString(kw, "fg", "#111827"))
		applyKw(a, w, kw)
		return w, nil
	})

	reg("button", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("button", args, 0, 1); err != nil {
			return nil, err
		}
		w := a.addWidget(parentID(kw), "button", textArg(args), int(kwNumber(kw, "size", 16)), kwString(kw, "fg", "#FFFFFF"))
		applyKw(a, w, kw)
		return w, nil
	})

	reg("edit", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("edit", args, 0, 1); err != nil {
			return nil, err
		}
		w := a.addWidget(parentID(kw), "edit", textArg(args), int(kwNumber(kw, "size", 16)), "")
		w.value = 0
		a.bridge.SetText(w.id, w.text)
		applyKw(a, w, kw)
		return w, nil
	})

	reg("checkbox", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("checkbox", args, 0, 1); err != nil {
			return nil, err
		}
		w := a.addWidget(parentID(kw), "checkbox", textArg(args), int(kwNumber(kw, "size", 16)), "")
		w.checked = kwBool(kw, "checked", false)
		a.bridge.SetChecked(w.id, w.checked)
		applyKw(a, w, kw)
		return w, nil
	})

	reg("progress", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("progress", args, 0, 1); err != nil {
			return nil, err
		}
		w := a.addWidget(parentID(kw), "progress", "", 16, "")
		w.max = kwNumber(kw, "max", 100)
		w.value = kwNumber(kw, "value", 0)
		if len(args) > 0 {
			w.value = number(args[0])
		}
		a.bridge.SetProgress(w.id, int(w.value), int(w.max))
		applyKw(a, w, kw)
		return w, nil
	})

	reg("image", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("image", args, 0, 1); err != nil {
			return nil, err
		}
		w := a.addWidget(parentID(kw), "image", "", 16, "")
		w.src = textArg(args)
		if w.src == "" {
			w.src = kwString(kw, "src", "")
		}
		a.bridge.SetImage(w.id, w.src)
		applyKw(a, w, kw)
		return w, nil
	})

	reg("canvas", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("canvas", args, 0, 0); err != nil {
			return nil, err
		}
		w := a.addWidget(parentID(kw), "canvas", "", 16, kwString(kw, "bg", "#0f172a"))
		applyKw(a, w, kw)
		w.refreshCanvas()
		return w, nil
	})

	reg("font", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		m := lang.NewMap()
		family := "sans-serif"
		if len(args) > 0 {
			family = lang.ToString(args[0])
		}
		size := kwNumber(kw, "size", 16)
		m.Set("family", family)
		m.Set("size", size)
		m.Set("bold", kwBool(kw, "bold", false))
		m.Set("italic", kwBool(kw, "italic", false))
		return m, nil
	})

	reg("alert", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("alert", args, 1, 2); err != nil {
			return nil, err
		}
		title := "提示"
		msg := lang.ToString(args[0])
		if len(args) > 1 {
			title = lang.ToString(args[0])
			msg = lang.ToString(args[1])
		}
		a.bridge.Alert(title, msg)
		return nil, nil
	})

	reg("toast", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("toast", args, 1, 1); err != nil {
			return nil, err
		}
		a.bridge.Toast(lang.ToString(args[0]))
		return nil, nil
	})

	reg("after", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("after", args, 2, 2); err != nil {
			return nil, err
		}
		id := a.timer(int(number(args[0])), false, args[1])
		return float64(id), nil
	})

	reg("every", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("every", args, 2, 2); err != nil {
			return nil, err
		}
		id := a.timer(int(number(args[0])), true, args[1])
		return float64(id), nil
	})

	reg("cancel_timer", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("cancel_timer", args, 1, 1); err != nil {
			return nil, err
		}
		a.cancelTimer(int(number(args[0])))
		return nil, nil
	})

	reg("quit", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		a.finish()
		return nil, nil
	})

	reg("platform", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		return "android", nil
	})

	reg("dpi", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		return float64(2), nil
	})

	reg("clipboard", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		return a.bridge.GetClipboard(), nil
	})

	reg("set_clipboard", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("set_clipboard", args, 1, 1); err != nil {
			return nil, err
		}
		a.bridge.SetClipboard(lang.ToString(args[0]))
		return nil, nil
	})

	reg("open_url", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("open_url", args, 1, 1); err != nil {
			return nil, err
		}
		a.bridge.OpenURL(lang.ToString(args[0]))
		return nil, nil
	})

	reg("vibrate", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		ms := 200
		if len(args) > 0 {
			ms = int(number(args[0]))
		}
		if ms < 1 {
			ms = 1
		}
		a.bridge.Vibrate(ms)
		return nil, nil
	})

	reg("notify", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("notify", args, 1, 2); err != nil {
			return nil, err
		}
		title := "Agui"
		msg := lang.ToString(args[0])
		if len(args) > 1 {
			title = lang.ToString(args[0])
			msg = lang.ToString(args[1])
		}
		a.bridge.Notify(title, msg)
		return nil, nil
	})

	reg("share", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("share", args, 1, 1); err != nil {
			return nil, err
		}
		a.bridge.Share(lang.ToString(args[0]))
		return nil, nil
	})

	reg("save_file", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("save_file", args, 1, 2); err != nil {
			return nil, err
		}
		name := lang.ToString(args[0])
		content := ""
		if len(args) > 1 {
			content = lang.ToString(args[1])
		}
		a.bridge.SaveFile(name, content)
		return nil, nil
	})

	reg("pick_file", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("pick_file", args, 0, 1); err != nil {
			return nil, err
		}
		mime := "*/*"
		if len(args) > 0 {
			mime = lang.ToString(args[0])
		}
		a.bridge.PickFile(mime)
		return nil, nil
	})

	reg("request_permission", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("request_permission", args, 1, 1); err != nil {
			return nil, err
		}
		name := lang.ToString(args[0])
		if !strings.HasPrefix(name, "android.permission.") {
			return nil, fmt.Errorf("权限名必须以 android.permission. 开头")
		}
		a.bridge.RequestPermission(name)
		return nil, nil
	})

	reg("request_permissions", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("request_permissions 至少需要一个权限名")
		}
		names := make([]string, 0, len(args))
		for _, v := range args {
			name := lang.ToString(v)
			if !strings.HasPrefix(name, "android.permission.") {
				return nil, fmt.Errorf("权限名必须以 android.permission. 开头：%s", name)
			}
			names = append(names, name)
		}
		a.bridge.RequestPermissions(strings.Join(names, ","))
		return nil, nil
	})

	reg("foreground", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("foreground", args, 1, 2); err != nil {
			return nil, err
		}
		title := lang.ToString(args[0])
		text := ""
		if len(args) == 2 {
			text = lang.ToString(args[1])
		}
		a.bridge.Foreground(title, text)
		return nil, nil
	})

	reg("stop_foreground", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("stop_foreground", args, 0, 0); err != nil {
			return nil, err
		}
		a.bridge.StopForeground()
		return nil, nil
	})

	reg("camera", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("camera", args, 0, 0); err != nil {
			return nil, err
		}
		a.bridge.Camera()
		return nil, nil
	})

	reg("location", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("location", args, 0, 0); err != nil {
			return nil, err
		}
		return a.bridge.Location(), nil
	})

	reg("start_record", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("start_record", args, 0, 0); err != nil {
			return nil, err
		}
		a.bridge.StartRecording()
		return nil, nil
	})

	reg("stop_record", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("stop_record", args, 0, 0); err != nil {
			return nil, err
		}
		a.bridge.StopRecording()
		return nil, nil
	})

	reg("sensor", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("sensor", args, 1, 1); err != nil {
			return nil, err
		}
		return a.bridge.Sensor(lang.ToString(args[0])), nil
	})

	reg("device_info", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("device_info", args, 0, 0); err != nil {
			return nil, err
		}
		return a.bridge.DeviceInfo(), nil
	})

	reg("battery", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("battery", args, 0, 0); err != nil {
			return nil, err
		}
		return float64(a.bridge.Battery()), nil
	})

	reg("network", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if err := argCount("network", args, 0, 0); err != nil {
			return nil, err
		}
		return a.bridge.Network(), nil
	})

	a.registerGame()
}

func walkTree(t *Widget, v lang.Value, depth int) {
	if list, ok := v.(*lang.List); ok {
		if list.Len() == 2 {
			if children, ok := list.Items[1].(*lang.List); ok {
				t.addTreeLabel(lang.ToString(list.Items[0]), depth)
				walkTree(t, children, depth+1)
				return
			}
		}
		for _, item := range list.Items {
			walkTree(t, item, depth)
		}
		return
	}
	t.addTreeLabel(lang.ToString(v), depth)
}

func (w *Widget) addTreeLabel(text string, depth int) {
	idx := len(w.treeTexts)
	label := w.app.addWidget(w.id, "label", strings.Repeat("  ", depth)+"▸ "+text, 16, "#37352F")
	w.treeTexts = append(w.treeTexts, text)
	w.treeLabels = append(w.treeLabels, label)
	label.handlers["click"] = append(label.handlers["click"], func() error {
		for i, l := range w.treeLabels {
			if i == idx {
				_ = l.SetProp("fg", "#9A3412")
			} else {
				_ = l.SetProp("fg", "#37352F")
			}
		}
		w.treeIndex = idx
		w.emit("select")
		return nil
	})
}

func tableJSON(headers, rows lang.Value) (string, error) {
	h, ok := headers.(*lang.List)
	if !ok {
		return "", fmt.Errorf("table 的表头必须是列表")
	}
	r, ok := rows.(*lang.List)
	if !ok {
		return "", fmt.Errorf("table 的数据必须是列表")
	}
	out := make([][]string, 0, r.Len()+1)
	head := make([]string, 0, h.Len())
	for _, v := range h.Items {
		head = append(head, lang.ToString(v))
	}
	out = append(out, head)
	for _, rv := range r.Items {
		row, ok := rv.(*lang.List)
		if !ok {
			return "", fmt.Errorf("table 的每一行必须是列表")
		}
		line := make([]string, 0, row.Len())
		for _, v := range row.Items {
			line = append(line, lang.ToString(v))
		}
		out = append(out, line)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func applyKw(a *App, w *Widget, kw map[string]lang.Value) {
	if v, ok := kw["visible"]; ok {
		w.visible = lang.Truthy(v)
		a.bridge.SetVisible(w.id, w.visible)
	}
	if v, ok := kw["enabled"]; ok {
		w.enabled = lang.Truthy(v)
		a.bridge.SetEnabled(w.id, w.enabled)
	}
	if v, ok := kw["fg"]; ok {
		w.fg = lang.ToString(v)
		a.bridge.SetStyle(w.id, w.fg, w.size)
	}
	if v, ok := kw["size"]; ok {
		w.size = int(number(v))
		a.bridge.SetStyle(w.id, w.fg, w.size)
	}
	if v, ok := kw["font"]; ok {
		if m, ok := v.(*lang.Map); ok {
			if f, ok := m.Get("size"); ok {
				w.size = int(number(f))
				a.bridge.SetStyle(w.id, w.fg, w.size)
			}
		}
	}
	if v, ok := kw["checked"]; ok && w.kind == "checkbox" {
		w.checked = lang.Truthy(v)
		a.bridge.SetChecked(w.id, w.checked)
	}
	if v, ok := kw["max"]; ok && w.kind == "progress" {
		w.max = number(v)
		a.bridge.SetProgress(w.id, int(w.value), int(w.max))
	}
	if v, ok := kw["value"]; ok && w.kind == "progress" {
		w.value = number(v)
		a.bridge.SetProgress(w.id, int(w.value), int(w.max))
	}
}

func parentID(kw map[string]lang.Value) int {
	if v, ok := kw["parent"]; ok {
		if w, ok := v.(*Widget); ok {
			return w.id
		}
	}
	return 1
}

var _ = fmt.Sprintf
