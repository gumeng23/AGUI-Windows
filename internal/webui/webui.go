//go:build js && wasm

// Package webui is a small DOM backend for the Agui language. It lets an Agui
// script build a mobile-friendly web UI that runs inside an Android WebView.
package webui

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"agui/internal/lang"
)

type App struct {
	i       *lang.Interp
	doc     js.Value
	root    js.Value
	widgets map[string]*Widget
	nextID  int
	timers  map[int]timerEntry
	done    chan struct{}
	once    sync.Once
}

type Widget struct {
	app      *App
	id       string
	kind     string
	el       js.Value
	content  js.Value
	titleEl  js.Value
	handlers map[string][]func() error
	props    map[string]lang.Value
	jsFuncs  []js.Func
	closed   bool
}

type timerKind int

const (
	timeoutTimer timerKind = iota
	intervalTimer
)

type timerEntry struct {
	kind timerKind
	id   js.Value
}

func Register(i *lang.Interp) *App {
	doc := js.Global().Get("document")
	root := doc.Call("getElementById", "app")
	if root.IsNull() || root.IsUndefined() {
		root = doc.Get("body")
	}
	a := &App{
		i:       i,
		doc:     doc,
		root:    root,
		widgets: map[string]*Widget{},
		nextID:  1,
		timers:  map[int]timerEntry{},
		done:    make(chan struct{}),
	}
	i.SetDispatcher(a.Post)
	a.install()
	return a
}

func (a *App) Interp() *lang.Interp  { return a.i }
func (a *App) Done() <-chan struct{} { return a.done }

func (a *App) Fatal(err error) {
	msg := "Agui 启动失败"
	if err != nil {
		msg += "\n" + err.Error()
	}
	fmt.Fprintln(os.Stderr, msg)
	js.Global().Get("console").Call("error", msg)
	overlay := a.doc.Call("getElementById", "fatal")
	if overlay.IsNull() || overlay.IsUndefined() {
		return
	}
	overlay.Get("style").Set("display", "flex")
	overlay.Call("querySelector", ".fatal-msg").Set("textContent", msg)
}

func (a *App) Post(fn func()) {
	select {
	case <-a.done:
		return
	default:
	}
	var cb js.Func
	cb = js.FuncOf(func(this js.Value, args []js.Value) any {
		defer cb.Release()
		select {
		case <-a.done:
			return nil
		default:
		}
		fn()
		return nil
	})
	js.Global().Call("setTimeout", cb, 0)
}

func (a *App) quit() {
	a.once.Do(func() { close(a.done) })
	android := js.Global().Get("Android")
	if !android.IsUndefined() && !android.IsNull() && android.Get("finish").Type() == js.TypeFunction {
		android.Call("finish")
	}
}

func (a *App) errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, msg)
	con := js.Global().Get("console")
	if !con.IsUndefined() {
		con.Call("error", msg)
	}
	if toast := js.Global().Get("aguiToast"); toast.Type() == js.TypeFunction {
		toast.Invoke("错误：" + msg)
	}
}

func (a *App) install() {
	a.native("window", a.nativeWindow)
	a.native("panel", a.nativePanel)
	a.native("label", a.nativeLabel)
	a.native("button", a.nativeButton)
	a.native("edit", a.nativeEdit)
	a.native("checkbox", a.nativeCheckbox)
	a.native("progress", a.nativeProgress)
	a.native("image", a.nativeImage)
	a.native("canvas", a.nativeCanvas)
	a.native("font", a.nativeFont)
	a.native("msgbox", a.nativeMsgBox)
	a.native("confirm", a.nativeConfirm)
	a.native("input", a.nativeInput)
	a.native("after", a.nativeAfter)
	a.native("every", a.nativeEvery)
	a.native("cancel_timer", a.nativeCancelTimer)
	a.native("quit", a.nativeQuit)
	a.native("toast", a.nativeToast)
	a.native("vibrate", a.nativeVibrate)
	a.native("open_url", a.nativeOpenURL)
	a.native("play_audio", a.nativePlayAudio)
	a.native("stop_audio", a.nativeStopAudio)
	a.native("storage_set", a.nativeStorageSet)
	a.native("storage_get", a.nativeStorageGet)
	a.native("storage_remove", a.nativeStorageRemove)
	a.native("storage_clear", a.nativeStorageClear)
	a.native("platform", a.nativePlatform)
	a.native("random_int", a.nativeRandomInt)
	a.native("random_float", a.nativeRandomFloat)
	a.native("clamp", a.nativeClamp)
	a.native("now_ms", a.nativeNowMS)
	a.native("screenshot", func(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		return nil, fmt.Errorf("WebView 暂不支持 screenshot()")
	})
	a.native("set_clipboard", func(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("set_clipboard(text) 需要 1 个参数")
		}
		js.Global().Get("navigator").Get("clipboard").Call("writeText", lang.ToString(args[0]))
		return nil, nil
	})
	a.native("get_clipboard", func(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		return "", nil
	})
}

