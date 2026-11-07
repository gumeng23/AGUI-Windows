package gui

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"agui/internal/lang"
)

// Tray is a system tray (notification area) icon with an optional popup menu.
// It implements lang.Widget so it can be stored in variables and receive
// click/dblclick events like any other GUI object.
type Tray struct {
	app      *App
	hwnd     syscall.Handle
	id       uint32
	iconPath string
	tooltip  string
	hIcon    syscall.Handle
	menu     *Menu
	added    bool
	lastErr  string
	handlers map[string][]func() error
}

const (
	nimAdd        = 0
	nimModify     = 1
	nimDelete     = 2
	nimSetVersion = 4

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004
	nifInfo    = 0x00000010

	niifInfo    = 0x00000001
	niifWarning = 0x00000002
	niifError   = 0x00000003

	wmTray         = 0x8000 + 2
	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100
	tpmLeftAlign   = 0x0000
	scMinimize     = 0xF020
)

// notifyIconDataW mirrors NOTIFYICONDATAW. Windows 10+ expects cbSize 976;
// keeping the full structure avoids truncating the balloon fields.
type notifyIconDataW struct {
	CbSize           uint32
	HWnd             syscall.Handle
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            syscall.Handle
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     syscall.Handle
}

var (
	pShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	pExtractIconExW   = shell32.NewProc("ExtractIconExW")
	pTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	pGetCursorPos     = user32.NewProc("GetCursorPos")
	pRegisterWindowMs = user32.NewProc("RegisterWindowMessageW")
)

type pointT struct {
	X, Y int32
}

func copyUTF16(dst []uint16, s string) {
	if len(dst) == 0 {
		return
	}
	runes := []rune(s)
	n := 0
	for _, r := range runes {
		if n >= len(dst)-1 {
			break
		}
		if r < 0x10000 {
			dst[n] = uint16(r)
			n++
		} else {
			r -= 0x10000
			dst[n] = uint16(0xD800 + (r >> 10))
			n++
			if n < len(dst)-1 {
				dst[n] = uint16(0xDC00 + (r & 0x3FF))
				n++
			}
		}
	}
	dst[n] = 0
}

func (a *App) newTray(iconPath, tooltip string) (*Tray, error) {
	win := a.EnsureWindow()
	t := &Tray{
		app:      a,
		hwnd:     win.hwnd,
		id:       uint32(a.nextID),
		tooltip:  tooltip,
		handlers: map[string][]func() error{},
	}
	a.nextID++
	popup, _, _ := pCreatePopupMenu.Call()
	if popup != 0 {
		t.menu = &Menu{app: a, hmenu: syscall.Handle(popup), title: "tray"}
	}
	if err := t.setIcon(iconPath); err != nil {
		return nil, err
	}
	if err := t.add(); err != nil {
		t.lastErr = err.Error()
	}
	a.tray = t
	return t, nil
}

func (t *Tray) setIcon(path string) error {
	t.iconPath = path
	var h syscall.Handle
	if strings.TrimSpace(path) != "" {
		real, apErr := t.app.interp.AssetPath(path)
		if apErr != nil {
			return apErr
		}
		var small syscall.Handle
		n, _, _ := pExtractIconExW.Call(uintptr(unsafe.Pointer(u16(real))), 0,
			uintptr(unsafe.Pointer(&h)), uintptr(unsafe.Pointer(&small)), 1)
		if n == 0 || h == 0 {
			return fmt.Errorf("加载托盘图标失败: %s", path)
		}
		if small != 0 {
			pDestroyIcon.Call(uintptr(small))
		}
	} else {
		icon, _, _ := pLoadIconW.Call(0, 32512) // IDI_APPLICATION
		h = syscall.Handle(icon)
	}
	if h == 0 {
		return fmt.Errorf("无法获得默认托盘图标")
	}
	if t.hIcon != 0 {
		pDestroyIcon.Call(uintptr(t.hIcon))
	}
	t.hIcon = h
	if t.added {
		return t.modify()
	}
	return nil
}

