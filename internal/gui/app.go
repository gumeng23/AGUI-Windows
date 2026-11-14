package gui

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"agui/internal/lang"
)

const (
	mainClassName   = "AguiMainWindow"
	dialogClassName = "AguiInputDialog"
	imageClassName  = "AguiImage"
	canvasClassName = "AguiCanvas"
	panelClassName  = "AguiPanel"
)

// App owns the Win32 state of one Agui GUI program.
type App struct {
	interp *lang.Interp

	hwnd      syscall.Handle
	window    *Widget // active top-level window
	windows   []*Widget
	main      *Widget
	widgets   map[syscall.Handle]*Widget
	byID      map[int]*Widget
	menuItems map[int]*MenuItem
	hMenu     syscall.Handle
	nextID    int
	margin    int32
	gap       int32
	fontSize  float64
	darkTheme bool

	dpi   int32
	scale float64

	hFont      syscall.Handle
	bgColor    uint32
	bgBrush    syscall.Handle
	whiteBrush syscall.Handle
	brushes    map[uint32]syscall.Handle
	fonts      []syscall.Handle

	timers         map[uintptr]*timerEntry
	nextTimer      uintptr
	asyncMu        sync.Mutex
	asyncFns       map[uintptr]func()
	nextAsync      uintptr
	tasks          []*lang.Task
	pendingUI      []func()
	stopped        bool
	accelTable     syscall.Handle
	tableEdits     map[syscall.Handle]*Widget
	pendingParent  *Widget
	tray           *Tray
	minimizeToTray bool
	taskbarMsg     uint32
	tooltipHwnd    syscall.Handle
	tooltipText    map[syscall.Handle][]uint16
	game           *gameRuntime
	gameImages     map[string]*gameImage
	gameFonts      map[string]syscall.Handle
	audio          map[string]*audioChannel
	startupShot    string
	ran            bool
	running        bool
	exitCode       int
	windowStyle    uint32
}

type timerEntry struct {
	id     uintptr
	hwnd   syscall.Handle
	repeat bool
	fn     func() error
}

var currentApp *App

var mainWndProcCallback = syscall.NewCallback(mainWndProc)

// NewApp prepares the Win32 side. It must be called on the goroutine that will
// create windows; the goroutine is locked to its OS thread for Win32's sake.
func NewApp(i *lang.Interp) *App {
	runtime.LockOSThread()
	a := &App{
		interp:      i,
		widgets:     map[syscall.Handle]*Widget{},
		byID:        map[int]*Widget{},
		menuItems:   map[int]*MenuItem{},
		brushes:     map[uint32]syscall.Handle{},
		timers:      map[uintptr]*timerEntry{},
		asyncFns:    map[uintptr]func(){},
		tableEdits:  map[syscall.Handle]*Widget{},
		tooltipText: map[syscall.Handle][]uint16{},
		gameImages:  map[string]*gameImage{},
		gameFonts:   map[string]syscall.Handle{},
		audio:       map[string]*audioChannel{},
		nextID:      1000,
		nextTimer:   1,
		margin:      20,
		gap:         10,
		fontSize:    10,
	}
	processPerMonitor = enablePerMonitorDPI()
	a.dpi = systemDPI()
	a.scale = float64(a.dpi) / 96.0
	if a.scale < 1 {
		a.scale = 1
	}
	a.margin = a.px(20)
	a.gap = a.px(10)
	a.fontSize = 10

	// Activate the comctl32 v6 manifest before registering any common control
	// classes; this is what gives buttons, edits and check boxes their modern
	// visual style instead of the Win95 look.
	_ = enableVisualStyles()
	initCommonControls()
	_ = initRichEdit()
	a.hFont = createFont("Microsoft YaHei UI", 10, false, false, a.dpi)
	a.fonts = append(a.fonts, a.hFont)
	a.whiteBrush = createBrush(parseColorOr("#FFFFFF", 0xFFFFFF))
	a.setBackground("#F7F8FA")

	if err := registerWindowClass(mainClassName, mainWndProcCallback); err != nil {
		panic(err)
	}
	if err := registerWindowClass(dialogClassName, dialogProcCallback); err != nil {
		panic(err)
	}
	if err := registerWindowClass(imageClassName, imageProcCallback); err != nil {
		panic(err)
	}
	if err := registerWindowClass(canvasClassName, canvasProcCallback); err != nil {
		panic(err)
	}
	if err := registerWindowClass(panelClassName, panelProcCallback); err != nil {
		panic(err)
	}
	i.SetDispatcher(a.Post)
	if msg, _, _ := pRegisterWindowMs.Call(uintptr(unsafe.Pointer(u16("TaskbarCreated")))); msg != 0 {
		a.taskbarMsg = uint32(msg)
	}
	currentApp = a
	return a
}

