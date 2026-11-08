package gui

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"agui/internal/lang"
)

// Widget is the Agui-visible wrapper around a Win32 control.
type Widget struct {
	app  *App
	hwnd syscall.Handle
	kind string
	id   int

	// Top-level window state (only used when kind == "window").
	parent      *Widget
	children    []*Widget
	layoutY     int32
	windowStyle uint32
	bgColor     uint32
	bgBrush     syscall.Handle
	menuBar     syscall.Handle
	menus       []*Menu
	iconBig     syscall.Handle
	iconSmall   syscall.Handle
	dropped     []string

	handlers map[string][]func() error

	x, y, w, h int32
	displayH   int32
	ctrlH      int32
	autoX      bool
	autoY      bool
	autoW      bool
	fullWidth  bool
	defW       int32
	gap        int32

	// Container/layout state.
	hidden   bool
	anchor   string
	gridCols int
	scrollY  int32
	scrollH  int32

	// Tabs/splitter/toolbar containers.
	paneA, paneB  *Widget
	splitRatio    float64
	splitVertical bool
	splitDrag     bool
	tabPages      []*Widget
	tabButtons    []*Widget
	activeTab     int
	tabHeader     bool
	tabOwner      *Widget
	tabIndex      int

	groupOldProc uintptr

	title  string
	bgSpec string
	font   *lang.Font
	hasFg  bool
	hasBg  bool
	fg     uint32
	bg     uint32

	items []string
	minV  float64
	maxV  float64
	cols  int

	headers     []string
	editable    bool
	virtual     bool
	virtualRows [][]string
	virtualBuf  []uint16
	editHwnd    syscall.Handle
	editOldProc uintptr
	editRow     int32
	editCol     int32
	sortCol     int
	sortAsc     bool

	gdiImage  uintptr
	imgW      int32
	imgH      int32
	imgPath   string
	imgFit    bool
	imgLoaded bool

	ownerDraw bool

	treeLabels map[syscall.Handle]string
	treePaths  map[string]syscall.Handle

	canvasOps   []canvasOp
	canvasBatch bool
	mouseX      float64
	mouseY      float64
	mouseDown   bool
	mouseButton string
	key         string

	// Fullscreen/borderless restore state.
	fullscreen    bool
	windowedStyle uint32
	windowedRect  rect

	// Extra controls (statusbar/date/time/tooltip).
	statusParts    []string
	dateValue      time.Time
	tooltip        string
	tooltipInfo    toolInfoW
	tooltipTracked bool
	richLinks      []richLink
	opacity        float64
}

// Kind implements lang.Widget.
func (w *Widget) Kind() string { return w.kind }

var validEvents = map[string]map[string]bool{
	"window":    {"close": true, "resize": true, "drop": true, "keydown": true, "keyup": true},
	"label":     {"click": true, "dblclick": true},
	"button":    {"click": true},
	"checkbox":  {"click": true, "change": true},
	"radio":     {"click": true, "change": true},
	"statusbar": {"click": true, "dblclick": true},
	"date":      {"change": true},
	"time":      {"change": true},
	"edit":      {"change": true},
	"richedit":  {"change": true, "click": true},
	"listbox":   {"select": true, "dblclick": true},
	"combo":     {"select": true, "change": true, "dblclick": true},
	"slider":    {"change": true},
	"progress":  {},
	"image":     {"click": true, "dblclick": true},
	"table":     {"select": true, "click": true, "dblclick": true, "edit": true},
	"tree":      {"select": true, "expand": true, "dblclick": true},
	"canvas":    {"click": true, "mousedown": true, "mouseup": true, "mousemove": true},
}

// Listen implements lang.Widget.
func (w *Widget) Listen(event string, fn func() error) error {
	allowed, ok := validEvents[w.kind]
	if !ok {
		return fmt.Errorf("%s 控件不支持事件", w.kind)
	}
	if !allowed[event] {
		var names []string
		for k := range allowed {
			names = append(names, k)
		}
		if len(names) == 0 {
			return fmt.Errorf("%s 控件不支持任何事件", w.kind)
		}
		return fmt.Errorf("%s 控件不支持事件 %q（可用：%s）", w.kind, event, strings.Join(names, "、"))
	}
	if w.handlers == nil {
		w.handlers = map[string][]func() error{}
	}
	w.handlers[event] = append(w.handlers[event], fn)
	return nil
}

func (a *App) addChild(kind, class, text string, style uint32, full bool, defW, defH int32, xv, yv, wv, hv *int32) (*Widget, error) {
	win := a.EnsureWindow()
	container := win
	if a.pendingParent != nil {
		container = a.pendingParent
		a.pendingParent = nil
	}
	cr, _ := getClientRect(container.hwnd)
	margin := a.containerMargin(container.kind)
	gap := a.gap

	autoX := xv == nil
	autoY := yv == nil
	autoW := wv == nil
	fullWidth := full
	if kind == "label" && xv != nil {
		fullWidth = false
	}
	x := margin
	if !autoX {
		x = *xv
	}
	y := container.layoutY
	if y == 0 {
		y = a.containerStart(container.kind)
	}
	if !autoY {
		y = *yv
	}
	width := defW
	if !autoW {
		width = *wv
	} else if fullWidth {
		width = cr.right - x - margin
		if width < a.px(40) {
			width = a.px(40)
		}
	}
	height := defH
	if hv != nil {
		height = *hv
	}
	displayH := height
	ctrlH := height
	if kind == "combo" {
		ctrlH = a.px(220)
		if hv != nil {
			ctrlH = height
		}
		displayH = a.px(30)
	}

	id := a.nextID
	a.nextID++
	hwnd := createWindow(0, class, text, wsChild|wsVisible|style, x, y, width, ctrlH, container.hwnd, id)
	if hwnd == 0 {
		return nil, fmt.Errorf("创建 %s 控件失败", kind)
	}
	w := &Widget{
		app:       a,
		hwnd:      hwnd,
		kind:      kind,
		id:        id,
		parent:    container,
		x:         x,
		y:         y,
		w:         width,
		h:         height,
		displayH:  displayH,
		ctrlH:     ctrlH,
		autoX:     autoX,
		autoY:     autoY,
		autoW:     autoW,
		fullWidth: fullWidth,
		defW:      defW,
		gap:       gap,
		handlers:  map[string][]func() error{},
		minV:      0,
		maxV:      100,
	}
	a.addWidget(w)
	container.children = append(container.children, w)
	container.layoutY = y + displayH + gap
	sendMessage(hwnd, wmSetFont, uintptr(a.hFont), 1)
	if a.darkTheme {
		switch kind {
		case "listbox", "tree", "table", "edit", "richedit", "combo":
			setWindowTheme(hwnd, "DarkMode_Explorer", "")
		}
	}
	return w, nil
}

func (w *Widget) topWindow() *Widget {
	for w != nil && w.kind != "window" {
		w = w.parent
	}
	return w
}