func (a *App) native(name string, fn func([]lang.Value, map[string]lang.Value) (lang.Value, error)) {
	a.i.RegisterNative(name, func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		return fn(args, kwargs)
	})
}

func (a *App) newWidget(kind, tag, class, text string, kwargs map[string]lang.Value) (*Widget, error) {
	el := a.doc.Call("createElement", tag)
	id := fmt.Sprintf("agui-%s-%d", kind, a.nextID)
	a.nextID++
	el.Set("id", id)
	el.Set("className", "agui "+kind+" "+class)
	w := &Widget{
		app:      a,
		id:       id,
		kind:     kind,
		el:       el,
		content:  el,
		handlers: map[string][]func() error{},
		props:    map[string]lang.Value{},
	}
	if kind == "window" {
		outer := a.doc.Call("createElement", "section")
		outer.Set("className", "agui window card")
		head := a.doc.Call("createElement", "header")
		head.Set("className", "win-head")
		titleEl := a.doc.Call("createElement", "span")
		titleEl.Set("className", "win-title")
		titleEl.Set("textContent", "Agui")
		head.Call("appendChild", titleEl)
		closeBtn := a.doc.Call("createElement", "button")
		closeBtn.Set("className", "win-close")
		closeBtn.Set("aria-label", "关闭")
		closeBtn.Set("textContent", "×")
		head.Call("appendChild", closeBtn)
		body := a.doc.Call("createElement", "div")
		body.Set("className", "win-body")
		outer.Call("appendChild", head)
		outer.Call("appendChild", body)
		w.el = outer
		w.content = body
		w.titleEl = titleEl
		w.props["title"] = "Agui"
		w.jsFuncs = append(w.jsFuncs, a.bind(closeBtn, "click", func() {
			if w.closed {
				return
			}
			w.emit("close")
			w.close()
		}))
	} else if text != "" {
		setText(el, text)
	}
	target := a.root
	if pv, ok := kwargs["parent"]; ok {
		p, ok := pv.(*Widget)
		if !ok || p == nil {
			return nil, fmt.Errorf("parent 必须是 Web 控件")
		}
		target = p.content
		if target.IsNull() || target.IsUndefined() {
			target = p.el
		}
	}
	target.Call("appendChild", el)
	a.widgets[id] = w
	a.applyCommon(w, kwargs)
	a.bindWidget(w)
	return w, nil
}

func (a *App) bindWidget(w *Widget) {
	switch w.kind {
	case "button":
		w.jsFuncs = append(w.jsFuncs, a.bind(w.el, "click", func() {
			if w.enabled() {
				w.emit("click")
			}
		}))
	case "edit":
		w.jsFuncs = append(w.jsFuncs, a.bind(w.el, "input", func() {
			w.props["text"] = w.el.Get("value").String()
			w.emit("change")
		}))
	case "checkbox":
		w.jsFuncs = append(w.jsFuncs, a.bind(w.el, "change", func() {
			w.props["checked"] = w.el.Get("checked").Bool()
			w.emit("change")
		}))
	case "image", "canvas":
		w.jsFuncs = append(w.jsFuncs, a.bind(w.el, "click", func() { w.emit("click") }))
	}
}

func (a *App) bind(target js.Value, event string, fn func()) js.Func {
	cb := js.FuncOf(func(this js.Value, args []js.Value) any {
		fn()
		return nil
	})
	target.Call("addEventListener", event, cb)
	return cb
}

