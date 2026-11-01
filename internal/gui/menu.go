package gui

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"agui/internal/lang"
)

// Menu is a popup menu. A top-level Menu is attached to a window's menu bar;
// a submenu is attached to its parent popup.
type Menu struct {
	app    *App
	window *Widget
	parent *Menu
	hmenu  syscall.Handle
	title  string
	index  int // position in the parent/window menu bar
}

// MenuItem is one clickable entry. It has no HWND; clicks arrive through
// WM_COMMAND and are routed by id.
type MenuItem struct {
	app       *App
	menu      *Menu
	id        int
	text      string
	accel     string
	checkable bool
	checked   bool
	enabled   bool
	iconPath  string
	hBitmap   syscall.Handle
	handlers  map[string][]func() error
}

const (
	mfString     = 0x00000000
	mfPopup      = 0x00000010
	mfSeparator  = 0x00000800
	mfByCommand  = 0x00000000
	mfByPosition = 0x00000400
	mfGrayed     = 0x00000001
	mfEnabled    = 0x00000000
	mfChecked    = 0x00000008
	mfUnchecked  = 0x00000000
)

var (
	pCreateMenu             = user32.NewProc("CreateMenu")
	pCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	pAppendMenuW            = user32.NewProc("AppendMenuW")
	pSetMenu                = user32.NewProc("SetMenu")
	pDrawMenuBar            = user32.NewProc("DrawMenuBar")
	pModifyMenuW            = user32.NewProc("ModifyMenuW")
	pEnableMenuItem         = user32.NewProc("EnableMenuItem")
	pCheckMenuItem          = user32.NewProc("CheckMenuItem")
	pSetMenuItemBitmaps     = user32.NewProc("SetMenuItemBitmaps")
	pCreateAcceleratorTable = user32.NewProc("CreateAcceleratorTableW")
	pTranslateAccelerator   = user32.NewProc("TranslateAcceleratorW")
	pDestroyAccelerator     = user32.NewProc("DestroyAcceleratorTable")
)

// accelW mirrors Win32 ACCEL.
type accelW struct {
	fVirt uint8
	key   uint16
	cmd   uint16
}

const (
	fvirtKey = 0x01
	fshift   = 0x04
	fcontrol = 0x08
	falt     = 0x10
)

func (a *App) newMenu(title string) (*Menu, error) {
	win := a.EnsureWindow()
	if win.menuBar == 0 {
		hbar, _, _ := pCreateMenu.Call()
		if hbar == 0 {
			return nil, fmt.Errorf("创建菜单栏失败")
		}
		win.menuBar = syscall.Handle(hbar)
		a.hMenu = win.menuBar
		pSetMenu.Call(uintptr(win.hwnd), uintptr(win.menuBar))
	}
	popup, _, _ := pCreatePopupMenu.Call()
	if popup == 0 {
		return nil, fmt.Errorf("创建菜单失败")
	}
	pAppendMenuW.Call(uintptr(win.menuBar), uintptr(mfPopup), popup, uintptr(unsafe.Pointer(u16(title))))
	pDrawMenuBar.Call(uintptr(win.hwnd))
	m := &Menu{app: a, window: win, hmenu: syscall.Handle(popup), title: title, index: len(win.menus)}
	win.menus = append(win.menus, m)
	// 菜单栏会占用客户区高度，重新排列自动布局的控件。
	a.layoutWindow(win)
	return m, nil
}

func (a *App) callback(fn lang.Value) (func() error, error) {
	if !callable(fn) {
		return nil, fmt.Errorf("菜单回调必须是函数")
	}
	return func() error {
		return a.interp.Protect(func() { a.interp.CallValue(fn) })
	}, nil
}

func (m *Menu) drawBar() {
	if m.window != nil {
		pDrawMenuBar.Call(uintptr(m.window.hwnd))
	}
}

var accelKeys = map[string]uint16{
	"ENTER": 0x0D, "RETURN": 0x0D, "ESC": 0x1B, "ESCAPE": 0x1B,
	"TAB": 0x09, "SPACE": 0x20, "BACKSPACE": 0x08, "DELETE": 0x2E,
	"DEL": 0x2E, "INSERT": 0x2D, "INS": 0x2D, "HOME": 0x24, "END": 0x23,
	"PAGEUP": 0x21, "PAGEDOWN": 0x22, "UP": 0x26, "DOWN": 0x28,
	"LEFT": 0x25, "RIGHT": 0x27, "PLUS": 0xBB, "MINUS": 0xBD,
}