func (a *App) createLabel(text string, x, y, w, h *int32) (*Widget, error) {
	style := uint32(ssLeft | ssNoPrefix | ssNotify)
	return a.addChild("label", "STATIC", text, style, true, a.px(300), a.px(24), x, y, w, h)
}

func (a *App) createGroupBox(text string, x, y, w, h *int32) (*Widget, error) {
	style := uint32(bsGroupBox)
	widget, err := a.addChild("groupbox", "BUTTON", text, style, true, 0, a.px(140), x, y, w, h)
	if err != nil {
		return nil, err
	}
	widget.groupOldProc = setWindowLong(widget.hwnd, gwlWndProc, groupboxProcCallback)
	return widget, nil
}

// createPanel creates a layout container. kind is "panel", "scroll" or "grid";
// groupbox is created by createGroupBox because it uses the BUTTON class.
func (a *App) createPanel(kind string, cols int, x, y, w, h *int32) (*Widget, error) {
	style := uint32(wsClipChildren)
	if kind == "scroll" {
		style |= wsVScroll
	}
	defH := a.px(140)
	switch kind {
	case "scroll":
		defH = a.px(180)
	case "grid":
		defH = a.px(160)
	}
	widget, err := a.addChild(kind, panelClassName, "", style, true, 0, defH, x, y, w, h)
	if err != nil {
		return nil, err
	}
	if cols < 1 {
		cols = 2
	}
	widget.gridCols = cols
	return widget, nil
}

func (a *App) createButton(text string, x, y, w, h *int32) (*Widget, error) {
	style := uint32(bsPushButton | wsTabStop)
	return a.addChild("button", "BUTTON", text, style, false, a.px(150), a.px(34), x, y, w, h)
}

func (a *App) createCheckbox(text string, checked bool, x, y, w, h *int32) (*Widget, error) {
	style := uint32(bsAutoCheckBox | wsTabStop)
	widget, err := a.addChild("checkbox", "BUTTON", text, style, true, 0, a.px(28), x, y, w, h)
	if err != nil {
		return nil, err
	}
	if checked {
		sendMessage(widget.hwnd, bmSetCheck, bstChecked, 0)
	}
	return widget, nil
}

func (a *App) createEdit(text string, multiline, readonly, password bool, x, y, w, h *int32) (*Widget, error) {
	style := uint32(wsBorder | esAutoHScroll | wsTabStop)
	defH := a.px(30)
	if multiline {
		style |= esMultiLine | esAutoVScroll | wsVScroll
		defH = a.px(120)
	}
	if readonly {
		style |= esReadOnly
	}
	widget, err := a.addChild("edit", "EDIT", text, style, true, 0, defH, x, y, w, h)
	if err != nil {
		return nil, err
	}
	if password {
		sendMessage(widget.hwnd, emSetPasswordChar, 0x25CF, 0)
	}
	return widget, nil
}

func (a *App) createListbox(items []string, x, y, w, h *int32) (*Widget, error) {
	style := uint32(wsBorder | wsVScroll | lbsNotify | lbsNoIntegralHeight | wsTabStop)
	widget, err := a.addChild("listbox", "LISTBOX", "", style, true, 0, a.px(140), x, y, w, h)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		a.listAdd(widget, item)
	}
	return widget, nil
}

func (a *App) createCombo(items []string, x, y, w, h *int32) (*Widget, error) {
	style := uint32(wsVScroll | cbsDropDownList | wsTabStop)
	widget, err := a.addChild("combo", "COMBOBOX", "", style, false, a.px(260), a.px(30), x, y, w, h)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		a.listAdd(widget, item)
	}
	return widget, nil
}

func (a *App) createProgress(value float64, max float64, x, y, w, h *int32) (*Widget, error) {
	widget, err := a.addChild("progress", "msctls_progress32", "", pbsSmooth, true, 0, a.px(18), x, y, w, h)
	if err != nil {
		return nil, err
	}
	if max <= 0 {
		max = 100
	}
	widget.minV = 0
	widget.maxV = max
	sendMessage(widget.hwnd, pbmSetRange32, 0, uintptr(int32(max)))
	widget.SetProp("value", value)
	return widget, nil
}

func (a *App) createSlider(min, max, value float64, x, y, w, h *int32) (*Widget, error) {
	widget, err := a.addChild("slider", "msctls_trackbar32", "", tbsAutoTicks|wsTabStop, true, 0, a.px(36), x, y, w, h)
	if err != nil {
		return nil, err
	}
	if max <= min {
		max = min + 100
	}
	widget.minV = min
	widget.maxV = max
	widget.applyRange()
	widget.SetProp("value", value)
	sendMessage(widget.hwnd, tbmSetTicFreq, uintptr(int(max-min)/10+1), 0)
	return widget, nil
}

func (a *App) listAdd(w *Widget, item string) int {
	var msg uint32
	if w.kind == "combo" {
		msg = cbAddString
	} else {
		msg = lbAddString
	}
	r := sendMessage(w.hwnd, msg, 0, uintptr(unsafe.Pointer(u16(item))))
	w.items = append(w.items, item)
	return int(int32(r))
}

func (w *Widget) applyRange() {
	min := int(w.minV)
	max := int(w.maxV)
	if min < 0 {
		min = 0
	}
	if max > 65535 {
		max = 65535
	}
	if max < min {
		max = min
	}
	if w.kind == "slider" {
		lparam := uintptr(uint32(min) | uint32(max)<<16)
		sendMessage(w.hwnd, tbmSetRange, 1, lparam)
	} else if w.kind == "progress" {
		sendMessage(w.hwnd, pbmSetRange32, 0, uintptr(int32(max)))
	}
}