func (a *App) applyCommon(w *Widget, kwargs map[string]lang.Value) {
	if v, ok := kwargs["text"]; ok {
		setText(w.el, lang.ToString(v))
		w.props["text"] = v
	}
	if v, ok := kwargs["value"]; ok {
		if w.kind == "progress" {
			w.el.Set("value", number(v))
		} else {
			setText(w.el, lang.ToString(v))
		}
		w.props["value"] = v
	}
	if v, ok := kwargs["placeholder"]; ok {
		w.el.Set("placeholder", lang.ToString(v))
	}
	if v, ok := kwargs["title"]; ok {
		w.el.Set("title", lang.ToString(v))
	}
	if v, ok := kwargs["class"]; ok {
		w.el.Set("className", "agui "+w.kind+" "+lang.ToString(v))
	}
	if v, ok := kwargs["grid"]; ok {
		if n := int(number(v)); n > 1 {
			w.el.Set("className", "agui "+w.kind+" grid grid-"+strconv.Itoa(n))
		}
	}
	if v, ok := kwargs["w"]; ok {
		w.el.Get("style").Set("width", fmt.Sprintf("%.0fpx", number(v)))
	}
	if v, ok := kwargs["h"]; ok {
		w.el.Get("style").Set("minHeight", fmt.Sprintf("%.0fpx", number(v)))
	}
	if v, ok := kwargs["fg"]; ok {
		w.el.Get("style").Set("color", lang.ToString(v))
	}
	if v, ok := kwargs["bg"]; ok {
		w.el.Get("style").Set("backgroundColor", lang.ToString(v))
	}
	if v, ok := kwargs["tooltip"]; ok {
		w.el.Set("title", lang.ToString(v))
	}
	if v, ok := kwargs["visible"]; ok {
		w.el.Get("style").Set("display", boolDisplay(v))
	}
	if v, ok := kwargs["enabled"]; ok {
		w.el.Set("disabled", !lang.Truthy(v))
		w.props["enabled"] = lang.Truthy(v)
	}
	if fv, ok := kwargs["font"]; ok {
		applyFont(w.el, fv)
	}
	if w.kind == "progress" {
		maxv := numberDefault(kwargs["max"], 100)
		val := numberDefault(kwargs["value"], 0)
		w.el.Set("max", maxv)
		w.el.Set("value", val)
		w.props["max"] = maxv
		w.props["value"] = val
	}
	if w.kind == "checkbox" {
		checked := false
		if v, ok := kwargs["checked"]; ok {
			checked = lang.Truthy(v)
		}
		w.el.Set("checked", checked)
		w.props["checked"] = checked
	}
}

func setText(el js.Value, text string) {
	tag := strings.ToLower(el.Get("tagName").String())
	switch tag {
	case "input", "textarea":
		el.Set("value", text)
	default:
		el.Set("textContent", text)
	}
}

func applyFont(el js.Value, v lang.Value) {
	m, ok := v.(*lang.Map)
	if !ok {
		return
	}
	st := el.Get("style")
	if family, ok := m.Get("family"); ok {
		st.Set("fontFamily", lang.ToString(family))
	}
	if size, ok := m.Get("size"); ok {
		st.Set("fontSize", fmt.Sprintf("%.0fpx", number(size)))
	}
	if bold, ok := m.Get("bold"); ok && lang.Truthy(bold) {
		st.Set("fontWeight", "700")
	}
	if italic, ok := m.Get("italic"); ok && lang.Truthy(italic) {
		st.Set("fontStyle", "italic")
	}
}

func boolDisplay(v lang.Value) string {
	if lang.Truthy(v) {
		return ""
	}
	return "none"
}

func (a *App) nativeWindow(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	title := "Agui"
	if len(args) > 0 {
		title = lang.ToString(args[0])
	}
	w, err := a.newWidget("window", "section", "", "", kwargs)
	if err != nil {
		return nil, err
	}
	w.setTitle(title)
	a.root.Set("className", "agui-root has-window")
	return w, nil
}

func (a *App) nativePanel(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	return a.newWidget("panel", "div", "card", "", kwargs)
}

func (a *App) nativeLabel(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	text := ""
	if len(args) > 0 {
		text = lang.ToString(args[0])
	}
	return a.newWidget("label", "div", "", text, kwargs)
}

func (a *App) nativeButton(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	text := ""
	if len(args) > 0 {
		text = lang.ToString(args[0])
	}
	w, err := a.newWidget("button", "button", "", text, kwargs)
	if err != nil {
		return nil, err
	}
	w.el.Set("type", "button")
	return w, nil
}