func (a *App) px(v float64) int32 {
	return int32(v*float64(a.scale) + 0.5)
}

func (a *App) logical(v int32) float64 {
	return float64(v) / float64(a.scale)
}

func (a *App) setBackground(color string) uint32 {
	c := parseColorOr(color, 0xF7F8FA)
	a.bgColor = c
	if a.bgBrush != 0 {
		deleteObject(a.bgBrush)
	}
	a.bgBrush = createBrush(c)
	if a.window != nil {
		a.setWindowBackground(a.window, color)
	}
	return c
}

func (a *App) brushFor(color uint32) syscall.Handle {
	if color == a.bgColor {
		return a.bgBrush
	}
	if color == 0xFFFFFF {
		return a.whiteBrush
	}
	if h, ok := a.brushes[color]; ok {
		return h
	}
	h := createBrush(color)
	a.brushes[color] = h
	return h
}

// EnsureWindow creates the implicit main window if it does not exist yet.
func (a *App) EnsureWindow() *Widget {
	if len(a.windows) > 0 {
		if a.window == nil {
			a.activate(a.windows[len(a.windows)-1])
		}
		return a.window
	}
	w, err := a.createWindow("Agui 应用", 900, 640, "#F7F8FA", true, nil)
	if err != nil {
		panic(err)
	}
	return w
}

// applyDPI updates the cached scale and re-applies the default UI font after a
// window moves to a monitor with a different DPI.
func (a *App) applyDPI(dpi int32) {
	if dpi < 72 || dpi == a.dpi {
		return
	}
	a.dpi = dpi
	a.scale = float64(dpi) / 96.0
	if a.scale < 1 {
		a.scale = 1
	}
	a.margin = a.px(20)
	a.gap = a.px(10)
	h := createFont("Microsoft YaHei UI", 10, false, false, dpi)
	a.fonts = append(a.fonts, h)
	a.hFont = h
	for _, w := range a.widgets {
		if w.hwnd != 0 && w.font == nil {
			sendMessage(w.hwnd, wmSetFont, uintptr(h), 1)
		}
	}
}

func (a *App) activate(w *Widget) {
	if w == nil {
		a.hwnd = 0
		a.window = nil
		return
	}
	a.hwnd = w.hwnd
	a.window = w
	a.windowStyle = w.windowStyle
}

func (a *App) windowByHWND(h syscall.Handle) *Widget {
	for _, w := range a.windows {
		if w.hwnd == h {
			return w
		}
	}
	return nil
}

func (a *App) setWindowBackground(w *Widget, bg string) {
	c := parseColorOr(bg, 0xF7F8FA)
	w.bgColor = c
	if w.bgBrush != 0 {
		deleteObject(w.bgBrush)
	}
	w.bgBrush = createBrush(c)
}

// createWindow always creates a new top-level window and makes it active.
// Scripts that need controls in a previous window can call win.use().
func (a *App) createWindow(title string, width, height int, bg string, resizable bool, parent *Widget) (*Widget, error) {
	style := uint32(wsOverlappedWindow)
	if !resizable {
		style &^= wsThickFrame | wsMinimizeBox | wsMaximizeBox
	}
	exStyle := uint32(wsExControlParent | wsExAppWindow)
	r := rect{0, 0, a.px(float64(width)), a.px(float64(height))}
	adjustWindowRect(&r, style, false, exStyle)
	adjW := r.right - r.left
	adjH := r.bottom - r.top
	var owner syscall.Handle
	if parent != nil && parent.kind == "window" {
		owner = parent.hwnd
	}
	hwnd := createWindow(exStyle, mainClassName, title, style, cwUseDefault, cwUseDefault, adjW, adjH, owner, 0)
	if hwnd == 0 {
		return nil, fmt.Errorf("创建窗口失败")
	}
	w := &Widget{app: a, hwnd: hwnd, kind: "window", id: 0, parent: parent, opacity: 1}
	w.title = title
	w.displayH = a.px(float64(height))
	w.ctrlH = a.px(float64(height))
	w.w = a.px(float64(width))
	w.h = a.px(float64(height))
	w.windowStyle = style
	a.windows = append(a.windows, w)
	if a.main == nil {
		a.main = w
	}
	a.widgets[hwnd] = w
	pDragAcceptFiles.Call(uintptr(hwnd), 1)
	a.activate(w)
	a.setWindowBackground(w, bg)
	cr, _ := getClientRect(hwnd)
	w.w = cr.right
	w.h = cr.bottom
	if a.running {
		showWindow(hwnd, swShowNormal)
		pUpdateWindow.Call(uintptr(hwnd))
		pSetForegroundWindow.Call(uintptr(hwnd))
	}
	return w, nil
}