// GetProp implements lang.Widget.
func (w *Widget) GetProp(name string) (lang.Value, bool) {
	switch name {
	case "kind":
		return w.kind, true
	case "x":
		return w.getX(), true
	case "y":
		return w.getY(), true
	case "w", "width":
		return w.getW(), true
	case "h", "height":
		return w.getH(), true
	case "enabled":
		return isWindowEnabled(w.hwnd), true
	case "visible":
		return isWindowVisible(w.hwnd), true
	case "anchor":
		return w.anchor, true
	case "scroll_y":
		return w.app.logical(w.scrollY), true
	case "scroll_height":
		return w.app.logical(w.scrollH), true
	case "tooltip":
		return w.tooltip, true
	case "tooltip_visible":
		return w.tooltipVisible(), true
	case "tooltip_tools":
		if w.app.tooltipHwnd == 0 {
			return float64(0), true
		}
		return float64(sendMessage(w.app.tooltipHwnd, ttmGetToolCount, 0, 0)), true
	}
	switch w.kind {
	case "window":
		switch name {
		case "title", "text":
			return w.title, true
		case "bg":
			if w.bgSpec != "" {
				return w.bgSpec, true
			}
			return "#F7F8FA", true
		case "dropped", "dropped_files":
			out := &lang.List{}
			for _, p := range w.dropped {
				out.Items = append(out.Items, p)
			}
			return out, true
		case "state":
			return w.windowState(), true
		case "topmost":
			return w.isTopmost(), true
		case "opacity":
			return w.opacity, true
		case "key":
			return w.key, true
		}
	case "label", "button", "checkbox", "radio", "edit", "richedit":
		if name == "text" {
			return getWindowText(w.hwnd), true
		}
		if name == "checked" && (w.kind == "checkbox" || w.kind == "radio") {
			return sendMessage(w.hwnd, bmGetCheck, 0, 0) == bstChecked, true
		}
		if w.kind == "richedit" {
			switch name {
			case "selection_start":
				start, _ := w.richSelection()
				return float64(start), true
			case "selection_end":
				_, end := w.richSelection()
				return float64(end), true
			case "selected_text":
				return w.richSelectedText(), true
			case "link_count":
				return float64(len(w.richLinks)), true
			}
		}
	case "statusbar":
		switch name {
		case "text":
			return w.title, true
		case "parts":
			return w.statusPartsValue(), true
		}
	case "date", "time":
		switch name {
		case "text", "value":
			return w.dateText(), true
		}
	case "listbox", "combo":
		switch name {
		case "items":
			out := &lang.List{Items: make([]lang.Value, len(w.items))}
			for i, item := range w.items {
				out.Items[i] = item
			}
			return out, true
		case "selected":
			return float64(w.selectedIndex()), true
		case "count":
			return float64(len(w.items)), true
		case "text":
			idx := w.selectedIndex()
			if idx < 0 {
				return "", true
			}
			return w.itemText(idx), true
		}
	case "progress", "slider":
		switch name {
		case "value":
			if w.kind == "progress" {
				return float64(sendMessage(w.hwnd, pbmGetPos, 0, 0)), true
			}
			return float64(int32(sendMessage(w.hwnd, tbmGetPos, 0, 0))), true
		case "min":
			return w.minV, true
		case "max":
			return w.maxV, true
		}
	case "image":
		switch name {
		case "src", "path":
			return w.imgPath, true
		case "fit":
			return w.imgFit, true
		case "width", "height":
			if name == "width" {
				return float64(w.imgW), true
			}
			return float64(w.imgH), true
		}
	case "tabs":
		switch name {
		case "active", "index":
			return float64(w.activeTab), true
		case "page_count", "count":
			return float64(len(w.tabPages)), true
		}
	case "splitter":
		switch name {
		case "ratio":
			return w.splitRatio, true
		case "vertical":
			return w.splitVertical, true
		}
	case "tree":
		switch name {
		case "selected":
			return w.treeSelected(), true
		case "count":
			return float64(w.treeCount()), true
		}
	case "canvas":
		switch name {
		case "mouse_x":
			return w.mouseX, true
		case "mouse_y":
			return w.mouseY, true
		case "mouse_down":
			return w.mouseDown, true
		case "mouse_button":
			return w.mouseButton, true
		case "count":
			return float64(len(w.canvasOps)), true
		}
	case "table":
		switch name {
		case "selected":
			return float64(w.tableSelected()), true
		case "row_count":
			return float64(sendMessage(w.hwnd, lvmGetItemCount, 0, 0)), true
		case "column_count":
			return float64(w.cols), true
		case "rows":
			return w.tableRows(), true
		case "headers":
			return w.tableHeaders(), true
		case "editable":
			return w.editable, true
		case "sort_col":
			return float64(w.sortCol), true
		case "sort_asc":
			return w.sortAsc, true
		case "editing":
			return w.editHwnd != 0, true
		case "edit_text":
			if w.editHwnd == 0 {
				return "", true
			}
			return getWindowText(w.editHwnd), true
		}
	}
	if name == "font" && w.font != nil {
		return w.font, true
	}
	return nil, false
}

func (w *Widget) getX() float64 {
	if w.kind == "window" {
		r, _ := getWindowRect(w.hwnd)
		return w.app.logical(r.left)
	}
	return w.app.logical(w.x)
}

func (w *Widget) getY() float64 {
	if w.kind == "window" {
		r, _ := getWindowRect(w.hwnd)
		return w.app.logical(r.top)
	}
	return w.app.logical(w.y)
}

func (w *Widget) getW() float64 {
	if w.kind == "window" {
		r, _ := getClientRect(w.hwnd)
		return w.app.logical(r.right)
	}
	return w.app.logical(w.w)
}

func (w *Widget) getH() float64 {
	if w.kind == "window" {
		r, _ := getClientRect(w.hwnd)
		return w.app.logical(r.bottom)
	}
	if w.kind == "combo" {
		return w.app.logical(w.displayH)
	}
	return w.app.logical(w.h)
}

func (w *Widget) selectedIndex() int {
	var r uintptr
	if w.kind == "combo" {
		r = sendMessage(w.hwnd, cbGetCurSel, 0, 0)
	} else {
		r = sendMessage(w.hwnd, lbGetCurSel, 0, 0)
	}
	idx := int(int32(r))
	if int32(r) < 0 {
		return -1
	}
	return idx
}

func (w *Widget) itemText(idx int) string {
	if idx < 0 || idx >= len(w.items) {
		return ""
	}
	var msg uint32
	if w.kind == "combo" {
		msg = cbGetLBText
	} else {
		msg = lbGetText
	}
	buf := make([]uint16, 1024)
	sendMessage(w.hwnd, msg, uintptr(idx), uintptr(unsafe.Pointer(&buf[0])))
	return syscall.UTF16ToString(buf)
}

func (w *Widget) setGeometry(prop string, val float64) error {
	v := w.app.px(val)
	if w.kind == "window" {
		switch prop {
		case "x", "y":
			r, _ := getWindowRect(w.hwnd)
			x, y := r.left, r.top
			if prop == "x" {
				x = v
			} else {
				y = v
			}
			moveWindow(w.hwnd, x, y, r.right-r.left, r.bottom-r.top, true)
		case "w", "width":
			cr, _ := getClientRect(w.hwnd)
			w.app.resizeClient(v, cr.bottom)
		case "h", "height":
			cr, _ := getClientRect(w.hwnd)
			w.app.resizeClient(cr.right, v)
		}
		return nil
	}
	x, y, width := w.x, w.y, w.w
	switch prop {
	case "x":
		x = v
		w.autoX = false
	case "y":
		y = v
		w.autoY = false
	case "w", "width":
		width = v
		w.autoW = false
		if width < 0 {
			width = 0
		}
	case "h", "height":
		w.h = v
		w.displayH = v
		if w.kind != "combo" {
			w.ctrlH = v
		}
	}
	moveWindow(w.hwnd, x, y, width, w.ctrlH, true)
	w.x, w.y, w.w = x, y, width
	return nil
}