func (a *App) nativeEdit(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	w, err := a.newWidget("edit", "input", "", "", kwargs)
	if err != nil {
		return nil, err
	}
	w.el.Set("type", "text")
	if len(args) > 0 {
		w.el.Set("value", lang.ToString(args[0]))
	}
	if v, ok := kwargs["placeholder"]; ok {
		w.el.Set("placeholder", lang.ToString(v))
	}
	return w, nil
}

func (a *App) nativeCheckbox(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	text := ""
	if len(args) > 0 {
		text = lang.ToString(args[0])
	}
	wrap, err := a.newWidget("checkbox", "label", "check", "", kwargs)
	if err != nil {
		return nil, err
	}
	box := a.doc.Call("createElement", "input")
	box.Set("type", "checkbox")
	box.Set("className", "agui-check-input")
	checked := false
	if len(args) > 1 && lang.Truthy(args[1]) {
		checked = true
	}
	if v, ok := kwargs["checked"]; ok {
		checked = lang.Truthy(v)
	}
	box.Set("checked", checked)
	wrap.el.Call("appendChild", box)
	lab := a.doc.Call("createElement", "span")
	lab.Set("textContent", text)
	wrap.el.Call("appendChild", lab)
	wrap.el = box
	wrap.content = box
	wrap.props["checked"] = checked
	wrap.jsFuncs = append(wrap.jsFuncs, a.bind(box, "change", func() {
		wrap.props["checked"] = box.Get("checked").Bool()
		wrap.emit("change")
	}))
	return wrap, nil
}

func (a *App) nativeProgress(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	w, err := a.newWidget("progress", "progress", "", "", kwargs)
	if err != nil {
		return nil, err
	}
	if len(args) > 0 {
		w.el.Set("value", number(args[0]))
		w.props["value"] = args[0]
	}
	return w, nil
}

func (a *App) nativeImage(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	w, err := a.newWidget("image", "img", "", "", kwargs)
	if err != nil {
		return nil, err
	}
	if len(args) > 0 {
		w.el.Set("src", lang.ToString(args[0]))
		w.props["src"] = args[0]
	}
	if v, ok := kwargs["src"]; ok {
		w.el.Set("src", lang.ToString(v))
		w.props["src"] = v
	}
	w.el.Set("alt", "OC")
	return w, nil
}

func (a *App) nativeCanvas(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	w, err := a.newWidget("canvas", "canvas", "", "", kwargs)
	if err != nil {
		return nil, err
	}
	w.el.Set("width", 640)
	w.el.Set("height", 640)
	if v, ok := kwargs["w"]; ok {
		w.el.Set("width", number(v))
	}
	if v, ok := kwargs["h"]; ok {
		w.el.Set("height", number(v))
	}
	return w, nil
}

func (a *App) nativeFont(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	m := lang.NewMap()
	family := "system-ui"
	if len(args) > 0 {
		family = lang.ToString(args[0])
	}
	m.Set("family", family)
	size := 16.0
	if len(args) > 1 {
		size = number(args[1])
	}
	m.Set("size", size)
	if v, ok := kwargs["bold"]; ok {
		m.Set("bold", lang.Truthy(v))
	}
	if v, ok := kwargs["italic"]; ok {
		m.Set("italic", lang.Truthy(v))
	}
	return m, nil
}

func (a *App) nativeMsgBox(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	text := ""
	if len(args) > 0 {
		text = lang.ToString(args[0])
	}
	js.Global().Call("alert", text)
	return nil, nil
}

func (a *App) nativeConfirm(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	text := ""
	if len(args) > 0 {
		text = lang.ToString(args[0])
	}
	return js.Global().Call("confirm", text).Bool(), nil
}

func (a *App) nativeInput(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	text := ""
	if len(args) > 0 {
		text = lang.ToString(args[0])
	}
	def := ""
	if len(args) > 1 {
		def = lang.ToString(args[1])
	}
	v := js.Global().Call("prompt", text, def)
	if v.IsNull() || v.IsUndefined() {
		return nil, nil
	}
	return v.String(), nil
}