func (t *Tray) nid(flags uint32) notifyIconDataW {
	var d notifyIconDataW
	d.CbSize = uint32(unsafe.Sizeof(d))
	d.HWnd = t.hwnd
	d.UID = t.id
	d.UFlags = flags
	if flags&nifMessage != 0 {
		d.UCallbackMessage = wmTray
	}
	if flags&nifIcon != 0 {
		d.HIcon = t.hIcon
	}
	if flags&nifTip != 0 {
		copyUTF16(d.SzTip[:], t.tooltip)
	}
	return d
}

func (t *Tray) add() error {
	d := t.nid(nifMessage | nifIcon | nifTip)
	ok, _, err := pShellNotifyIconW.Call(uintptr(nimAdd), uintptr(unsafe.Pointer(&d)))
	if ok == 0 {
		// Explorer may already know the icon after a restart; modify instead.
		d2 := t.nid(nifMessage | nifIcon | nifTip)
		ok2, _, _ := pShellNotifyIconW.Call(uintptr(nimModify), uintptr(unsafe.Pointer(&d2)))
		if ok2 == 0 {
			return fmt.Errorf("添加托盘图标失败: %v", err)
		}
	}
	t.added = true
	return nil
}

func (t *Tray) modify() error {
	d := t.nid(nifMessage | nifIcon | nifTip)
	ok, _, err := pShellNotifyIconW.Call(uintptr(nimModify), uintptr(unsafe.Pointer(&d)))
	if ok == 0 {
		return fmt.Errorf("更新托盘图标失败: %v", err)
	}
	return nil
}

func (t *Tray) remove() {
	if !t.added {
		return
	}
	d := t.nid(0)
	pShellNotifyIconW.Call(uintptr(nimDelete), uintptr(unsafe.Pointer(&d)))
	t.added = false
}

func (t *Tray) reAdd() {
	if t.added {
		t.add()
	}
}

func (t *Tray) notify(title, text string, warning bool) error {
	d := t.nid(nifInfo)
	copyUTF16(d.SzInfoTitle[:], title)
	copyUTF16(d.SzInfo[:], text)
	d.UVersion = 10000
	if warning {
		d.DwInfoFlags = niifWarning
	} else {
		d.DwInfoFlags = niifInfo
	}
	ok, _, err := pShellNotifyIconW.Call(uintptr(nimModify), uintptr(unsafe.Pointer(&d)))
	if ok == 0 {
		return fmt.Errorf("显示托盘通知失败: %v", err)
	}
	return nil
}

func (t *Tray) showMenu() {
	if t.menu == nil || t.menu.hmenu == 0 {
		return
	}
	var pt pointT
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWindow.Call(uintptr(t.hwnd))
	cmd, _, _ := pTrackPopupMenu.Call(uintptr(t.menu.hmenu), tpmReturnCmd|tpmRightButton|tpmLeftAlign,
		uintptr(pt.X), uintptr(pt.Y), 0, uintptr(t.hwnd), 0)
	if cmd != 0 {
		if mi, ok := t.app.menuItems[int(cmd)]; ok {
			mi.emit("click")
		}
	}
}

func (a *App) handleTray(lparam uintptr) {
	t := a.tray
	if t == nil {
		return
	}
	switch uint32(lparam) {
	case 0x0201: // WM_LBUTTONDOWN
		if win := a.window; win != nil && win.hwnd != 0 {
			pShowWindow.Call(uintptr(win.hwnd), 5) // SW_SHOW
			pSetForegroundWindow.Call(uintptr(win.hwnd))
		}
		t.emit("click")
	case 0x0203: // WM_LBUTTONDBLCLK
		if win := a.window; win != nil && win.hwnd != 0 {
			pShowWindow.Call(uintptr(win.hwnd), 5)
			pSetForegroundWindow.Call(uintptr(win.hwnd))
		}
		t.emit("dblclick")
	case 0x0205: // WM_RBUTTONUP
		t.showMenu()
	}
}