// SetProp implements lang.Widget.
func (w *Widget) SetProp(name string, v lang.Value) error {
	switch name {
	case "x", "y", "w", "h", "width", "height":
		f, ok := v.(float64)
		if !ok {
			return fmt.Errorf("%s 必须是数字", name)
		}
		return w.setGeometry(name, f)
	case "enabled":
		enableWindow(w.hwnd, lang.Truthy(v))
		return nil
	case "visible":
		w.hidden = !lang.Truthy(v)
		if lang.Truthy(v) {
			showWindow(w.hwnd, swShow)
		} else {
			showWindow(w.hwnd, swHide)
		}
		if w.parent != nil {
			w.app.layoutContainer(w.parent, w.parent.w, w.parent.h)
		}
		return nil
	case "anchor":
		w.anchor = lang.ToString(v)
		if w.parent != nil {
			w.app.layoutContainer(w.parent, w.parent.w, w.parent.h)
		}
		return nil
	case "scroll_y":
		f, ok := v.(float64)
		if !ok {
			return fmt.Errorf("scroll_y 必须是数字")
		}
		w.scrollY = w.app.px(f)
		w.app.layoutContainer(w, w.w, w.h)
		return nil
	case "font":
		f, ok := v.(*lang.Font)
		if !ok {
			return fmt.Errorf("font 必须是 font() 返回的字体")
		}
		h, ok := f.Handle.(syscall.Handle)
		if !ok || h == 0 {
			return fmt.Errorf("字体句柄无效")
		}
		w.font = f
		sendMessage(w.hwnd, wmSetFont, uintptr(h), 1)
		return nil
	case "fg", "bg":
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("%s 必须是颜色字符串", name)
		}
		c, ok := parseColor(s)
		if !ok {
			return fmt.Errorf("无法识别颜色 %q", s)
		}
		if name == "fg" {
			w.hasFg, w.fg = true, c
		} else {
			w.hasBg, w.bg = true, c
			w.bgColor = c
			if w.kind == "window" {
				w.bgSpec = s
				w.app.setWindowBackground(w, s)
				if w.app.window == w {
					w.app.bgColor = w.bgColor
				}
			} else {
				if w.bgBrush != 0 {
					deleteObject(w.bgBrush)
				}
				w.bgBrush = createBrush(c)
			}
		}
		if w.kind == "button" {
			w.setOwnerDraw()
		}
		if w.kind == "richedit" {
			if name == "bg" {
				sendMessage(w.hwnd, emSetBkColor, 0, uintptr(c))
			} else {
				_ = w.applyRichFormat(map[string]lang.Value{"color": s}, true)
			}
		}
		if w.kind == "table" {
			if name == "bg" {
				w.setTableBgColor(c)
			} else {
				w.setTableTextColor(c)
			}
		}
		invalidateRect(w.hwnd, nil, true)
		return nil
	case "tooltip":
		w.setTooltip(lang.ToString(v))
		return nil
	}

	switch w.kind {
	case "window":
		switch name {
		case "title", "text":
			w.title = lang.ToString(v)
			setWindowText(w.hwnd, w.title)
			return nil
		case "icon":
			return w.setWindowIcon(lang.ToString(v))
		case "center":
			if lang.Truthy(v) {
				w.centerWindow()
			}
			return nil
		case "topmost":
			w.setTopmost(lang.Truthy(v))
			return nil
		case "opacity":
			f, ok := v.(float64)
			if !ok {
				return fmt.Errorf("opacity 必须是 0 到 1 的数字")
			}
			return w.setOpacity(f)
		case "state":
			return w.setWindowState(lang.ToString(v))
		case "flash":
			if lang.Truthy(v) {
				w.flashWindow()
			}
			return nil
		}
	case "image":
		switch name {
		case "src", "path":
			return w.loadImage(lang.ToString(v))
		case "fit":
			w.imgFit = lang.Truthy(v)
			invalidateRect(w.hwnd, nil, true)
			return nil
		}
	case "tabs":
		switch name {
		case "active", "index":
			f, ok := v.(float64)
			if !ok {
				return fmt.Errorf("active 必须是数字")
			}
			w.activateTab(int(f))
			return nil
		}
	case "splitter":
		switch name {
		case "ratio":
			f, ok := v.(float64)
			if !ok {
				return fmt.Errorf("ratio 必须是数字")
			}
			w.splitRatio = f
			if w.splitRatio < 0.1 {
				w.splitRatio = 0.1
			}
			if w.splitRatio > 0.9 {
				w.splitRatio = 0.9
			}
			w.app.layoutSplitter(w, w.w, w.h)
			return nil
		case "vertical":
			w.splitVertical = lang.Truthy(v)
			w.app.layoutSplitter(w, w.w, w.h)
			return nil
		}
	case "tree":
		switch name {
		case "items":
			return w.setTreeItems(v)
		case "selected":
			h := w.treeFind(lang.ToString(v))
			if h != 0 {
				w.selectTreeHandle(h)
			}
			return nil
		}
	case "table":
		switch name {
		case "selected":
			f, ok := v.(float64)
			if !ok {
				return fmt.Errorf("selected 必须是数字下标")
			}
			w.tableSelect(int(f))
			return nil
		case "headers":
			headers, err := toItems(v)
			if err != nil {
				return err
			}
			w.setHeaders(headers)
			return nil
		case "rows":
			rows, err := toRows(v)
			if err != nil {
				return err
			}
			if w.virtual {
				w.virtualRows = append([][]string(nil), rows...)
				sendMessage(w.hwnd, lvmSetItemCount, 0, 0)
				sendMessage(w.hwnd, lvmSetItemCount, uintptr(len(rows)), 0)
				return nil
			}
			sendMessage(w.hwnd, lvmDeleteAllItems, 0, 0)
			for _, row := range rows {
				w.addRow(row)
			}
			return nil
		case "editable":
			w.editable = lang.Truthy(v)
			return nil
		}
	case "statusbar":
		switch name {
		case "text":
			w.title = lang.ToString(v)
			parts := strings.Split(w.title, "\t")
			w.setStatusParts(parts)
			return nil
		case "parts":
			items, err := toItems(v)
			if err != nil {
				return err
			}
			w.setStatusParts(items)
			return nil
		}
	case "date", "time":
		switch name {
		case "text", "value":
			return w.setDateText(lang.ToString(v))
		}
	case "label", "button", "checkbox", "radio", "edit", "richedit":
		switch name {
		case "text":
			setWindowText(w.hwnd, lang.ToString(v))
			return nil
		case "selection":
			if w.kind != "richedit" {
				return fmt.Errorf("selection 只适用于 richedit")
			}
			list, ok := v.(*lang.List)
			if !ok || len(list.Items) != 2 {
				return fmt.Errorf("selection 必须是 [start, end]")
			}
			a, _ := list.Items[0].(float64)
			b, _ := list.Items[1].(float64)
			w.richSelectRange(int32(a), int32(b))
			return nil
		case "checked":
			if w.kind == "checkbox" || w.kind == "radio" {
				val := uintptr(0)
				if lang.Truthy(v) {
					val = bstChecked
				}
				sendMessage(w.hwnd, bmSetCheck, val, 0)
				return nil
			}
		case "readonly":
			if w.kind == "edit" || w.kind == "richedit" {
				val := uintptr(0)
				if lang.Truthy(v) {
					val = 1
				}
				sendMessage(w.hwnd, emSetReadOnly, val, 0)
				return nil
			}
		case "password":
			if w.kind == "edit" {
				ch := uintptr(0)
				if lang.Truthy(v) {
					ch = 0x25CF
				}
				sendMessage(w.hwnd, emSetPasswordChar, ch, 0)
				return nil
			}
		case "bold", "italic", "underline":
			if w.kind == "richedit" {
				return w.applyRichFormat(map[string]lang.Value{name: v}, true)
			}
		case "align":
			if w.kind == "richedit" {
				return w.applyRichPara(map[string]lang.Value{"align": v}, true)
			}
		}
	case "listbox", "combo":
		switch name {
		case "items":
			list, ok := v.(*lang.List)
			if !ok {
				return fmt.Errorf("items 必须是列表")
			}
			msg := uint32(lbResetContent)
			if w.kind == "combo" {
				msg = cbResetContent
			}
			sendMessage(w.hwnd, msg, 0, 0)
			w.items = nil
			for _, item := range list.Items {
				w.app.listAdd(w, lang.ToString(item))
			}
			return nil
		case "selected":
			f, ok := v.(float64)
			if !ok {
				return fmt.Errorf("selected 必须是数字下标")
			}
			idx := int(f)
			var msg uint32
			if w.kind == "combo" {
				msg = cbSetCurSel
			} else {
				msg = lbSetCurSel
			}
			if idx < 0 {
				sendMessage(w.hwnd, msg, ^uintptr(0), 0)
			} else {
				sendMessage(w.hwnd, msg, uintptr(idx), 0)
			}
			return nil
		case "text":
			return fmt.Errorf("%s 的 text 是只读的，请设置 selected", w.kind)
		}
	case "progress", "slider":
		switch name {
		case "value":
			f, ok := v.(float64)
			if !ok {
				return fmt.Errorf("value 必须是数字")
			}
			return w.setValue(f)
		case "max":
			f, ok := v.(float64)
			if !ok {
				return fmt.Errorf("max 必须是数字")
			}
			w.maxV = f
			w.applyRange()
			return nil
		case "min":
			f, ok := v.(float64)
			if !ok {
				return fmt.Errorf("min 必须是数字")
			}
			w.minV = f
			w.applyRange()
			return nil
		}
	}
	return fmt.Errorf("%s 没有属性 %q", w.kind, name)
}