func (a *App) resizeWindowClient(w *Widget, cw, ch int32) {
	if w == nil || w.hwnd == 0 {
		return
	}
	r := rect{0, 0, cw, ch}
	adjustWindowRect(&r, w.windowStyle, false, wsExControlParent)
	adjW := r.right - r.left
	adjH := r.bottom - r.top
	cur, ok := getWindowRect(w.hwnd)
	if !ok {
		return
	}
	moveWindow(w.hwnd, cur.left, cur.top, adjW, adjH, true)
	w.w = cw
	w.h = ch
}

func (a *App) resizeClient(w, h int32) {
	a.resizeWindowClient(a.window, w, h)
}

// LayoutChildren repositions widgets in the active window that were created
// without explicit coordinates. It runs on creation and on resize.
func (a *App) LayoutChildren() {
	a.layoutWindow(a.window)
}

func (a *App) layoutWindow(win *Widget) {
	if win == nil || win.hwnd == 0 {
		return
	}
	cr, ok := getClientRect(win.hwnd)
	if !ok || cr.right <= 0 || cr.bottom <= 0 {
		return
	}
	a.layoutContainer(win, cr.right, cr.bottom)
}

func isContainerKind(kind string) bool {
	switch kind {
	case "groupbox", "panel", "scroll", "grid", "toolbar", "splitter", "tabs", "tabpage":
		return true
	}
	return false
}

func (a *App) containerMargin(kind string) int32 {
	switch kind {
	case "groupbox":
		return a.px(12)
	case "panel", "scroll":
		return a.px(8)
	case "grid":
		return a.px(4)
	}
	return a.margin
}

func (a *App) containerStart(kind string) int32 {
	if kind == "groupbox" {
		return a.px(26)
	}
	return a.px(8)
}

// layoutContainer lays out one container recursively. Coordinates are relative
// to the container's client area; hidden children take no space.
func (a *App) layoutContainer(c *Widget, width, height int32) {
	if c == nil || c.hwnd == 0 || width <= 0 || height <= 0 {
		return
	}
	c.w, c.h = width, height
	switch c.kind {
	case "grid":
		a.layoutGrid(c, width, height)
		return
	case "toolbar":
		a.layoutToolbar(c, width, height)
		return
	case "splitter":
		a.layoutSplitter(c, width, height)
		return
	case "tabs":
		a.layoutTabs(c, width, height)
		return
	case "tabpage":
		// A tab page lays out like an ordinary panel.
	}
	margin := a.containerMargin(c.kind)
	cursor := a.containerStart(c.kind)
	var statusbar *Widget
	for _, w := range c.children {
		if w == nil || w.hwnd == 0 || w.hidden {
			continue
		}
		if w.kind == "statusbar" {
			statusbar = w
			continue
		}
		x, y := w.x, w.y
		if w.autoX {
			x = margin
		}
		if w.autoY {
			y = cursor
		}
		ww := w.w
		if w.autoW {
			if w.fullWidth {
				ww = width - 2*margin
			} else {
				ww = w.defW
			}
			if ww < a.px(40) {
				ww = a.px(40)
			}
		}
		hh := w.ctrlH
		if hh == 0 {
			hh = w.h
		}
		anchor := strings.ToLower(w.anchor)
		hasLeft := strings.Contains(anchor, "left")
		hasRight := strings.Contains(anchor, "right")
		hasTop := strings.Contains(anchor, "top")
		hasBottom := strings.Contains(anchor, "bottom")
		if hasRight && !hasLeft {
			x = width - margin - ww
		}
		if hasLeft && hasRight {
			x = margin
			ww = width - 2*margin
		}
		if hasBottom && !hasTop {
			y = height - margin - hh
		}
		if hasTop && hasBottom {
			y = margin
			hh = height - 2*margin
		}
		screenY := y
		if c.kind == "scroll" {
			screenY -= c.scrollY
		}
		moveWindow(w.hwnd, x, screenY, ww, hh, true)
		w.x, w.y, w.w = x, y, ww
		if hh > 0 {
			w.h = hh
		}
		cursor = y + w.displayH + w.gap
		if isContainerKind(w.kind) {
			a.layoutContainer(w, ww, hh)
		}
	}
	if statusbar != nil {
		ww := width - 2*margin
		hh := statusbar.h
		if hh <= 0 {
			hh = a.px(28)
		}
		y := height - margin - hh
		moveWindow(statusbar.hwnd, margin, y, ww, hh, true)
		statusbar.x, statusbar.y, statusbar.w, statusbar.h = margin, y, ww, hh
		statusbar.setStatusParts(statusbar.statusParts)
	}
	if c.kind == "scroll" {
		c.scrollH = cursor + margin
		a.applyScroll(c)
	}
}