func (a *App) onTaskbarCreated() {
	if a.tray != nil {
		a.tray.reAdd()
	}
}

// Kind implements lang.Widget.
func (t *Tray) Kind() string { return "tray" }

func (t *Tray) GetProp(name string) (lang.Value, bool) {
	switch name {
	case "tooltip", "text":
		return t.tooltip, true
	case "icon", "icon_path":
		return t.iconPath, true
	case "visible", "added":
		return t.added, true
	case "last_error":
		return t.lastErr, true
	}
	return nil, false
}

func (t *Tray) SetProp(name string, v lang.Value) error {
	switch name {
	case "tooltip", "text":
		t.tooltip = lang.ToString(v)
		return t.modify()
	case "icon", "icon_path":
		return t.setIcon(lang.ToString(v))
	case "visible":
		if lang.Truthy(v) {
			if !t.added {
				return t.add()
			}
			return nil
		}
		t.remove()
		return nil
	}
	return fmt.Errorf("tray 不支持属性 %q", name)
}

func (t *Tray) Listen(event string, fn func() error) error {
	switch event {
	case "click", "dblclick", "menu":
	default:
		return fmt.Errorf("tray 不支持事件 %q", event)
	}
	t.handlers[event] = append(t.handlers[event], fn)
	return nil
}

func (t *Tray) emit(event string) {
	for _, fn := range t.handlers[event] {
		if err := fn(); err != nil {
			t.app.reportError(err)
		}
	}
}

func (t *Tray) CallMethod(name string, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, bool, error) {
	switch name {
	case "item", "add_item":
		if t.menu == nil {
			return nil, true, fmt.Errorf("tray 菜单不可用")
		}
		if len(args) < 1 || len(args) > 2 {
			return nil, true, fmt.Errorf("tray.item(文字, 回调?) 接受 1 到 2 个参数")
		}
		var fn lang.Value
		if len(args) == 2 {
			fn = args[1]
		}
		item, err := t.menu.addItem(lang.ToString(args[0]), fn)
		return item, true, err
	case "separator":
		if t.menu == nil {
			return nil, true, fmt.Errorf("tray 菜单不可用")
		}
		pAppendMenuW.Call(uintptr(t.menu.hmenu), uintptr(mfSeparator), 0, 0)
		return nil, true, nil
	case "notify", "balloon":
		if len(args) < 1 || len(args) > 3 {
			return nil, true, fmt.Errorf("tray.notify(标题, 内容?, warning?) 接受 1 到 3 个参数")
		}
		title := lang.ToString(args[0])
		text := ""
		if len(args) >= 2 {
			text = lang.ToString(args[1])
		}
		warn := false
		if len(args) == 3 {
			warn = lang.Truthy(args[2])
		}
		return nil, true, t.notify(title, text, warn)
	case "set_icon":
		if len(args) != 1 {
			return nil, true, fmt.Errorf("tray.set_icon(路径) 需要 1 个参数")
		}
		return nil, true, t.setIcon(lang.ToString(args[0]))
	case "set_tooltip":
		if len(args) != 1 {
			return nil, true, fmt.Errorf("tray.set_tooltip(文字) 需要 1 个参数")
		}
		t.tooltip = lang.ToString(args[0])
		return nil, true, t.modify()
	case "minimize_to_tray":
		v := true
		if len(args) == 1 {
			v = lang.Truthy(args[0])
		}
		t.app.minimizeToTray = v
		return nil, true, nil
	case "remove", "hide":
		t.remove()
		return nil, true, nil
	case "show":
		if !t.added {
			return nil, true, t.add()
		}
		return nil, true, t.modify()
	}
	return nil, false, nil
}
