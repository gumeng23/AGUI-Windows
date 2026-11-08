package gui

import (
	"syscall"
	"unsafe"
)

// panelWndProc handles the custom container window used by panel/scroll/grid.
// Child controls are parented to it, so commands and notifications are
// forwarded to the owning top-level window.
func panelWndProc(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) uintptr {
	a := currentApp
	if a == nil {
		return defWindowProc(hwnd, msg, wparam, lparam)
	}
	w := a.widgetByHWND(hwnd)
	switch msg {
	case wmGetObject:
		if lr := handleGetObject(uintptr(hwnd), wparam); lr != 0 {
			return lr
		}
	case wmLButtonDown:
		if w != nil && w.kind == "splitter" {
			x := int32(int16(lparam & 0xFFFF))
			y := int32(int16((lparam >> 16) & 0xFFFF))
			if w.startSplitDrag(x, y) {
				return 0
			}
		}
	case wmMouseMove:
		if w != nil && w.kind == "splitter" && w.splitDrag {
			x := int32(int16(lparam & 0xFFFF))
			y := int32(int16((lparam >> 16) & 0xFFFF))
			w.updateSplitDrag(x, y)
			return 0
		}
	case wmLButtonUp:
		if w != nil && w.kind == "splitter" && w.splitDrag {
			w.endSplitDrag()
			return 0
		}
	case wmEraseBkgnd:
		if w != nil {
			hdc := syscall.Handle(wparam)
			if rc, ok := getClientRect(hwnd); ok {
				brush := w.bgBrush
				if brush == 0 {
					brush = a.bgBrush
				}
				fillRect(hdc, &rc, brush)
			}
			return 1
		}
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := pBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		if w != nil {
			if rc, ok := getClientRect(hwnd); ok {
				brush := w.bgBrush
				if brush == 0 {
					brush = a.bgBrush
				}
				fillRect(syscall.Handle(hdc), &rc, brush)
			}
		}
		pEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmVScroll:
		if w != nil {
			a.scrollPanel(w, wparam)
			return 0
		}
	case wmMouseWheel:
		if w != nil && w.kind == "scroll" {
			delta := int16(wparam >> 16)
			w.scrollY -= int32(delta) / 120 * a.px(30)
			a.layoutContainer(w, w.w, w.h)
			return 0
		}
	case wmSize:
		if w != nil && isContainerKind(w.kind) {
			if rc, ok := getClientRect(hwnd); ok {
				a.layoutContainer(w, rc.right, rc.bottom)
			}
		}
	case wmCommand, wmNotify:
		if w != nil {
			if top := w.topWindow(); top != nil && top.hwnd != hwnd {
				return sendMessage(top.hwnd, uint32(msg), wparam, lparam)
			}
		}
	}
	return defWindowProc(hwnd, msg, wparam, lparam)
}

var panelProcCallback = syscall.NewCallback(panelWndProc)

// groupboxProc forwards child commands and notifications (for example a
// button placed inside a group box) to the owning top-level window. A plain
// BUTTON group box otherwise swallows WM_COMMAND from its children.
func groupboxProc(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) uintptr {
	if msg == wmCommand || msg == wmNotify {
		if a := currentApp; a != nil {
			if w := a.widgetByHWND(hwnd); w != nil {
				if top := w.topWindow(); top != nil {
					return sendMessage(top.hwnd, msg, wparam, lparam)
				}
			}
		}
	}
	w := (*Widget)(nil)
	if a := currentApp; a != nil {
		w = a.widgetByHWND(hwnd)
	}
	old := uintptr(0)
	if w != nil {
		old = w.groupOldProc
	}
	if old == 0 {
		return defWindowProc(hwnd, msg, wparam, lparam)
	}
	r, _, _ := pCallWindowProcW.Call(old, uintptr(hwnd), uintptr(msg), wparam, lparam)
	return r
}

var groupboxProcCallback = syscall.NewCallback(groupboxProc)