// parseAccel turns "Ctrl+Shift+S" or "F5" into a Win32 ACCEL entry.
func parseAccel(s string) (accelW, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return accelW{}, false
	}
	parts := strings.Split(s, "+")
	var out accelW
	out.fVirt = fvirtKey
	for idx := 0; idx < len(parts)-1; idx++ {
		switch strings.ToLower(strings.TrimSpace(parts[idx])) {
		case "ctrl", "control":
			out.fVirt |= fcontrol
		case "shift":
			out.fVirt |= fshift
		case "alt":
			out.fVirt |= falt
		default:
			return accelW{}, false
		}
	}
	key := strings.ToUpper(strings.TrimSpace(parts[len(parts)-1]))
	if len(key) == 1 {
		r := rune(key[0])
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out.key = uint16(r)
			return out, true
		}
	}
	if len(key) >= 2 && key[0] == 'F' {
		if n, err := strconv.Atoi(key[1:]); err == nil && n >= 1 && n <= 24 {
			out.key = uint16(0x70 + n - 1)
			return out, true
		}
	}
	if code, ok := accelKeys[key]; ok {
		out.key = code
		return out, true
	}
	return accelW{}, false
}

func (a *App) rebuildAccelerators() {
	if a.accelTable != 0 {
		pDestroyAccelerator.Call(uintptr(a.accelTable))
		a.accelTable = 0
	}
	var accels []accelW
	for _, item := range a.menuItems {
		if !item.enabled || item.accel == "" {
			continue
		}
		ac, ok := parseAccel(item.accel)
		if !ok {
			continue
		}
		ac.cmd = uint16(item.id)
		accels = append(accels, ac)
	}
	if len(accels) == 0 {
		return
	}
	h, _, _ := pCreateAcceleratorTable.Call(uintptr(unsafe.Pointer(&accels[0])), uintptr(len(accels)))
	if h != 0 {
		a.accelTable = syscall.Handle(h)
	}
}

func (a *App) translateAccelerator(msg *msgT) bool {
	if a.accelTable == 0 {
		return false
	}
	for _, w := range a.windows {
		if r, _, _ := pTranslateAccelerator.Call(uintptr(w.hwnd), uintptr(a.accelTable), uintptr(unsafe.Pointer(msg))); r != 0 {
			return true
		}
	}
	return false
}

func (m *Menu) displayText(text, accel string, checkable, checked, enabled bool) string {
	if accel != "" {
		text += "\t" + accel
	}
	return text
}

func (m *Menu) addItemEx(text string, fn lang.Value, accel string, checkable, checked, enabled bool) (*MenuItem, error) {
	item := &MenuItem{
		app:       m.app,
		menu:      m,
		id:        m.app.nextID,
		text:      text,
		accel:     accel,
		checkable: checkable,
		checked:   checked,
		enabled:   enabled,
		handlers:  map[string][]func() error{},
	}
	if accel != "" {
		if _, ok := parseAccel(accel); !ok {
			return nil, fmt.Errorf("无法识别快捷键 %q（示例：Ctrl+O、Ctrl+Shift+S、F5）", accel)
		}
	}
	m.app.nextID++
	flags := uintptr(mfString)
	if checkable && checked {
		flags |= mfChecked
	}
	if !enabled {
		flags |= mfGrayed
	}
	display := m.displayText(text, accel, checkable, checked, enabled)
	pAppendMenuW.Call(uintptr(m.hmenu), flags, uintptr(item.id), uintptr(unsafe.Pointer(u16(display))))
	m.app.menuItems[item.id] = item
	m.app.rebuildAccelerators()
	if fn != nil {
		cb, err := m.app.callback(fn)
		if err != nil {
			return nil, err
		}
		item.handlers["click"] = append(item.handlers["click"], cb)
	}
	m.drawBar()
	return item, nil
}

func (m *Menu) addItem(text string, fn lang.Value) (*MenuItem, error) {
	return m.addItemEx(text, fn, "", false, false, true)
}