func (w *Widget) setValue(f float64) error {
	if f < w.minV {
		f = w.minV
	}
	if f > w.maxV {
		f = w.maxV
	}
	if w.kind == "progress" {
		sendMessage(w.hwnd, pbmSetPos, uintptr(int32(f)), 0)
	} else {
		sendMessage(w.hwnd, tbmSetPos, 1, uintptr(int32(f)))
	}
	return nil
}

// CallMethod implements lang.Widget.
func (w *Widget) CallMethod(name string, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, bool, error) {
	switch name {
	case "use", "activate":
		if w.kind == "window" {
			w.app.activate(w)
			return nil, true, nil
		}
		return nil, true, fmt.Errorf("只有窗口可以 use")
	case "simulate_drop":
		if w.kind != "window" {
			return nil, true, fmt.Errorf("simulate_drop 只适用于窗口")
		}
		if len(args) == 0 {
			return nil, true, fmt.Errorf("simulate_drop(路径列表) 需要一个参数")
		}
		return nil, true, w.simulateDrop(stringsFromValue(args[0]))
	case "icon":
		if w.kind != "window" {
			return nil, true, fmt.Errorf("icon 只能用于窗口")
		}
		if len(args) < 1 {
			return nil, true, fmt.Errorf("icon(图片路径) 需要一个参数")
		}
		return nil, true, w.setWindowIcon(lang.ToString(args[0]))
	case "topmost":
		if w.kind != "window" {
			return nil, true, fmt.Errorf("topmost 只能用于窗口")
		}
		w.setTopmost(len(args) == 0 || lang.Truthy(args[0]))
		return nil, true, nil
	case "opacity":
		if w.kind != "window" {
			return nil, true, fmt.Errorf("opacity 只能用于窗口")
		}
		v := lang.Pos(args, 0, float64(1))
		f, ok := v.(float64)
		if !ok {
			return nil, true, fmt.Errorf("opacity(0~1) 需要数字")
		}
		return nil, true, w.setOpacity(f)
	case "state":
		if w.kind != "window" {
			return nil, true, fmt.Errorf("state 只能用于窗口")
		}
		if len(args) == 0 {
			return w.windowState(), true, nil
		}
		return nil, true, w.setWindowState(lang.ToString(args[0]))
	case "flash":
		if w.kind != "window" {
			return nil, true, fmt.Errorf("flash 只能用于窗口")
		}
		w.flashWindow()
		return nil, true, nil
	case "tooltip", "set_tooltip":
		w.setTooltip(lang.ToString(lang.Pos(args, 0, "")))
		return nil, true, nil
	case "parts":
		if w.kind != "statusbar" {
			return nil, true, fmt.Errorf("parts 只能用于 statusbar")
		}
		if len(args) == 0 {
			return w.statusPartsValue(), true, nil
		}
		parts, err := toItems(args[0])
		if err != nil {
			return nil, true, err
		}
		w.setStatusParts(parts)
		return nil, true, nil
	case "set_value", "set_date":
		if w.kind != "date" && w.kind != "time" {
			return nil, true, fmt.Errorf("set_value 只能用于 date/time")
		}
		return nil, true, w.setDateText(lang.ToString(lang.Pos(args, 0, "")))
	case "today", "now":
		if w.kind == "date" || w.kind == "time" {
			return nil, true, w.setDateText(time.Now().Format("2006-01-02 15:04:05"))
		}
		return nil, true, fmt.Errorf("today 只能用于 date/time")
	case "load", "set_src":
		if w.kind != "image" {
			return nil, true, fmt.Errorf("load 只能用于 image")
		}
		if len(args) < 1 {
			return nil, true, fmt.Errorf("load(图片路径) 需要一个参数")
		}
		return nil, true, w.loadImage(lang.ToString(args[0]))
	case "focus":
		setFocus(w.hwnd)
		return nil, true, nil
	case "click":
		sendMessage(w.hwnd, bmClick, 0, 0)
		return nil, true, nil
	case "has_node":
		if w.kind != "tree" {
			return nil, true, fmt.Errorf("has_node 只适用于 tree")
		}
		return w.treeFind(lang.ToString(lang.Pos(args, 0, ""))) != 0, true, nil
	case "add", "add_root":
		if w.kind != "tree" {
			return nil, true, fmt.Errorf("add 只适用于 tree")
		}
		text := lang.ToString(lang.Pos(args, 0, ""))
		return nil, true, w.treeAdd(text, 0)
	case "add_child":
		if w.kind != "tree" {
			return nil, true, fmt.Errorf("add_child 只适用于 tree")
		}
		if len(args) < 2 {
			return nil, true, fmt.Errorf("add_child(父节点文字, 新节点文字) 需要两个参数")
		}
		parent := w.treeFind(lang.ToString(args[0]))
		if parent == 0 {
			return nil, true, fmt.Errorf("找不到父节点 %q", lang.ToString(args[0]))
		}
		return nil, true, w.treeAdd(lang.ToString(args[1]), parent)
	case "expand_all":
		if w.kind != "tree" {
			return nil, true, fmt.Errorf("expand_all 只适用于 tree")
		}
		w.treeExpandAll(true)
		return nil, true, nil
	case "collapse_all":
		if w.kind != "tree" {
			return nil, true, fmt.Errorf("collapse_all 只适用于 tree")
		}
		w.treeExpandAll(false)
		return nil, true, nil
	case "add_row", "append_row":
		if w.kind != "table" {
			return nil, true, fmt.Errorf("add_row 只适用于 table")
		}
		cells, err := toItems(lang.Pos(args, 0, nil))
		if err != nil {
			return nil, true, fmt.Errorf("add_row(列表) 需要一个列表")
		}
		w.addRow(cells)
		return nil, true, nil
	case "set_cell":
		if w.kind != "table" {
			return nil, true, fmt.Errorf("set_cell 只适用于 table")
		}
		if len(args) < 3 {
			return nil, true, fmt.Errorf("set_cell(行, 列, 文本) 需要三个参数")
		}
		rf, ok1 := args[0].(float64)
		cf, ok2 := args[1].(float64)
		if !ok1 || !ok2 {
			return nil, true, fmt.Errorf("set_cell 的行列必须是数字")
		}
		w.setTableCell(int32(rf), int32(cf), lang.ToString(args[2]))
		return nil, true, nil
	case "get_cell":
		if w.kind != "table" {
			return nil, true, fmt.Errorf("get_cell 只适用于 table")
		}
		if len(args) < 2 {
			return nil, true, fmt.Errorf("get_cell(行, 列) 需要两个参数")
		}
		rf, ok1 := args[0].(float64)
		cf, ok2 := args[1].(float64)
		if !ok1 || !ok2 {
			return nil, true, fmt.Errorf("get_cell 的行列必须是数字")
		}
		return w.tableCell(int(rf), int(cf)), true, nil
	case "remove_row":
		if w.kind != "table" {
			return nil, true, fmt.Errorf("remove_row 只适用于 table")
		}
		if len(args) < 1 {
			return nil, true, fmt.Errorf("remove_row(行) 需要一个参数")
		}
		f, ok := args[0].(float64)
		if !ok {
			return nil, true, fmt.Errorf("remove_row 的行必须是数字")
		}
		sendMessage(w.hwnd, lvmDeleteItem, uintptr(int(f)), 0)
		return nil, true, nil
	case "image", "sprite":
		if w.kind != "canvas" {
			return nil, true, fmt.Errorf("image 只适用于 canvas")
		}
		return nil, true, w.canvasImage(args, kwargs)
	case "tilemap":
		if w.kind != "canvas" {
			return nil, true, fmt.Errorf("tilemap 只适用于 canvas")
		}
		return nil, true, w.canvasTilemap(args, kwargs)
	case "batch_begin":
		w.canvasBatchBegin()
		return nil, true, nil
	case "batch_end", "flip":
		w.canvasBatchEnd()
		return nil, true, nil
	case "fullscreen":
		if w.kind != "window" {
			return nil, true, fmt.Errorf("fullscreen 只适用于 window")
		}
		v, err := w.nativeFullscreen(args, kwargs)
		return v, true, err
	case "borderless":
		if w.kind != "window" {
			return nil, true, fmt.Errorf("borderless 只适用于 window")
		}
		v, err := w.nativeBorderless(args, kwargs)
		return v, true, err
	case "add_page":
		if w.kind != "tabs" {
			return nil, true, fmt.Errorf("add_page 只能用于 tabs")
		}
		page, err := w.addPage(lang.ToString(lang.Pos(args, 0, "页面")))
		if err != nil {
			return nil, true, err
		}
		return page, true, nil
	case "page":
		if w.kind != "tabs" {
			return nil, true, fmt.Errorf("page 只能用于 tabs")
		}
		f, _ := lang.Pos(args, 0, float64(0)).(float64)
		idx := int(f)
		if idx < 0 || idx >= len(w.tabPages) {
			return nil, true, fmt.Errorf("页面下标越界")
		}
		return w.tabPages[idx], true, nil
	case "pane":
		if w.kind != "splitter" {
			return nil, true, fmt.Errorf("pane 只能用于 splitter")
		}
		f, _ := lang.Pos(args, 0, float64(0)).(float64)
		p := w.splitterPane(int(f))
		if p == nil {
			return nil, true, fmt.Errorf("分栏不存在")
		}
		return p, true, nil
	case "set_prop":
		if w.kind != "table" {
			return nil, true, fmt.Errorf("set_prop 只能用于 propertygrid/table")
		}
		if len(args) < 2 {
			return nil, true, fmt.Errorf("set_prop(键, 值) 需要两个参数")
		}
		w.setProperty(lang.ToString(args[0]), lang.ToString(args[1]))
		return nil, true, nil
	case "get_prop":
		if w.kind != "table" {
			return nil, true, fmt.Errorf("get_prop 只能用于 propertygrid/table")
		}
		return w.getProperty(lang.ToString(lang.Pos(args, 0, ""))), true, nil
	case "clear":
		switch w.kind {
		case "edit", "richedit":
			setWindowText(w.hwnd, "")
		case "table":
			sendMessage(w.hwnd, lvmDeleteAllItems, 0, 0)
		case "tree":
			sendMessage(w.hwnd, tvmDeleteItem, 0, 0)
		case "canvas":
			w.canvasOps = nil
			invalidateRect(w.hwnd, nil, true)
		case "listbox", "combo":
			msg := uint32(lbResetContent)
			if w.kind == "combo" {
				msg = cbResetContent
			}
			sendMessage(w.hwnd, msg, 0, 0)
			w.items = nil
		}
		return nil, true, nil
	case "add_item", "append_item":
		if w.kind != "listbox" && w.kind != "combo" {
			return nil, true, fmt.Errorf("%s 控件不能添加条目", w.kind)
		}
		s := lang.ToString(lang.Pos(args, 0, ""))
		idx := w.app.listAdd(w, s)
		return float64(idx), true, nil
	case "remove_item":
		if w.kind != "listbox" && w.kind != "combo" {
			return nil, true, fmt.Errorf("%s 控件没有条目", w.kind)
		}
		if len(args) < 1 {
			return nil, true, fmt.Errorf("remove_item(下标) 需要下标")
		}
		f, ok := args[0].(float64)
		if !ok {
			return nil, true, fmt.Errorf("remove_item(下标) 需要数字")
		}
		idx := int(f)
		if idx < 0 || idx >= len(w.items) {
			return nil, true, fmt.Errorf("条目下标越界")
		}
		msg := uint32(lbDeleteString)
		if w.kind == "combo" {
			msg = cbDeleteString
		}
		sendMessage(w.hwnd, msg, uintptr(idx), 0)
		w.items = append(w.items[:idx], w.items[idx+1:]...)
		return nil, true, nil
	case "sort":
		if w.kind != "table" {
			return nil, true, fmt.Errorf("sort 只适用于 table")
		}
		if len(args) < 1 {
			return nil, true, fmt.Errorf("sort(列, 升序=true) 至少需要列号")
		}
		cf, ok := args[0].(float64)
		if !ok {
			return nil, true, fmt.Errorf("sort 的列号必须是数字")
		}
		asc := true
		if len(args) > 1 {
			asc = lang.Truthy(args[1])
		}
		w.tableSort(int(cf), asc)
		return nil, true, nil
	case "edit_cell", "cell_edit":
		if w.kind != "table" {
			return nil, true, fmt.Errorf("edit_cell 只适用于 table")
		}
		if len(args) < 2 {
			return nil, true, fmt.Errorf("edit_cell(行, 列) 需要两个参数")
		}
		rf, ok1 := args[0].(float64)
		cf, ok2 := args[1].(float64)
		if !ok1 || !ok2 {
			return nil, true, fmt.Errorf("edit_cell 的行列必须是数字")
		}
		return nil, true, w.editCell(int32(rf), int32(cf))
	case "set_edit_text":
		if w.kind != "table" || w.editHwnd == 0 {
			return nil, true, fmt.Errorf("当前没有正在编辑的单元格")
		}
		setWindowText(w.editHwnd, lang.ToString(lang.Pos(args, 0, "")))
		return nil, true, nil
	case "end_edit", "commit_edit":
		if w.kind != "table" {
			return nil, true, fmt.Errorf("end_edit 只适用于 table")
		}
		save := true
		if len(args) > 0 {
			save = lang.Truthy(args[0])
		}
		w.finishCellEdit(save)
		return nil, true, nil
	case "line":
		if w.kind != "canvas" {
			return nil, true, fmt.Errorf("line 只适用于 canvas")
		}
		return nil, true, w.canvasLine(args, kwargs)
	case "rect":
		if w.kind != "canvas" {
			return nil, true, fmt.Errorf("rect 只适用于 canvas")
		}
		return nil, true, w.canvasRect(args, kwargs)
	case "circle":
		if w.kind != "canvas" {
			return nil, true, fmt.Errorf("circle 只适用于 canvas")
		}
		return nil, true, w.canvasCircle(args, kwargs)
	case "text":
		if w.kind != "canvas" {
			return nil, true, fmt.Errorf("text 只适用于 canvas")
		}
		return nil, true, w.canvasText(args, kwargs)
	case "redraw":
		if w.kind != "canvas" {
			return nil, true, fmt.Errorf("redraw 只适用于 canvas")
		}
		invalidateRect(w.hwnd, nil, true)
		return nil, true, nil
	case "select":
		if len(args) < 1 {
			return nil, true, fmt.Errorf("select(下标) 需要下标")
		}
		if w.kind == "tree" {
			text := lang.ToString(args[0])
			h := w.treeFind(text)
			if h == 0 {
				return nil, true, fmt.Errorf("找不到树节点 %q", text)
			}
			w.selectTreeHandle(h)
			return nil, true, nil
		}
		f, ok := args[0].(float64)
		if !ok {
			return nil, true, fmt.Errorf("select(下标) 需要数字")
		}
		if w.kind == "table" {
			w.tableSelect(int(f))
			return nil, true, nil
		}
		return nil, true, w.SetProp("selected", f)
	case "append":
		if w.kind != "edit" && w.kind != "richedit" {
			return nil, true, fmt.Errorf("append 只适用于 edit/richedit")
		}
		s := lang.ToString(lang.Pos(args, 0, ""))
		if w.kind == "richedit" {
			return nil, true, w.appendRich(s, kwargs)
		}
		sendMessage(w.hwnd, emSetSel, ^uintptr(0), ^uintptr(0))
		sendMessage(w.hwnd, emReplaceSel, 1, uintptr(unsafe.Pointer(u16(s))))
		return nil, true, nil
	case "format":
		if w.kind != "richedit" {
			return nil, true, fmt.Errorf("format 只适用于 richedit")
		}
		all := true
		if v, ok := kwargs["all"]; ok {
			all = lang.Truthy(v)
		}
		if err := w.applyRichFormat(kwargs, all); err != nil {
			return nil, true, err
		}
		return nil, true, w.applyRichPara(kwargs, all)
	case "select_all":
		if w.kind != "richedit" {
			return nil, true, fmt.Errorf("select_all 只适用于 richedit")
		}
		sendMessage(w.hwnd, emSetSel, 0, ^uintptr(0))
		return nil, true, nil
	case "select_range":
		if w.kind != "richedit" {
			return nil, true, fmt.Errorf("select_range 只适用于 richedit")
		}
		if len(args) < 2 {
			return nil, true, fmt.Errorf("select_range(start, end) 需要两个参数")
		}
		a, ok1 := args[0].(float64)
		b, ok2 := args[1].(float64)
		if !ok1 || !ok2 {
			return nil, true, fmt.Errorf("select_range 参数必须是数字")
		}
		w.richSelectRange(int32(a), int32(b))
		return nil, true, nil
	case "selected_text":
		if w.kind != "richedit" {
			return nil, true, fmt.Errorf("selected_text 只适用于 richedit")
		}
		return w.richSelectedText(), true, nil
	case "selection":
		if w.kind != "richedit" {
			return nil, true, fmt.Errorf("selection 只适用于 richedit")
		}
		start, end := w.richSelection()
		return lang.NewList([]lang.Value{float64(start), float64(end)}), true, nil
	case "insert_text":
		if w.kind != "richedit" {
			return nil, true, fmt.Errorf("insert_text 只适用于 richedit")
		}
		w.richInsertText(lang.ToString(lang.Pos(args, 0, "")), false)
		return nil, true, nil
	case "replace_selection":
		if w.kind != "richedit" {
			return nil, true, fmt.Errorf("replace_selection 只适用于 richedit")
		}
		w.richInsertText(lang.ToString(lang.Pos(args, 0, "")), true)
		return nil, true, nil
	case "insert_image":
		if w.kind != "richedit" {
			return nil, true, fmt.Errorf("insert_image 只适用于 richedit")
		}
		if len(args) < 1 {
			return nil, true, fmt.Errorf("insert_image(图片) 需要一个参数")
		}
		return nil, true, w.richInsertImage(args[0])
	case "link":
		if w.kind != "richedit" {
			return nil, true, fmt.Errorf("link 只适用于 richedit")
		}
		if len(args) < 2 {
			return nil, true, fmt.Errorf("link(文字, URL) 需要两个参数")
		}
		return nil, true, w.richLink(lang.ToString(args[0]), lang.ToString(args[1]))
	case "scroll_caret":
		if w.kind != "richedit" {
			return nil, true, fmt.Errorf("scroll_caret 只适用于 richedit")
		}
		sendMessage(w.hwnd, emScrollCaret, 0, 0)
		return nil, true, nil
	case "close":
		if w.kind == "window" {
			pPostMessageW.Call(uintptr(w.hwnd), wmClose, 0, 0)
			return nil, true, nil
		}
		return nil, true, fmt.Errorf("只有窗口可以 close")
	case "center":
		if w.kind == "window" {
			w.center()
			return nil, true, nil
		}
		return nil, true, fmt.Errorf("只有窗口可以 center")
	case "minimize":
		showWindow(w.hwnd, swMinimize)
		return nil, true, nil
	case "maximize":
		showWindow(w.hwnd, swMaximize)
		return nil, true, nil
	case "restore":
		showWindow(w.hwnd, swRestore)
		return nil, true, nil
	case "show":
		showWindow(w.hwnd, swShow)
		return nil, true, nil
	case "hide":
		showWindow(w.hwnd, swHide)
		return nil, true, nil
	case "hover":
		if w.tooltip == "" {
			return nil, true, fmt.Errorf("控件没有 tooltip")
		}
		w.hoverTooltip()
		return nil, true, nil
	case "set_range":
		if w.kind != "slider" && w.kind != "progress" {
			return nil, true, fmt.Errorf("%s 没有范围", w.kind)
		}
		if len(args) != 2 {
			return nil, true, fmt.Errorf("set_range(最小, 最大) 需要两个参数")
		}
		lo, ok1 := args[0].(float64)
		hi, ok2 := args[1].(float64)
		if !ok1 || !ok2 {
			return nil, true, fmt.Errorf("set_range 的参数必须是数字")
		}
		w.minV, w.maxV = lo, hi
		w.applyRange()
		return nil, true, nil
	}
	return nil, false, nil
}

