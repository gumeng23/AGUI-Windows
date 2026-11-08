// Package mobile is the native Android runtime for Agui. It is bound with
// gomobile and talks to a Java Activity through the Bridge interface. The
// interpreter is the same Go implementation used by the desktop build.
package mobile

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"agui/internal/ast"
	"agui/internal/lang"
	"agui/internal/lexer"
	"agui/internal/parser"
	"agui/internal/token"
)

// Bridge is implemented by the Java Activity. All methods are called from Go
// and the Java side is responsible for marshalling them onto the UI thread.
type Bridge interface {
	AddWidget(id int, parent int, kind string, text string)
	SetText(id int, text string)
	SetProgress(id int, value int, max int)
	SetChecked(id int, checked bool)
	SetImage(id int, src string)
	SetStyle(id int, fg string, size int)
	SetVisible(id int, visible bool)
	SetEnabled(id int, enabled bool)
	Toast(text string)
	Alert(title string, message string)
	Finish()
	CanvasOps(id int, json string)
	SaveData(name string, data string)
	LoadData(name string) string
	Share(text string)
	PickFile(mime string)
	RequestPermission(name string)
	RequestPermissions(names string)
	Foreground(title string, text string)
	StopForeground()
	Camera()
	Location() string
	StartRecording()
	StopRecording()
	Sensor(name string) string
	SetTable(id int, json string)
	SetList(id int, json string)
	Confirm(title string, message string)
	SaveFile(name string, content string)
	OpenURL(url string)
	Vibrate(ms int)
	Notify(title string, text string)
	SetClipboard(text string)
	GetClipboard() string
}

// App owns the mobile widget tree and the Agui interpreter.
type App struct {
	bridge Bridge
	i      *lang.Interp
	mu     sync.Mutex

	widgets     map[int]*Widget
	nextID      int
	windowID    int
	timers      map[int]*time.Timer
	nextTimer   int
	done        chan struct{}
	closeOnce   sync.Once
	game        *gameState
	lastFile    string
	lastPerm    string
	lastPermOK  bool
	lastConfirm bool
	lastSaved   string
	lastIntent  string
}

// Widget is a native view exposed to Agui.
type Widget struct {
	app      *App
	id       int
	parent   int
	kind     string
	text     string
	value    float64
	max      float64
	checked  bool
	visible  bool
	enabled  bool
	src      string
	fg       string
	size     int
	handlers map[string][]func() error

	ops       []canvasOp
	batch     bool
	mouseX    float64
	mouseY    float64
	mouseDown bool
	mouseBtn  string

	tabHeader  *Widget
	tabPages   []*Widget
	tabButtons []*Widget
	activeTab  int

	treeLabels []*Widget
	treeTexts  []string
	treeIndex  int
}

var (
	currentMu sync.Mutex
	current   *App
)

// Pause suspends the game loop (called from Activity.onPause).
func Pause() {
	currentMu.Lock()
	a := current
	currentMu.Unlock()
	if a != nil && a.game != nil {
		a.game.paused = true
	}
}

// Resume resumes the game loop (called from Activity.onResume).
func Resume() {
	currentMu.Lock()
	a := current
	currentMu.Unlock()
	if a != nil && a.game != nil {
		a.game.paused = false
	}
}

// Share sends plain text to the Android share sheet.
func Share(text string) {
	currentMu.Lock()
	a := current
	currentMu.Unlock()
	if a != nil && a.bridge != nil {
		a.bridge.Share(text)
	}
}

// Start parses and runs a script. It returns immediately; errors are reported
// through the bridge. Only one script can run at a time.
func Start(script string, bridge Bridge) {
	stopCurrent()
	go func() {
		app := &App{
			bridge:  bridge,
			widgets: map[int]*Widget{},
			nextID:  100,
			timers:  map[int]*time.Timer{},
			done:    make(chan struct{}),
		}
		currentMu.Lock()
		current = app
		currentMu.Unlock()

		i := lang.New()
		app.i = i
		i.SetSource("app.ag", script)
		registerUI(i, app)

		prog, err := parse(script)
		if err != nil {
			bridge.Toast("语法错误：" + err.Error())
			app.finish()
			return
		}
		if err := i.Execute(prog); err != nil {
			bridge.Toast("运行错误：" + i.FormatError(err))
			app.finish()
			return
		}
		<-app.done
	}()
}

func stopCurrent() {
	currentMu.Lock()
	old := current
	current = nil
	currentMu.Unlock()
	if old != nil {
		old.finish()
	}
}

