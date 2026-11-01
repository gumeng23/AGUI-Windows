package gui

import (
	"fmt"
	"syscall"
	"unsafe"
)

type inputState struct {
	app    *App
	hwnd   syscall.Handle
	edit   syscall.Handle
	done   bool
	ok     bool
	result string
}

var inputCurrent *inputState
var dialogProcCallback = syscall.NewCallback(dialogProc)

func (a *App) showInput(title, prompt, initial string) (string, bool) {
	w := a.px(480)
	h := a.px(210)
	owner := a.hwnd
	x, y := a.centeredPosition(owner, w, h)
	hwnd := createWindow(wsExDialogModalFrame, dialogClassName, title, wsPopup|wsCaption|wsSysMenu, x, y, w, h, owner, 0)
	if hwnd == 0 {
		return "", false
	}
	st := &inputState{app: a, hwnd: hwnd}
	inputCurrent = st
	defer func() { inputCurrent = nil }()

	pad := a.px(16)
	labelW := w - pad*2
	labelH := a.px(44)
	promptHwnd := createWindow(0, "STATIC", prompt, wsChild|wsVisible|ssLeft|ssNoPrefix, pad, a.px(14), labelW, labelH, hwnd, 0)
	editHwnd := createWindow(0, "EDIT", initial, wsChild|wsVisible|wsBorder|esAutoHScroll|wsTabStop,
		pad, a.px(62), labelW, a.px(32), hwnd, 0)
	okHwnd := createWindow(0, "BUTTON", "确定", wsChild|wsVisible|wsTabStop|bsPushButton,
		w-a.px(200), a.px(118), a.px(80), a.px(32), hwnd, idOK)
	cancelHwnd := createWindow(0, "BUTTON", "取消", wsChild|wsVisible|wsTabStop|bsPushButton,
		w-a.px(110), a.px(118), a.px(80), a.px(32), hwnd, idCancel)
	if promptHwnd == 0 || editHwnd == 0 || okHwnd == 0 || cancelHwnd == 0 {
		pDestroyWindow.Call(uintptr(hwnd))
		return "", false
	}
	for _, child := range []syscall.Handle{promptHwnd, editHwnd, okHwnd, cancelHwnd} {
		sendMessage(child, wmSetFont, uintptr(a.hFont), 1)
	}
	st.edit = editHwnd
	setWindowText(editHwnd, initial)
	sendMessage(editHwnd, emSetSel, 0, ^uintptr(0))
	if owner != 0 {
		enableWindow(owner, false)
	}
	showWindow(hwnd, swShow)
	pUpdateWindow.Call(uintptr(hwnd))
	setFocus(editHwnd)

	var msg msgT
	for !st.done {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if msg.message == wmKeyDown {
			switch msg.wParam {
			case vkReturn:
				finishInput(true)
				continue
			case vkEscape:
				finishInput(false)
				continue
			}
		}
		if r, _, _ := pIsDialogMessageW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&msg))); r != 0 {
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
	if owner != 0 {
		enableWindow(owner, true)
		pSetForegroundWindow.Call(uintptr(owner))
	}
	return st.result, st.ok
}

func (a *App) centeredPosition(owner syscall.Handle, w, h int32) (int32, int32) {
	if owner != 0 {
		r, ok := getWindowRect(owner)
		if ok {
			return r.left + (r.right-r.left-w)/2, r.top + (r.bottom-r.top-h)/2
		}
	}
	sw, sh := screenSize()
	return (sw - w) / 2, (sh - h) / 2
}

func dialogProc(hwnd, msg, wparam, lparam uintptr) (ret uintptr) {
	h := syscall.Handle(hwnd)
	st := inputCurrent
	defer func() {
		if r := recover(); r != nil {
			ret = defWindowProc(h, uint32(msg), wparam, lparam)
		}
	}()
	switch uint32(msg) {
	case wmCommand:
		if st == nil || st.hwnd != h {
			return 0
		}
		id := int(wparam & 0xFFFF)
		switch id {
		case idOK:
			finishInput(true)
		case idCancel:
			finishInput(false)
		}
		return 0
	case wmClose:
		finishInput(false)
		return 0
	case wmKeyDown:
		switch int(wparam) {
		case vkReturn:
			finishInput(true)
			return 0
		case vkEscape:
			finishInput(false)
			return 0
		}
	case wmEraseBkgnd:
		if st != nil {
			var cr rect
			if ok, _, _ := pGetClientRect.Call(uintptr(h), uintptr(unsafe.Pointer(&cr))); ok != 0 {
				fillRect(syscall.Handle(wparam), &cr, st.app.whiteBrush)
			}
			return 1
		}
	case wmCtlColorStatic:
		if st != nil {
			setBkMode(syscall.Handle(wparam), 1)
			setTextColor(syscall.Handle(wparam), 0x1E1E1E)
			return uintptr(st.app.whiteBrush)
		}
	}
	return defWindowProc(h, uint32(msg), wparam, lparam)
}

func finishInput(ok bool) {
	st := inputCurrent
	if st == nil || st.done {
		return
	}
	st.done = true
	if ok {
		st.result = getWindowText(st.edit)
		st.ok = true
	}
	pDestroyWindow.Call(uintptr(st.hwnd))
}

func (a *App) openFile(filter, title string) (string, bool, error) {
	buf := make([]uint16, 4096)
	filter16 := filterSpec(filter)
	ofn := openFileNameW{
		structSize: uint32(unsafe.Sizeof(openFileNameW{})),
		hwndOwner:  a.hwnd,
		filter:     &filter16[0],
		file:       &buf[0],
		maxFile:    uint32(len(buf)),
		title:      u16(title),
		flags:      ofnExplorer | ofnFileMustExist | ofnPathMustExist | ofnNoChangeDir,
	}
	r, _, _ := pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		if errCode, _, _ := pCommDlgExtendedError.Call(); errCode != 0 {
			return "", false, fmt.Errorf("打开文件对话框失败 (0x%X)", errCode)
		}
		return "", false, nil
	}
	return syscall.UTF16ToString(buf), true, nil
}

func (a *App) saveFile(filter, title, defaultName string) (string, bool, error) {
	buf := make([]uint16, 4096)
	if defaultName != "" {
		copy(buf, syscall.StringToUTF16(defaultName))
	}
	filter16 := filterSpec(filter)
	ofn := openFileNameW{
		structSize: uint32(unsafe.Sizeof(openFileNameW{})),
		hwndOwner:  a.hwnd,
		filter:     &filter16[0],
		file:       &buf[0],
		maxFile:    uint32(len(buf)),
		title:      u16(title),
		flags:      ofnExplorer | ofnOverwritePrompt | ofnPathMustExist | ofnNoChangeDir,
	}
	r, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		if errCode, _, _ := pCommDlgExtendedError.Call(); errCode != 0 {
			return "", false, fmt.Errorf("保存文件对话框失败 (0x%X)", errCode)
		}
		return "", false, nil
	}
	return syscall.UTF16ToString(buf), true, nil
}