func (a *App) nativeAfter(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("after(ms, fn) 需要 2 个参数")
	}
	id := a.timer(timeoutTimer, int(number(args[0])), func() { a.callback(args[1]) })
	return float64(id), nil
}

func (a *App) nativeEvery(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("every(ms, fn) 需要 2 个参数")
	}
	ms := int(number(args[0]))
	if ms < 16 {
		ms = 16
	}
	id := a.timer(intervalTimer, ms, func() { a.callback(args[1]) })
	return float64(id), nil
}

func (a *App) callback(fn lang.Value) {
	err := a.i.Protect(func() { a.i.CallValue(fn) })
	if err != nil {
		a.errorf("定时器回调失败：%v", err)
	}
}

func (a *App) timer(kind timerKind, ms int, fn func()) int {
	id := a.nextID
	a.nextID++
	var cb js.Func
	if kind == timeoutTimer {
		cb = js.FuncOf(func(this js.Value, args []js.Value) any {
			defer cb.Release()
			fn()
			delete(a.timers, id)
			return nil
		})
		a.timers[id] = timerEntry{kind: kind, id: js.Global().Call("setTimeout", cb, ms)}
	} else {
		cb = js.FuncOf(func(this js.Value, args []js.Value) any {
			fn()
			return nil
		})
		a.timers[id] = timerEntry{kind: kind, id: js.Global().Call("setInterval", cb, ms)}
	}
	return id
}

func (a *App) nativeCancelTimer(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("cancel_timer(id) 需要 1 个参数")
	}
	id := int(number(args[0]))
	if t, ok := a.timers[id]; ok {
		if t.kind == timeoutTimer {
			js.Global().Call("clearTimeout", t.id)
		} else {
			js.Global().Call("clearInterval", t.id)
		}
		delete(a.timers, id)
	}
	return nil, nil
}

func (a *App) nativeQuit(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	a.quit()
	return nil, nil
}

func (a *App) nativeToast(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	text := ""
	if len(args) > 0 {
		text = lang.ToString(args[0])
	}
	android := js.Global().Get("Android")
	if !android.IsUndefined() && !android.IsNull() && android.Get("toast").Type() == js.TypeFunction {
		android.Call("toast", text)
		return nil, nil
	}
	if f := js.Global().Get("aguiToast"); !f.IsUndefined() && f.Type() == js.TypeFunction {
		f.Invoke(text)
	}
	return nil, nil
}

func (a *App) nativeVibrate(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	ms := 30.0
	if len(args) > 0 {
		ms = number(args[0])
	}
	android := js.Global().Get("Android")
	if !android.IsUndefined() && !android.IsNull() && android.Get("vibrate").Type() == js.TypeFunction {
		android.Call("vibrate", ms)
		return nil, nil
	}
	nav := js.Global().Get("navigator")
	if !nav.IsUndefined() && !nav.Get("vibrate").IsUndefined() {
		nav.Call("vibrate", ms)
	}
	return nil, nil
}

func (a *App) nativeOpenURL(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("open_url(url) 需要 1 个参数")
	}
	js.Global().Call("open", lang.ToString(args[0]), "_blank")
	return nil, nil
}

var audio = map[int]js.Value{}
var audioID int

func (a *App) nativePlayAudio(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("play_audio(path) 需要 1 个参数")
	}
	el := js.Global().Get("Audio").New(lang.ToString(args[0]))
	if v, ok := kwargs["loop"]; ok {
		el.Set("loop", lang.Truthy(v))
	}
	el.Call("play")
	audioID++
	audio[audioID] = el
	return float64(audioID), nil
}

func (a *App) nativeStopAudio(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	for id, el := range audio {
		el.Call("pause")
		delete(audio, id)
	}
	return nil, nil
}

func (a *App) nativeStorageSet(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("storage_set(key, value) 需要 2 个参数")
	}
	key := lang.ToString(args[0])
	data := js.Global().Get("JSON").Call("stringify", toJS(args[1])).String()
	js.Global().Get("localStorage").Call("setItem", key, data)
	return nil, nil
}

func (a *App) nativeStorageGet(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("storage_get(key) 需要至少 1 个参数")
	}
	key := lang.ToString(args[0])
	raw := js.Global().Get("localStorage").Call("getItem", key)
	if raw.IsNull() || raw.IsUndefined() {
		if len(args) > 1 {
			return args[1], nil
		}
		return nil, nil
	}
	return fromJS(js.Global().Get("JSON").Call("parse", raw.String())), nil
}