func (a *App) layoutGrid(c *Widget, width, height int32) {
	margin := a.px(4)
	gap := a.px(4)
	cols := c.gridCols
	if cols < 1 {
		cols = 2
	}
	cellW := (width - 2*margin - int32(cols-1)*gap) / int32(cols)
	if cellW < a.px(40) {
		cellW = a.px(40)
	}
	x, y := margin, margin
	rowH := int32(0)
	col := 0
	for _, w := range c.children {
		if w == nil || w.hwnd == 0 || w.hidden {
			continue
		}
		ww := w.w
		if w.autoW || w.fullWidth {
			ww = cellW
		}
		hh := w.ctrlH
		if hh == 0 {
			hh = w.h
		}
		moveWindow(w.hwnd, x, y, ww, hh, true)
		w.x, w.y, w.w = x, y, ww
		if hh > 0 {
			w.h = hh
		}
		if isContainerKind(w.kind) {
			a.layoutContainer(w, ww, hh)
		}
		if hh > rowH {
			rowH = hh
		}
		col++
		if col >= cols {
			col = 0
			x = margin
			y += rowH + gap
			rowH = 0
		} else {
			x += cellW + gap
		}
	}
}

func (a *App) applyScroll(w *Widget) {
	max := w.scrollH - w.h
	if max < 0 {
		max = 0
	}
	if w.scrollY > max {
		w.scrollY = max
	}
	if w.scrollY < 0 {
		w.scrollY = 0
	}
	pSetScrollRange.Call(uintptr(w.hwnd), sbVert, 0, uintptr(max), 1)
	pSetScrollPos.Call(uintptr(w.hwnd), sbVert, uintptr(w.scrollY), 1)
}

func (a *App) scrollPanel(w *Widget, code uintptr) {
	if w == nil {
		return
	}
	old := w.scrollY
	line := a.px(24)
	switch int(code & 0xffff) {
	case sbLineUp:
		w.scrollY -= line
	case sbLineDown:
		w.scrollY += line
	case sbPageUp:
		w.scrollY -= w.h - line
	case sbPageDown:
		w.scrollY += w.h - line
	case sbThumbTrack:
		w.scrollY = int32(int16(code >> 16))
	case sbTop:
		w.scrollY = 0
	case sbBottom:
		w.scrollY = w.scrollH
	}
	if w.scrollY != old {
		a.layoutContainer(w, w.w, w.h)
	}
}

// layoutGroup is kept for compatibility; layoutContainer handles all nested
// containers, including group boxes.
func (a *App) layoutGroup(g *Widget) {
	if g != nil && g.kind == "groupbox" {
		a.layoutContainer(g, g.w, g.h)
	}
}