func (m *Menu) addSubmenu(text string) (*Menu, error) {
	popup, _, _ := pCreatePopupMenu.Call()
	if popup == 0 {
		return nil, fmt.Errorf("创建子菜单失败")
	}
	pAppendMenuW.Call(uintptr(m.hmenu), uintptr(mfPopup), popup, uintptr(unsafe.Pointer(u16(text))))
	sub := &Menu{app: m.app, window: m.window, parent: m, hmenu: syscall.Handle(popup), title: text}
	m.drawBar()
	return sub, nil
}

func (m *Menu) Kind() string { return "menu" }

func (m *Menu) GetProp(name string) (lang.Value, bool) {
	switch name {
	case "kind":
		return "menu", true
	case "title", "text":
		return m.title, true
	}
	return nil, false
}

func (m *Menu) SetProp(name string, v lang.Value) error {
	switch name {
	case "title", "text":
		m.title = lang.ToString(v)
		if m.parent == nil && m.window != nil {
			// Re-label the popup in place, preserving its submenu handle.
			pModifyMenuW.Call(uintptr(m.window.menuBar), uintptr(m.index)|uintptr(mfByPosition|mfPopup),
				uintptr(m.hmenu), uintptr(unsafe.Pointer(u16(m.title))))
			m.drawBar()
		}
		return nil
	}
	return fmt.Errorf("菜单不支持设置属性 %q", name)
}

func (m *Menu) CallMethod(name string, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, bool, error) {
	switch name {
	case "item", "add_item":
		if len(args) < 1 || len(args) > 3 {
			return nil, true, fmt.Errorf("item(文字, 回调?, 快捷键?) 接受 1 到 3 个参数")
		}
		var fn lang.Value
		if len(args) >= 2 {
			fn = args[1]
		}
		accel := kwStr(kwargs, "accel", "")
		if len(args) == 3 {
			accel = lang.ToString(args[2])
		}
		item, err := m.addItemEx(
			lang.ToString(args[0]), fn, accel,
			kwBool(kwargs, "checkable", false) || kwBool(kwargs, "checked", false),
			kwBool(kwargs, "checked", false),
			kwBool(kwargs, "enabled", true),
		)
		if err != nil {
			return nil, true, err
		}
		if icon := kwStr(kwargs, "icon", ""); icon != "" {
			if err := item.setIcon(icon); err != nil {
				return nil, true, err
			}
		}
		return item, true, nil
	case "submenu":
		if len(args) != 1 || len(kwargs) > 0 {
			return nil, true, fmt.Errorf("submenu(文字) 接受 1 个参数")
		}
		sub, err := m.addSubmenu(lang.ToString(args[0]))
		if err != nil {
			return nil, true, err
		}
		return sub, true, nil
	case "separator", "sep":
		pAppendMenuW.Call(uintptr(m.hmenu), uintptr(mfSeparator), 0, 0)
		m.drawBar()
		return nil, true, nil
	}
	return nil, false, nil
}

func (m *Menu) Listen(string, func() error) error {
	return fmt.Errorf("menu 本身不触发事件，请用 menu.item() 创建菜单项并监听 click")
}

func (mi *MenuItem) setIcon(path string) error {
	if path == "" {
		if mi.hBitmap != 0 {
			pDeleteObject.Call(uintptr(mi.hBitmap))
			mi.hBitmap = 0
		}
		mi.iconPath = ""
		if mi.menu != nil {
			pSetMenuItemBitmaps.Call(uintptr(mi.menu.hmenu), uintptr(mi.id), mfByCommand, 0, 0)
		}
		return nil
	}
	resolved, err := mi.app.interp.AssetPath(path)
	if err != nil {
		return err
	}
	h, err := loadMenuBitmap(resolved)
	if err != nil {
		return err
	}
	if mi.hBitmap != 0 {
		pDeleteObject.Call(uintptr(mi.hBitmap))
	}
	mi.hBitmap = h
	mi.iconPath = path
	mi.applyIcon()
	mi.menu.drawBar()
	return nil
}

func (mi *MenuItem) applyIcon() {
	if mi.menu == nil || mi.hBitmap == 0 {
		return
	}
	checked := uintptr(0)
	if !mi.checkable {
		checked = uintptr(mi.hBitmap)
	}
	pSetMenuItemBitmaps.Call(uintptr(mi.menu.hmenu), uintptr(mi.id), mfByCommand, uintptr(mi.hBitmap), checked)
}