func (a *App) finish() {
	a.closeOnce.Do(func() {
		a.stopGame()
		a.mu.Lock()
		for _, t := range a.timers {
			t.Stop()
		}
		a.timers = map[int]*time.Timer{}
		a.mu.Unlock()
		close(a.done)
		if a.bridge != nil {
			a.bridge.Finish()
		}
	})
}

func (a *App) addWidget(parent int, kind, text string, size int, fg string) *Widget {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := a.nextID
	a.nextID++
	if kind == "window" {
		id = 1
		a.windowID = 1
		parent = 0
	}
	w := &Widget{
		app: a, id: id, parent: parent, kind: kind, text: text,
		visible: true, enabled: true, fg: fg, size: size,
		max: 100, handlers: map[string][]func() error{},
	}
	a.widgets[id] = w
	a.bridge.AddWidget(id, parent, kind, text)
	if kind == "window" {
		a.bridge.SetStyle(id, fg, size)
	}
	return w
}

func (a *App) get(id int) *Widget {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.widgets[id]
}

func (a *App) timer(ms int, repeat bool, fn lang.Value) int {
	if ms < 1 {
		ms = 1
	}
	a.mu.Lock()
	id := a.nextTimer
	a.nextTimer++
	a.mu.Unlock()

	var fire func()
	fire = func() {
		a.i.Protect(func() { a.i.CallValue(fn) })
		if repeat {
			a.mu.Lock()
			if _, ok := a.timers[id]; ok {
				a.timers[id] = time.AfterFunc(time.Duration(ms)*time.Millisecond, fire)
			}
			a.mu.Unlock()
		}
	}
	t := time.AfterFunc(time.Duration(ms)*time.Millisecond, fire)
	a.mu.Lock()
	a.timers[id] = t
	a.mu.Unlock()
	return id
}

func (a *App) cancelTimer(id int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if t, ok := a.timers[id]; ok {
		t.Stop()
		delete(a.timers, id)
	}
}

func (w *Widget) Kind() string { return w.kind }

func (w *Widget) Describe() string {
	return fmt.Sprintf("<%s #%d>", w.kind, w.id)
}

func (w *Widget) GetProp(name string) (lang.Value, bool) {
	switch name {
	case "text", "value_text":
		return w.text, true
	case "value":
		return w.value, true
	case "max":
		return w.max, true
	case "checked":
		return w.checked, true
	case "visible":
		return w.visible, true
	case "enabled":
		return w.enabled, true
	case "src", "path":
		return w.src, true
	case "id":
		return float64(w.id), true
	case "kind":
		return w.kind, true
	case "mouse_x":
		return w.mouseX, true
	case "mouse_y":
		return w.mouseY, true
	case "mouse_down":
		return w.mouseDown, true
	case "mouse_button", "mouse_btn":
		return w.mouseBtn, true
	case "file", "last_file":
		return w.app.lastFile, true
	case "permission", "permission_name":
		return w.app.lastPerm, true
	case "permission_granted":
		return w.app.lastPermOK, true
	case "confirm", "confirmed":
		return w.app.lastConfirm, true
	case "active", "active_tab":
		return float64(w.activeTab), true
	case "page_count":
		return float64(len(w.tabPages)), true
	case "saved", "saved_file":
		return w.app.lastSaved, true
	case "intent", "intent_url":
		return w.app.lastIntent, true
	case "selected":
		if w.kind == "tree" {
			return float64(w.treeIndex), true
		}
	case "count":
		if w.kind == "tree" {
			return float64(len(w.treeTexts)), true
		}
	}
	return nil, false
}

func (w *Widget) SetProp(name string, v lang.Value) error {
	switch name {
	case "text", "value_text":
		w.text = lang.ToString(v)
		w.app.bridge.SetText(w.id, w.text)
	case "value":
		w.value = number(v)
		w.app.bridge.SetProgress(w.id, int(w.value), int(w.max))
	case "max":
		w.max = number(v)
		w.app.bridge.SetProgress(w.id, int(w.value), int(w.max))
	case "checked":
		w.checked = lang.Truthy(v)
		w.app.bridge.SetChecked(w.id, w.checked)
	case "visible":
		w.visible = lang.Truthy(v)
		w.app.bridge.SetVisible(w.id, w.visible)
	case "enabled":
		w.enabled = lang.Truthy(v)
		w.app.bridge.SetEnabled(w.id, w.enabled)
	case "fg", "color":
		w.fg = lang.ToString(v)
		w.app.bridge.SetStyle(w.id, w.fg, w.size)
	case "src", "path":
		w.src = lang.ToString(v)
		w.app.bridge.SetImage(w.id, w.src)
	case "active", "active_tab":
		return w.activateTab(int(number(v)))
	default:
		return fmt.Errorf("控件 %s 没有可写属性 %q", w.kind, name)
	}
	return nil
}