func (a *App) nativeStorageRemove(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("storage_remove(key) 需要 1 个参数")
	}
	js.Global().Get("localStorage").Call("removeItem", lang.ToString(args[0]))
	return nil, nil
}

func (a *App) nativeStorageClear(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	js.Global().Get("localStorage").Call("clear")
	return nil, nil
}

func (a *App) nativePlatform(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	android := js.Global().Get("Android")
	if !android.IsUndefined() && !android.IsNull() {
		return "android", nil
	}
	return "web", nil
}

func (a *App) nativeRandomInt(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	lo, hi := 0, 100
	if len(args) > 0 {
		lo = int(number(args[0]))
	}
	if len(args) > 1 {
		hi = int(number(args[1]))
	}
	if hi < lo {
		lo, hi = hi, lo
	}
	if hi == lo {
		return float64(lo), nil
	}
	return float64(lo + rand.Intn(hi-lo+1)), nil
}

func (a *App) nativeRandomFloat(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	return rand.Float64(), nil
}

func (a *App) nativeClamp(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("clamp(v, min, max) 需要 3 个参数")
	}
	v, lo, hi := number(args[0]), number(args[1]), number(args[2])
	if v < lo {
		v = lo
	}
	if v > hi {
		v = hi
	}
	return v, nil
}

func (a *App) nativeNowMS(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	return float64(time.Now().UnixMilli()), nil
}

func (w *Widget) setTitle(title string) {
	w.props["title"] = title
	if !w.titleEl.IsNull() && !w.titleEl.IsUndefined() {
		w.titleEl.Set("textContent", title)
	}
}

func (w *Widget) close() {
	if w.closed {
		return
	}
	w.closed = true
	w.el.Get("style").Set("display", "none")
}

func (w *Widget) enabled() bool {
	if v, ok := w.props["enabled"]; ok {
		return lang.Truthy(v)
	}
	return true
}

func (w *Widget) emit(event string) {
	for _, h := range w.handlers[event] {
		h := h
		err := w.app.i.Protect(func() {
			if e := h(); e != nil {
				w.app.errorf("%s 事件失败：%v", event, e)
			}
		})
		if err != nil {
			w.app.errorf("%s 事件失败：%v", event, err)
		}
	}
}

func (w *Widget) Kind() string { return w.kind }

func (w *Widget) GetProp(name string) (lang.Value, bool) {
	switch name {
	case "text":
		if w.kind == "edit" {
			return w.el.Get("value").String(), true
		}
		return w.el.Get("textContent").String(), true
	case "value":
		if w.kind == "progress" {
			return w.el.Get("value").Float(), true
		}
		if w.kind == "edit" {
			return w.el.Get("value").String(), true
		}
	case "checked":
		if w.kind == "checkbox" {
			return w.el.Get("checked").Bool(), true
		}
	case "visible":
		return w.el.Get("style").Get("display").String() != "none", true
	case "enabled":
		return w.enabled(), true
	case "src":
		if w.kind == "image" {
			return w.el.Get("src").String(), true
		}
	case "width", "w":
		return w.el.Get("clientWidth").Float(), true
	case "height", "h":
		return w.el.Get("clientHeight").Float(), true
	case "title":
		if w.kind == "window" {
			return w.props["title"], true
		}
	}
	if v, ok := w.props[name]; ok {
		return v, true
	}
	return nil, false
}

func (w *Widget) SetProp(name string, v lang.Value) error {
	if w.kind == "progress" && (name == "value" || name == "progress") {
		w.el.Set("value", number(v))
		w.props["value"] = v
		return nil
	}
	switch name {
	case "text":
		setText(w.el, lang.ToString(v))
		w.props[name] = v
		return nil
	case "value":
		if w.kind == "edit" {
			w.el.Set("value", lang.ToString(v))
		} else {
			setText(w.el, lang.ToString(v))
		}
		w.props[name] = v
		return nil
	case "checked":
		if w.kind == "checkbox" {
			w.el.Set("checked", lang.Truthy(v))
			w.props["checked"] = lang.Truthy(v)
			return nil
		}
	case "visible":
		w.el.Get("style").Set("display", boolDisplay(v))
		return nil
	case "enabled":
		w.el.Set("disabled", !lang.Truthy(v))
		w.props["enabled"] = lang.Truthy(v)
		return nil
	case "fg":
		w.el.Get("style").Set("color", lang.ToString(v))
		return nil
	case "bg":
		w.el.Get("style").Set("backgroundColor", lang.ToString(v))
		return nil
	case "src":
		if w.kind == "image" {
			w.el.Set("src", lang.ToString(v))
			w.props["src"] = v
			return nil
		}
	case "title":
		w.setTitle(lang.ToString(v))
		return nil
	case "max":
		if w.kind == "progress" {
			w.el.Set("max", number(v))
			return nil
		}
	}
	return fmt.Errorf("%s 不支持属性 %q", w.kind, name)
}