func (mi *MenuItem) Kind() string { return "menuitem" }

func (mi *MenuItem) GetProp(name string) (lang.Value, bool) {
	switch name {
	case "kind":
		return "menuitem", true
	case "text":
		return mi.text, true
	case "accel":
		return mi.accel, true
	case "checked":
		return mi.checked, true
	case "checkable":
		return mi.checkable, true
	case "enabled":
		return mi.enabled, true
	case "icon":
		return mi.iconPath, true
	case "has_icon":
		return mi.hBitmap != 0, true
	}
	return nil, false
}

func (mi *MenuItem) refresh() {
	if mi.menu == nil {
		return
	}
	flags := uintptr(mfByCommand | mfString)
	display := mi.menu.displayText(mi.text, mi.accel, mi.checkable, mi.checked, mi.enabled)
	pModifyMenuW.Call(uintptr(mi.menu.hmenu), uintptr(mi.id), flags, uintptr(mi.id),
		uintptr(unsafe.Pointer(u16(display))))
	mi.applyCheckFlags()
	mi.applyEnabled()
	mi.applyIcon()
	mi.menu.drawBar()
}

func (mi *MenuItem) applyCheckFlags() {
	if !mi.checkable || mi.menu == nil {
		return
	}
	flags := uintptr(mfByCommand)
	if mi.checked {
		flags |= mfChecked
	} else {
		flags |= mfUnchecked
	}
	pCheckMenuItem.Call(uintptr(mi.menu.hmenu), uintptr(mi.id), flags)
}

func (mi *MenuItem) applyEnabled() {
	if mi.menu == nil {
		return
	}
	flags := uintptr(mfByCommand | mfEnabled)
	if !mi.enabled {
		flags = mfByCommand | mfGrayed
	}
	pEnableMenuItem.Call(uintptr(mi.menu.hmenu), uintptr(mi.id), flags)
}

func (mi *MenuItem) setChecked(v bool) {
	mi.checked = v
	mi.checkable = true
	mi.applyCheckFlags()
	mi.menu.drawBar()
}

func (mi *MenuItem) SetProp(name string, v lang.Value) error {
	switch name {
	case "text":
		mi.text = lang.ToString(v)
		mi.refresh()
		return nil
	case "accel":
		s := lang.ToString(v)
		if s != "" {
			if _, ok := parseAccel(s); !ok {
				return fmt.Errorf("无法识别快捷键 %q", s)
			}
		}
		mi.accel = s
		mi.refresh()
		mi.app.rebuildAccelerators()
		return nil
	case "icon", "image":
		return mi.setIcon(lang.ToString(v))
	case "checked":
		mi.setChecked(lang.Truthy(v))
		return nil
	case "checkable":
		mi.checkable = lang.Truthy(v)
		mi.refresh()
		return nil
	case "enabled":
		mi.enabled = lang.Truthy(v)
		mi.applyEnabled()
		mi.app.rebuildAccelerators()
		mi.menu.drawBar()
		return nil
	}
	return fmt.Errorf("菜单项没有属性 %q", name)
}

func (mi *MenuItem) CallMethod(name string, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, bool, error) {
	switch name {
	case "click":
		mi.emit("click")
		return nil, true, nil
	case "check":
		mi.setChecked(true)
		return nil, true, nil
	case "uncheck":
		mi.setChecked(false)
		return nil, true, nil
	case "toggle":
		mi.setChecked(!mi.checked)
		return nil, true, nil
	case "icon", "set_icon":
		if len(args) != 1 {
			return nil, true, fmt.Errorf("icon(路径) 需要 1 个参数")
		}
		return nil, true, mi.setIcon(lang.ToString(args[0]))
	}
	return nil, false, nil
}

func (mi *MenuItem) Listen(event string, fn func() error) error {
	if event != "click" {
		return fmt.Errorf("菜单项只支持 click 事件")
	}
	mi.handlers[event] = append(mi.handlers[event], fn)
	return nil
}

func (mi *MenuItem) emit(event string) {
	if event == "click" && mi.checkable {
		mi.setChecked(!mi.checked)
	}
	for _, fn := range mi.handlers[event] {
		if err := fn(); err != nil {
			mi.app.reportError(err)
		}
	}
}