func (w *Widget) CallMethod(name string, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, bool, error) {
	if w.kind == "canvas" {
		if v, ok, err := w.canvasCall(name, args, kwargs); ok {
			return v, true, err
		}
	}
	switch name {
	case "click":
		w.emit("click")
		return nil, true, nil
	case "close":
		w.emit("close")
		w.app.finish()
		return nil, true, nil
	case "focus":
		return nil, true, nil
	case "show":
		return nil, true, w.SetProp("visible", true)
	case "hide":
		return nil, true, w.SetProp("visible", false)
	case "set_text":
		if len(args) < 1 {
			return nil, true, fmt.Errorf("set_text 需要一个参数")
		}
		return nil, true, w.SetProp("text", args[0])
	case "append":
		if len(args) < 1 {
			return nil, true, fmt.Errorf("append 需要一个参数")
		}
		return nil, true, w.SetProp("text", w.text+lang.ToString(args[0]))
	case "clear":
		return nil, true, w.SetProp("text", "")
	case "set_checked":
		if len(args) < 1 {
			return nil, true, fmt.Errorf("set_checked 需要一个参数")
		}
		return nil, true, w.SetProp("checked", args[0])
	case "add_action":
		if w.kind != "actionbar" {
			return nil, true, fmt.Errorf("add_action 只能用于 actionbar")
		}
		if len(args) < 1 {
			return nil, true, fmt.Errorf("add_action 需要标题")
		}
		item := w.app.addWidget(w.id, "action", lang.ToString(args[0]), 15, "#FFFFFF")
		return item, true, nil
	case "add_page":
		if w.kind != "tabs" {
			return nil, true, fmt.Errorf("add_page 只能用于 tabs")
		}
		if len(args) < 1 {
			return nil, true, fmt.Errorf("add_page 需要标题")
		}
		return w.addTabPage(lang.ToString(args[0])), true, nil
	case "activate_tab":
		if w.kind != "tabs" {
			return nil, true, fmt.Errorf("activate_tab 只能用于 tabs")
		}
		if len(args) < 1 {
			return nil, true, fmt.Errorf("activate_tab 需要索引")
		}
		return nil, true, w.activateTab(int(number(args[0])))
	}
	return nil, false, nil
}

func (w *Widget) addTabPage(title string) *Widget {
	if w.tabHeader == nil {
		w.tabHeader = w.app.addWidget(w.id, "row", "", 16, "")
	}
	idx := len(w.tabPages)
	btn := w.app.addWidget(w.tabHeader.id, "button", title, 14, "#FFFFFF")
	page := w.app.addWidget(w.id, "panel", "", 16, "")
	w.tabButtons = append(w.tabButtons, btn)
	w.tabPages = append(w.tabPages, page)
	btn.handlers["click"] = append(btn.handlers["click"], func() error {
		return w.activateTab(idx)
	})
	_ = page.SetProp("visible", idx == 0)
	w.activeTab = 0
	return page
}

func (w *Widget) activateTab(i int) error {
	if i < 0 || i >= len(w.tabPages) {
		return fmt.Errorf("标签页索引越界：%d", i)
	}
	for n, p := range w.tabPages {
		_ = p.SetProp("visible", n == i)
	}
	w.activeTab = i
	return nil
}

func (w *Widget) Listen(event string, fn func() error) error {
	switch event {
	case "click":
		if w.kind != "button" && w.kind != "window" && w.kind != "image" && w.kind != "label" && w.kind != "action" {
			return fmt.Errorf("%s 不支持 click 事件", w.kind)
		}
	case "change":
		if w.kind != "edit" && w.kind != "checkbox" {
			return fmt.Errorf("%s 不支持 change 事件", w.kind)
		}
	case "close", "file", "permission", "confirm", "saved", "intent":
		if w.kind != "window" {
			return fmt.Errorf("%s 不支持 %s 事件", w.kind, event)
		}
	case "select":
		if w.kind != "tree" {
			return fmt.Errorf("%s 不支持 select 事件", w.kind)
		}
	case "mousedown", "mouseup", "mousemove":
		if w.kind != "canvas" {
			return fmt.Errorf("%s 不支持 %s 事件", w.kind, event)
		}
	default:
		return fmt.Errorf("%s 不支持事件 %q", w.kind, event)
	}
	w.handlers[event] = append(w.handlers[event], fn)
	return nil
}