func (w *Widget) setOwnerDraw() {
	if w.kind != "button" || w.ownerDraw || w.hwnd == 0 {
		return
	}
	style := getWindowLong(w.hwnd, gwlStyle)
	setWindowLong(w.hwnd, gwlStyle, style|bsOwnerDraw)
	w.ownerDraw = true
	invalidateRect(w.hwnd, nil, true)
}

func shadeColor(c uint32, f float64) uint32 {
	r := float64(c&0xFF) * f
	g := float64((c>>8)&0xFF) * f
	b := float64((c>>16)&0xFF) * f
	clamp := func(v float64) uint32 {
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		return uint32(v)
	}
	return clamp(r) | clamp(g)<<8 | clamp(b)<<16
}

func (w *Widget) drawButton(dis *drawItemStruct) {
	hdc := dis.hDC
	bg := uint32(0xF0F0F0)
	fg := uint32(0x1E1E1E)
	if w.hasBg {
		bg = w.bg
	}
	if w.hasFg {
		fg = w.fg
	}
	disabled := dis.itemState&odsDisabled != 0
	selected := dis.itemState&odsSelected != 0
	if disabled {
		bg = shadeColor(bg, 1.08)
		fg = 0x9A9A9A
	} else if selected {
		bg = shadeColor(bg, 0.86)
	}
	fillRect(hdc, &dis.rcItem, w.app.brushFor(bg))
	edge := uint32(edgeRaised)
	if selected {
		edge = edgeSunken
	}
	drawEdge(hdc, &dis.rcItem, edge, bfRect)

	textRect := dis.rcItem
	textRect.left += w.app.px(8)
	textRect.right -= w.app.px(8)
	textRect.top += 1
	textRect.bottom -= 1
	setBkMode(hdc, 1)
	setTextColor(hdc, fg)
	font := w.app.hFont
	if w.font != nil {
		if h, ok := w.font.Handle.(syscall.Handle); ok && h != 0 {
			font = h
		}
	}
	old, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(font))
	drawText(hdc, getWindowText(w.hwnd), &textRect, dtCenter|dtVCenter|dtSingleLine|dtEndEllipsis)
	pSelectObject.Call(uintptr(hdc), old)

	if dis.itemState&odsFocus != 0 {
		focus := dis.rcItem
		focus.left += w.app.px(4)
		focus.top += w.app.px(4)
		focus.right -= w.app.px(4)
		focus.bottom -= w.app.px(4)
		drawFocusRect(hdc, &focus)
	}
	if dis.itemState&odsDefault != 0 {
		outer := dis.rcItem
		outer.left++
		outer.top++
		outer.right--
		outer.bottom--
		frameRect(hdc, &outer, w.app.brushFor(0x000000))
	}
}