// Show displays every top-level window.
func (a *App) Show() {
	for _, w := range a.windows {
		showWindow(w.hwnd, swShowNormal)
		pUpdateWindow.Call(uintptr(w.hwnd))
	}
	if len(a.windows) > 0 {
		target := a.main
		if target == nil {
			target = a.windows[0]
		}
		// Make sure the window comes to the front when a script is launched
		// from a console; this is the one moment where stealing focus is
		// expected.
		pSetForegroundWindow.Call(uintptr(target.hwnd))
	}
}

// Run starts the Win32 message loop. It returns when the last window closes.
func (a *App) Run() error {
	if len(a.windows) == 0 {
		return fmt.Errorf("还没有窗口可以运行")
	}
	if !a.ran {
		a.Show()
	}
	if a.startupShot != "" {
		path := a.startupShot
		a.startupShot = ""
		a.addTimer(1000, false, func() error {
			err := a.Screenshot(path)
			a.Quit()
			return err
		})
	}
	a.drainPendingUI()
	a.ran = true
	a.running = true
	defer func() { a.running = false }()
	var msg msgT
	for {
		r, _, err := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		switch int32(r) {
		case 0:
			return nil
		case -1:
			return fmt.Errorf("消息循环失败: %v", err)
		}
		handled := a.translateAccelerator(&msg)
		if !handled {
			for _, w := range a.windows {
				if r, _, _ := pIsDialogMessageW.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&msg))); r != 0 {
					handled = true
					break
				}
			}
		}
		if handled {
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// Quit asks the message loop to stop.
func (a *App) Quit() {
	a.exitCode = 0
	pPostQuitMessage.Call(0)
}

// HasWindow reports whether the script created a window.
func (a *App) HasWindow() bool { return len(a.windows) > 0 }

// SetStartupShot schedules a screenshot once the message loop starts. This is
// used by `agui run --shot`, including scripts that call run() explicitly.
func (a *App) SetStartupShot(path string) {
	a.startupShot = path
}

// Ran reports whether the message loop has already been started.
func (a *App) Ran() bool { return a.ran }

func (a *App) widgetByHWND(h syscall.Handle) *Widget {
	return a.widgets[h]
}

func (a *App) addWidget(w *Widget) {
	a.widgets[w.hwnd] = w
	a.byID[w.id] = w
}

func (a *App) drawOwnerButton(lparam uintptr) bool {
	dis := (*drawItemStruct)(unsafe.Pointer(lparam))
	if dis.ctlType != odtButton {
		return false
	}
	w := a.widgetByHWND(dis.hwndItem)
	if w == nil || !w.ownerDraw {
		return false
	}
	w.drawButton(dis)
	return true
}

func (a *App) removeWindow(w *Widget) {
	if w == nil {
		return
	}
	delete(a.widgets, w.hwnd)
	for i, other := range a.windows {
		if other == w {
			a.windows = append(a.windows[:i], a.windows[i+1:]...)
			break
		}
	}
	if a.main == w {
		a.main = nil
		if len(a.windows) > 0 {
			a.main = a.windows[0]
		}
	}
	if a.window == w {
		a.activate(nil)
		if len(a.windows) > 0 {
			a.activate(a.windows[len(a.windows)-1])
		}
	}
	if len(a.windows) == 0 {
		pPostQuitMessage.Call(uintptr(a.exitCode))
	}
}

func (a *App) reportError(err error) {
	if err == nil {
		return
	}
	text := a.interp.FormatError(err)
	fmt.Fprintln(os.Stderr, text)
	owner := a.hwnd
	if owner == 0 && len(a.windows) > 0 {
		owner = a.windows[0].hwnd
	}
	if owner != 0 {
		messageBox(owner, text, "Agui 运行错误", mbOK|mbIconError)
	}
}

func (a *App) addTimer(ms int, repeat bool, fn func() error) uintptr {
	if ms < 1 {
		ms = 1
	}
	if ms > 24*60*60*1000 {
		ms = 24 * 60 * 60 * 1000
	}
	win := a.EnsureWindow()
	id := a.nextTimer
	a.nextTimer++
	if !setTimer(win.hwnd, id, uint32(ms)) {
		return 0
	}
	a.timers[id] = &timerEntry{id: id, hwnd: win.hwnd, repeat: repeat, fn: fn}
	return id
}

func (a *App) cancelTimer(id uintptr) {
	if e, ok := a.timers[id]; ok {
		delete(a.timers, id)
		killTimer(e.hwnd, e.id)
	}
}

// Post schedules fn on the UI thread. Background tasks use it to update the
// interface safely: Win32 windows must only be touched from the UI thread.
func (a *App) Post(fn func()) {
	if fn == nil {
		return
	}
	a.asyncMu.Lock()
	if a.stopped {
		a.asyncMu.Unlock()
		return
	}
	a.nextAsync++
	id := a.nextAsync
	a.asyncFns[id] = fn
	hwnd := a.hwnd
	if hwnd == 0 && a.main != nil {
		hwnd = a.main.hwnd
	}
	a.asyncMu.Unlock()
	if hwnd == 0 {
		// Console-only scripts never start a message loop; run the callback
		// immediately instead of queuing it forever.
		a.asyncMu.Lock()
		delete(a.asyncFns, id)
		a.asyncMu.Unlock()
		fn()
		return
	}
	if r, _, _ := pPostMessageW.Call(uintptr(hwnd), wmAppAsync, id, 0); r == 0 {
		a.asyncMu.Lock()
		delete(a.asyncFns, id)
		a.asyncMu.Unlock()
		fn()
	}
}

func (a *App) addTask(t *lang.Task) {
	a.asyncMu.Lock()
	a.tasks = append(a.tasks, t)
	a.asyncMu.Unlock()
}

// shutdownTasks cancels all background work and waits briefly for it to stop.
func (a *App) shutdownTasks() {
	a.asyncMu.Lock()
	a.stopped = true
	tasks := append([]*lang.Task(nil), a.tasks...)
	a.asyncMu.Unlock()
	for _, t := range tasks {
		t.Cancel()
	}
	deadline := time.Now().Add(2 * time.Second)
	for _, t := range tasks {
		remain := time.Until(deadline)
		if remain <= 0 {
			break
		}
		t.Wait(remain)
	}
}

func (a *App) drainPendingUI() {
	a.asyncMu.Lock()
	fns := a.pendingUI
	a.pendingUI = nil
	a.asyncMu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

func (a *App) runAsyncCallback(id uintptr) {
	a.asyncMu.Lock()
	fn := a.asyncFns[id]
	delete(a.asyncFns, id)
	a.asyncMu.Unlock()
	if fn != nil {
		fn()
	}
}

func mainWndProc(hwnd, msg, wparam, lparam uintptr) (ret uintptr) {
	a := currentApp
	h := syscall.Handle(hwnd)
	defer func() {
		if r := recover(); r != nil {
			if a != nil {
				a.reportError(fmt.Errorf("界面回调内部错误: %v", r))
			} else {
				fmt.Fprintf(os.Stderr, "agui: 界面回调内部错误: %v\n", r)
			}
			ret = defWindowProc(h, uint32(msg), wparam, lparam)
		}
	}()
	if a == nil {
		return defWindowProc(h, uint32(msg), wparam, lparam)
	}
	if a.taskbarMsg != 0 && uint32(msg) == a.taskbarMsg {
		a.onTaskbarCreated()
		return 0
	}
	switch uint32(msg) {
	case wmTray:
		a.handleTray(lparam)
		return 0

	case 0x0112: // WM_SYSCOMMAND
		if uint32(wparam&0xFFF0) == scMinimize && a.minimizeToTray {
			pShowWindow.Call(uintptr(h), 0) // SW_HIDE
			return 0
		}

	case wmCommand:
		id := int(wparam & 0xFFFF)
		code := int((wparam >> 16) & 0xFFFF)
		if mi, ok := a.menuItems[id]; ok {
			mi.emit("click")
			return 0
		}
		w := a.byID[id]
		if w != nil {
			w.command(code)
		}
		return 0

	case wmDropFiles:
		if w := a.windowByHWND(h); w != nil {
			w.handleDrop(syscall.Handle(wparam))
		}
		return 0

	case wmGetObject:
		if lr := handleGetObject(hwnd, uintptr(wparam)); lr != 0 {
			return lr
		}
	case wmDrawItem:
		if a.drawOwnerButton(lparam) {
			return 1
		}
		return 0

	case wmNotify:
		hdr := (*nmhdr)(unsafe.Pointer(lparam))
		if w := a.widgetByHWND(hdr.hwndFrom); w != nil {
			switch w.kind {
			case "table":
				w.tableNotify(hdr.code, lparam)
			case "tree":
				w.treeNotify(hdr.code, lparam)
			case "date", "time":
				if hdr.code == dtnDateTimeChange {
					w.syncDateFromControl()
					w.emit("change")
				}
			case "richedit":
				w.richNotify(hdr.code, lparam)
			}
		}
		return 0

	case wmHScroll:
		w := a.widgetByHWND(syscall.Handle(lparam))
		if w != nil && w.kind == "slider" {
			w.emit("change")
		}
		return 0

	case wmAppAsync:
		a.runAsyncCallback(uintptr(wparam))
		return 0

	case wmTimer:
		id := uintptr(wparam)
		if e, ok := a.timers[id]; ok {
			if !e.repeat {
				delete(a.timers, id)
				killTimer(e.hwnd, e.id)
			}
			if err := e.fn(); err != nil {
				a.reportError(err)
			}
		}
		return 0

	case wmKeyDown, wmSysKeyDown:
		a.handleKeyMessage(h, uint32(wparam), true)
		return 0

	case wmKeyUp, wmSysKeyUp:
		a.handleKeyMessage(h, uint32(wparam), false)
		return 0

	case wmClose:
		if win := a.windowByHWND(h); win != nil {
			win.emit("close")
		}
		pDestroyWindow.Call(uintptr(h))
		return 0

	case wmDestroy:
		a.removeWindow(a.windowByHWND(h))
		return 0

	case wmDpiChanged:
		if win := a.windowByHWND(h); win != nil {
			dpi := int32(wparam & 0xFFFF)
			if r := (*rect)(unsafe.Pointer(lparam)); r != nil {
				pSetWindowPos.Call(uintptr(h), 0, uintptr(r.left), uintptr(r.top),
					uintptr(r.right-r.left), uintptr(r.bottom-r.top),
					swpNoZOrder|swpNoActivate)
			}
			a.applyDPI(dpi)
			a.layoutWindow(win)
		}
		return 0

	case wmSize:
		if win := a.windowByHWND(h); win != nil && wparam != 1 { // ignore SIZE_MINIMIZED
			cw := int32(lparam & 0xFFFF)
			ch := int32((lparam >> 16) & 0xFFFF)
			if cw > 0 && ch > 0 {
				win.w = cw
				win.h = ch
				a.layoutWindow(win)
				win.emit("resize")
			}
		}
		return 0

	case wmGetMinMaxInfo:
		info := (*minMaxInfo)(unsafe.Pointer(lparam))
		info.minTrackSize.x = a.px(480)
		info.minTrackSize.y = a.px(360)
		return 0

	case wmEraseBkgnd:
		var cr rect
		brush := a.bgBrush
		if win := a.windowByHWND(h); win != nil && win.bgBrush != 0 {
			brush = win.bgBrush
		}
		if ok, _, _ := pGetClientRect.Call(uintptr(h), uintptr(unsafe.Pointer(&cr))); ok != 0 {
			fillRect(syscall.Handle(wparam), &cr, brush)
		}
		return 1

	case wmCtlColorStatic, wmCtlColorBtn:
		hdc := syscall.Handle(wparam)
		setBkMode(hdc, 1) // TRANSPARENT
		fg, bg := uint32(0x1E1E1E), a.bgColor
		if w := a.widgetByHWND(syscall.Handle(lparam)); w != nil {
			if w.hasFg {
				fg = w.fg
			}
			if w.hasBg {
				bg = w.bg
			}
			if w.parent != nil && !w.hasBg {
				bg = w.parent.bgColor
			}
		}
		setTextColor(hdc, fg)
		setBkColor(hdc, bg)
		return uintptr(a.brushFor(bg))

	case wmCtlColorEdit, wmCtlColorListBox:
		hdc := syscall.Handle(wparam)
		fg, bg := uint32(0x1E1E1E), uint32(0xFFFFFF)
		if w := a.widgetByHWND(syscall.Handle(lparam)); w != nil {
			if w.hasFg {
				fg = w.fg
			}
			if w.hasBg {
				bg = w.bg
			}
		}
		setTextColor(hdc, fg)
		setBkColor(hdc, bg)
		return uintptr(a.brushFor(bg))
	}
	return defWindowProc(h, uint32(msg), wparam, lparam)
}

func (w *Widget) command(code int) {
	if w.tabHeader && w.tabOwner != nil && code == bnClicked {
		w.tabOwner.activateTab(w.tabIndex)
		w.emit("click")
		return
	}
	switch w.kind {
	case "button":
		if code == bnClicked {
			w.emit("click")
		}
	case "checkbox":
		if code == bnClicked {
			w.emit("click")
			w.emit("change")
		}
	case "radio":
		if code == bnClicked {
			w.emit("click")
			w.emit("change")
		}
	case "statusbar":
		switch code {
		case stnClicked:
			w.emit("click")
		case stnDblClk:
			w.emit("dblclick")
		}
	case "label":
		switch code {
		case stnClicked:
			w.emit("click")
		case stnDblClk:
			w.emit("dblclick")
		}
	case "edit":
		if code == enChange {
			w.emit("change")
		}
	case "listbox":
		switch code {
		case lbnSelChange:
			w.emit("select")
		case lbnDblClk:
			w.emit("dblclick")
		}
	case "combo":
		switch code {
		case cbnSelChange:
			w.emit("select")
			w.emit("change")
		case cbnDblClk:
			w.emit("dblclick")
		case cbnEditChange:
			w.emit("change")
		}
	}
}

func (w *Widget) handleDrop(hdrop syscall.Handle) {
	if hdrop == 0 {
		return
	}
	defer pDragFinish.Call(uintptr(hdrop))
	n, _, _ := pDragQueryFileW.Call(uintptr(hdrop), ^uintptr(0), 0, 0)
	w.dropped = w.dropped[:0]
	for i := uintptr(0); i < n; i++ {
		buf := make([]uint16, 32768)
		got, _, _ := pDragQueryFileW.Call(uintptr(hdrop), i, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if got > 0 {
			w.dropped = append(w.dropped, syscall.UTF16ToString(buf[:got]))
		}
	}
	w.emit("drop")
}

func (w *Widget) emit(event string) {
	if os.Getenv("AGUI_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[debug] 事件 %s.%s\n", w.kind, event)
	}
	for _, fn := range w.handlers[event] {
		if err := fn(); err != nil {
			w.app.reportError(err)
		}
	}
}

// parseColor accepts "#RRGGBB", "#RGB" or a few common color names.
func parseColor(s string) (uint32, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasPrefix(s, "#") {
		hex := s[1:]
		if !validHex(hex) {
			return 0, false
		}
		var r, g, b uint32
		switch len(hex) {
		case 3:
			r = hexNibble(hex[0]) * 17
			g = hexNibble(hex[1]) * 17
			b = hexNibble(hex[2]) * 17
		case 6:
			r = hexNibble(hex[0])*16 + hexNibble(hex[1])
			g = hexNibble(hex[2])*16 + hexNibble(hex[3])
			b = hexNibble(hex[4])*16 + hexNibble(hex[5])
		default:
			return 0, false
		}
		return r | g<<8 | b<<16, true
	}
	switch s {
	case "white":
		return 0xFFFFFF, true
	case "black":
		return 0x000000, true
	case "red":
		return 0x0000FF, true
	case "green":
		return 0x008000, true
	case "blue":
		return 0xFF0000, true
	case "gray", "grey":
		return 0x808080, true
	case "orange":
		return 0x00A5FF, true
	case "yellow":
		return 0x00FFFF, true
	case "transparent":
		return 0, false
	}
	return 0, false
}

func colorString(c uint32) string {
	return fmt.Sprintf("#%02X%02X%02X", c&0xFF, (c>>8)&0xFF, (c>>16)&0xFF)
}

func parseColorOr(s string, def uint32) uint32 {
	if c, ok := parseColor(s); ok {
		return c
	}
	return def
}

func validHex(s string) bool {
	if len(s) != 3 && len(s) != 6 {
		return false
	}
	for i := 0; i < len(s); i++ {
		b := s[i]
		if !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')) {
			return false
		}
	}
	return true
}

func hexNibble(b byte) uint32 {
	switch {
	case b >= '0' && b <= '9':
		return uint32(b - '0')
	case b >= 'a' && b <= 'f':
		return uint32(b-'a') + 10
	case b >= 'A' && b <= 'F':
		return uint32(b-'A') + 10
	}
	return 0
}