func (w *Widget) emit(event string) {
	for _, fn := range w.handlers[event] {
		if err := fn(); err != nil && w.app.bridge != nil {
			w.app.bridge.Toast("错误：" + err.Error())
		}
	}
}

// DispatchClick is called by Java when a button/view is clicked.
func DispatchClick(id int) {
	currentMu.Lock()
	a := current
	currentMu.Unlock()
	if a == nil {
		return
	}
	if w := a.get(id); w != nil {
		w.emit("click")
	}
}

// DispatchChange is called by Java for edit/checkbox changes.
func DispatchChange(id int, text string, checked bool) {
	currentMu.Lock()
	a := current
	currentMu.Unlock()
	if a == nil {
		return
	}
	w := a.get(id)
	if w == nil {
		return
	}
	if w.kind == "edit" {
		w.text = text
		w.emit("change")
	}
	if w.kind == "checkbox" {
		w.checked = checked
		w.emit("change")
	}
}

// DispatchFile is called by Java after the user picks a file. The path is a
// private cache copy, so scripts can read it without URI permissions.
func DispatchFile(path string) {
	currentMu.Lock()
	a := current
	currentMu.Unlock()
	if a == nil {
		return
	}
	a.lastFile = path
	if w := a.get(a.windowID); w != nil {
		w.emit("file")
	}
}

// DispatchConfirm is called by Java after a confirm dialog.
func DispatchConfirm(ok bool) {
	currentMu.Lock()
	a := current
	currentMu.Unlock()
	if a == nil {
		return
	}
	a.lastConfirm = ok
	if w := a.get(a.windowID); w != nil {
		w.emit("confirm")
	}
}

// DispatchSaved is called by Java after the user saves a document.
func DispatchSaved(path string) {
	currentMu.Lock()
	a := current
	currentMu.Unlock()
	if a == nil {
		return
	}
	a.lastSaved = path
	if w := a.get(a.windowID); w != nil {
		w.emit("saved")
	}
}

// DispatchIntent is called by Java when the activity receives a deep link.
func DispatchIntent(uri string) {
	currentMu.Lock()
	a := current
	currentMu.Unlock()
	if a == nil {
		return
	}
	a.lastIntent = uri
	if w := a.get(a.windowID); w != nil {
		w.emit("intent")
	}
}

// DispatchPermission is called by Java after a runtime permission dialog.
func DispatchPermission(name string, granted bool) {
	currentMu.Lock()
	a := current
	currentMu.Unlock()
	if a == nil {
		return
	}
	a.lastPerm = name
	a.lastPermOK = granted
	if w := a.get(a.windowID); w != nil {
		w.emit("permission")
	}
}

// DispatchClose is called by Java when the Activity is closing.
func DispatchClose() {
	currentMu.Lock()
	a := current
	currentMu.Unlock()
	if a == nil {
		return
	}
	if w := a.get(a.windowID); w != nil {
		w.emit("close")
	}
	a.finish()
}

func number(v lang.Value) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
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

func textArg(args []lang.Value) string {
	if len(args) == 0 {
		return ""
	}
	return lang.ToString(args[0])
}

func kwString(kwargs map[string]lang.Value, key, def string) string {
	if v, ok := kwargs[key]; ok {
		return lang.ToString(v)
	}
	return def
}

func kwNumber(kwargs map[string]lang.Value, key string, def float64) float64 {
	if v, ok := kwargs[key]; ok {
		return number(v)
	}
	return def
}

func kwBool(kwargs map[string]lang.Value, key string, def bool) bool {
	if v, ok := kwargs[key]; ok {
		return lang.Truthy(v)
	}
	return def
}

func argCount(name string, args []lang.Value, min, max int) error {
	if len(args) < min || len(args) > max {
		if min == max {
			return fmt.Errorf("%s 需要 %d 个参数，收到 %d 个", name, min, len(args))
		}
		return fmt.Errorf("%s 需要 %d 到 %d 个参数，收到 %d 个", name, min, max, len(args))
	}
	return nil
}

func parse(src string) (*ast.Program, error) {
	return parseAgui(src)
}

func parseAgui(src string) (*ast.Program, error) {
	toks, err := lexer.Lex(src)
	if err != nil {
		return nil, err
	}
	return parser.Parse(toks)
}

func lex(src string) ([]token.Token, error) { return lexer.Lex(src) }
func parseTokens(toks []token.Token) (*ast.Program, error) {
	return parser.Parse(toks)
}