func (a *App) createImage(path string, x, y, w, h *int32) (*Widget, error) {
	iw, ih := probeImageSize(path)
	if iw <= 0 || ih <= 0 {
		iw, ih = a.px(240), a.px(160)
	}
	maxW, maxH := a.px(480), a.px(360)
	if iw > maxW {
		ih = ih * maxW / iw
		iw = maxW
	}
	if ih > maxH {
		iw = iw * maxH / ih
		ih = maxH
	}
	widget, err := a.addChild("image", imageClassName, "", 0, false, iw, ih, x, y, w, h)
	if err != nil {
		return nil, err
	}
	widget.imgFit = true
	if path != "" {
		if err := widget.loadImage(path); err != nil {
			return nil, err
		}
	}
	return widget, nil
}

func (w *Widget) setWindowIcon(path string) error {
	if w.kind != "window" {
		return fmt.Errorf("icon 只能在窗口上设置")
	}
	resolved, err := w.app.interp.AssetPath(path)
	if err != nil {
		return err
	}
	img, _, _, err := loadGdiImage(resolved)
	if err != nil {
		return err
	}
	defer disposeGdiImage(img)
	var hicon uintptr
	if status, _, _ := pGdipCreateHICONFromBitmap.Call(img, uintptr(unsafe.Pointer(&hicon))); status != 0 || hicon == 0 {
		return fmt.Errorf("无法把 %q 转换为窗口图标", path)
	}
	if w.iconBig != 0 {
		pDestroyIcon.Call(uintptr(w.iconBig))
	}
	if w.iconSmall != 0 {
		pDestroyIcon.Call(uintptr(w.iconSmall))
	}
	w.iconBig = syscall.Handle(hicon)
	w.iconSmall = syscall.Handle(hicon)
	sendMessage(w.hwnd, wmSetIcon, iconBig, hicon)
	sendMessage(w.hwnd, wmSetIcon, iconSmall, hicon)
	return nil
}

func (w *Widget) center() {
	if w.kind != "window" {
		return
	}
	r, _ := getWindowRect(w.hwnd)
	sw, sh := screenSize()
	width := r.right - r.left
	height := r.bottom - r.top
	x := (sw - width) / 2
	y := (sh - height) / 2
	moveWindow(w.hwnd, x, y, width, height, true)
}