func (w *Widget) CallMethod(name string, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, bool, error) {
	switch name {
	case "click":
		w.emit("click")
		return nil, true, nil
	case "close":
		w.emit("close")
		w.close()
		return nil, true, nil
	case "focus":
		w.el.Call("focus")
		return nil, true, nil
	case "clear":
		if w.kind == "edit" {
			w.el.Set("value", "")
		} else {
			w.el.Set("textContent", "")
		}
		return nil, true, nil
	case "show":
		w.el.Get("style").Set("display", "")
		return nil, true, nil
	case "hide":
		w.el.Get("style").Set("display", "none")
		return nil, true, nil
	case "set_text":
		if len(args) != 1 {
			return nil, true, fmt.Errorf("set_text(text) 需要 1 个参数")
		}
		setText(w.el, lang.ToString(args[0]))
		return nil, true, nil
	case "append":
		if len(args) != 1 {
			return nil, true, fmt.Errorf("append(text) 需要 1 个参数")
		}
		w.el.Set("textContent", w.el.Get("textContent").String()+lang.ToString(args[0]))
		return nil, true, nil
	case "scroll_into_view":
		w.el.Call("scrollIntoView", map[string]any{"behavior": "smooth", "block": "nearest"})
		return nil, true, nil
	}
	return nil, false, nil
}

func (w *Widget) Listen(event string, fn func() error) error {
	allowed := false
	switch w.kind {
	case "button":
		allowed = event == "click"
	case "edit":
		allowed = event == "change" || event == "input"
	case "checkbox":
		allowed = event == "change"
	case "image":
		allowed = event == "click"
	case "window":
		allowed = event == "close"
	case "canvas":
		allowed = event == "click"
	}
	if !allowed {
		return fmt.Errorf("%s 不支持事件 %q", w.kind, event)
	}
	w.handlers[event] = append(w.handlers[event], fn)
	return nil
}

func number(v lang.Value) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	case bool:
		if x {
			return 1
		}
	}
	return 0
}

func numberDefault(v lang.Value, def float64) float64 {
	if v == nil {
		return def
	}
	return number(v)
}

func toJS(v lang.Value) any {
	switch x := v.(type) {
	case nil:
		return nil
	case bool:
		return x
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		return x
	case *lang.List:
		out := make([]any, 0, len(x.Items))
		for _, item := range x.Items {
			out = append(out, toJS(item))
		}
		return out
	case *lang.Map:
		out := map[string]any{}
		for _, k := range x.Keys {
			out[k] = toJS(x.M[k])
		}
		return out
	case *lang.Bytes:
		return string(x.Data)
	default:
		return lang.ToString(v)
	}
}

func fromJS(v js.Value) lang.Value {
	switch v.Type() {
	case js.TypeNull, js.TypeUndefined:
		return nil
	case js.TypeBoolean:
		return v.Bool()
	case js.TypeNumber:
		return v.Float()
	case js.TypeString:
		return v.String()
	case js.TypeObject:
		if v.InstanceOf(js.Global().Get("Array")) {
			n := v.Length()
			items := make([]lang.Value, 0, n)
			for i := 0; i < n; i++ {
				items = append(items, fromJS(v.Index(i)))
			}
			return lang.NewList(items...)
		}
		m := lang.NewMap()
		keys := js.Global().Get("Object").Call("keys", v)
		for i := 0; i < keys.Length(); i++ {
			k := keys.Index(i).String()
			m.Set(k, fromJS(v.Get(k)))
		}
		return m
	default:
		return v.String()
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
