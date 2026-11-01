package gui

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"agui/internal/lang"
)

// Additional Win32 controls and window-management helpers.

const (
	bsAutoRadioButton = 0x00000009
	sbarsSizeGrip     = 0x00000100

	sbSetText  = 0x040B
	sbSetParts = 0x0404

	tbsAutoTicksStatus = 0

	dtsShortDate = 0x0000
	dtsTime      = 0x0009

	dtmSetSystemTime = 0x1002
	dtmGetSystemTime = 0x1000

	dtnDateTimeChange = 0xFFFFFD09

	wsExTopmost = 0x00000008
	wsExLayered = 0x00080000
	gwlExStyle  = -20

	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010

	lwaAlpha = 0x00000002

	flashwAll         = 0x00000003
	flashwTimerNoFG   = 0x0000000C
	ttmActivate       = 0x0401
	ttmGetToolCount   = 0x040D
	ttmSetDelayTime   = 0x0403
	ttmAddToolW       = 0x0432
	ttmSetMaxTipWidth = 0x0418
	ttmUpdate         = 0x041D
	ttmPopup          = 0x041E
	ttmTrackActivate  = 0x0411
	ttmTrackPosition  = 0x0412
	ttfTrack          = 0x20
	ttfAbsolute       = 0x80
	ttfIDishWnd       = 0x0001
	ttfSubclass       = 0x0010
	tooltipClass      = "tooltips_class32"
)

var (
	pSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	pFlashWindowEx              = user32.NewProc("FlashWindowEx")
	pIsZoomed                   = user32.NewProc("IsZoomed")
	pIsIconic                   = user32.NewProc("IsIconic")
)

type systemTimeW struct {
	Year         uint16
	Month        uint16
	DayOfWeek    uint16
	Day          uint16
	Hour         uint16
	Minute       uint16
	Second       uint16
	Milliseconds uint16
}

type flashInfoW struct {
	cbSize    uint32
	hwnd      syscall.Handle
	dwFlags   uint32
	uCount    uint32
	dwTimeout uint32
}

type toolInfoW struct {
	cbSize   uint32
	uFlags   uint32
	hwnd     syscall.Handle
	uID      uintptr
	hinst    syscall.Handle
	lpszText *uint16
	lParam   uintptr
}

func (a *App) createRadio(text string, checked bool, x, y, w, h *int32) (*Widget, error) {
	style := uint32(bsAutoRadioButton | wsTabStop)
	widget, err := a.addChild("radio", "BUTTON", text, style, false, a.px(180), a.px(28), x, y, w, h)
	if err != nil {
		return nil, err
	}
	if checked {
		sendMessage(widget.hwnd, bmSetCheck, bstChecked, 0)
	}
	return widget, nil
}

func (a *App) createStatusbar(text string, x, y, w, h *int32) (*Widget, error) {
	widget, err := a.addChild("statusbar", "msctls_statusbar32", text, sbarsSizeGrip, true, 0, a.px(28), x, y, w, h)
	if err != nil {
		return nil, err
	}
	widget.title = text
	if text != "" {
		widget.setStatusParts(strings.Split(text, "\t"))
	}
	return widget, nil
}

func (a *App) createDate(kind, value string, x, y, w, h *int32) (*Widget, error) {
	style := uint32(dtsShortDate | wsTabStop)
	if kind == "time" {
		style = uint32(dtsTime | wsTabStop)
	}
	widget, err := a.addChild(kind, "SysDateTimePick32", "", style, false, a.px(180), a.px(30), x, y, w, h)
	if err != nil {
		return nil, err
	}
	widget.dateValue = time.Now()
	if value == "" {
		value = widget.dateText()
	}
	if err := widget.setDateText(value); err != nil {
		return nil, err
	}
	return widget, nil
}

func (w *Widget) setStatusParts(parts []string) {
	if len(parts) == 0 {
		return
	}
	w.statusParts = append([]string(nil), parts...)
	bounds := make([]int32, len(parts))
	width := w.w
	if width <= 0 {
		width = w.app.px(600)
	}
	for i := range parts {
		bounds[i] = int32(float64(width) * float64(i+1) / float64(len(parts)))
	}
	sendMessage(w.hwnd, sbSetParts, uintptr(len(bounds)), uintptr(unsafe.Pointer(&bounds[0])))
	for i, part := range parts {
		sendMessage(w.hwnd, sbSetText, uintptr(i), uintptr(unsafe.Pointer(u16(part))))
	}
	w.title = strings.Join(parts, "\t")
}

func (w *Widget) statusPartsValue() *lang.List {
	out := &lang.List{Items: make([]lang.Value, len(w.statusParts))}
	for i, part := range w.statusParts {
		out.Items[i] = part
	}
	return out
}

func (w *Widget) setDateText(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	layouts := []string{"2006-01-02", "2006/01/02", "2006-01-02 15:04:05", "2006/01/02 15:04:05", "15:04:05", "15:04"}
	var parsed time.Time
	var ok bool
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, text, time.Local); err == nil {
			parsed, ok = t, true
			break
		}
	}
	if !ok {
		return fmt.Errorf("无法解析日期时间 %q（示例：2026-11-01 或 15:04:05）", text)
	}
	if parsed.Year() == 0 {
		now := time.Now()
		parsed = time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, time.Local)
	}
	w.dateValue = parsed
	st := systemTimeW{
		Year: uint16(parsed.Year()), Month: uint16(parsed.Month()), Day: uint16(parsed.Day()),
		Hour: uint16(parsed.Hour()), Minute: uint16(parsed.Minute()), Second: uint16(parsed.Second()),
	}
	sendMessage(w.hwnd, dtmSetSystemTime, 0, uintptr(unsafe.Pointer(&st)))
	return nil
}

func (w *Widget) syncDateFromControl() {
	var st systemTimeW
	sendMessage(w.hwnd, dtmGetSystemTime, 0, uintptr(unsafe.Pointer(&st)))
	if st.Year == 0 {
		return
	}
	w.dateValue = time.Date(int(st.Year), time.Month(st.Month), int(st.Day),
		int(st.Hour), int(st.Minute), int(st.Second), 0, time.Local)
}

func (w *Widget) dateText() string {
	if w.kind == "time" {
		return w.dateValue.Format("15:04:05")
	}
	return w.dateValue.Format("2006-01-02")
}

func (w *Widget) setTooltip(text string) {
	w.tooltip = text
	if w.app.tooltipHwnd == 0 {
		tip := createWindow(0, tooltipClass, "", wsPopup|0x01|0x02, 0, 0, 0, 0, 0, 0)
		if tip == 0 {
			return
		}
		w.app.tooltipHwnd = tip
		sendMessage(tip, ttmSetMaxTipWidth, 0, uintptr(w.app.px(400)))
		sendMessage(tip, ttmSetDelayTime, 300, 0)
		sendMessage(tip, ttmActivate, 1, 0)
	}
	buf, _ := syscall.UTF16FromString(text)
	hwnds := w.app.tooltipText
	hwnds[w.hwnd] = buf
	ti := toolInfoW{
		cbSize:   uint32(unsafe.Sizeof(toolInfoW{})),
		uFlags:   ttfIDishWnd | ttfSubclass,
		hwnd:     w.hwnd,
		uID:      uintptr(w.hwnd),
		lpszText: &buf[0],
	}
	sendMessage(w.app.tooltipHwnd, ttmAddToolW, 0, uintptr(unsafe.Pointer(&ti)))
	sendMessage(w.app.tooltipHwnd, ttmUpdate, 0, 0)
}

// hoverTooltip sends a synthetic mouse-move to the control so the native
// tooltip subclasses can start their hover timer. It is mostly useful for
// automated UI tests; normal users just move the mouse.
func (w *Widget) hoverTooltip() {
	if w.hwnd == 0 {
		return
	}
	x := int32(w.w / 2)
	y := int32(w.h / 2)
	lp := uintptr(uint16(x)) | uintptr(uint16(y))<<16
	sendMessage(w.hwnd, 0x0200, 0, lp) // WM_MOUSEMOVE
	w.showTooltip()
}

func (w *Widget) showTooltip() {
	if w.app.tooltipHwnd == 0 || w.tooltip == "" {
		return
	}
	buf := w.app.tooltipText[w.hwnd]
	if len(buf) == 0 {
		return
	}
	w.tooltipInfo = toolInfoW{
		cbSize:   uint32(unsafe.Sizeof(toolInfoW{})),
		uFlags:   ttfIDishWnd | ttfTrack | ttfAbsolute,
		hwnd:     w.hwnd,
		uID:      uintptr(w.hwnd),
		lpszText: &buf[0],
	}
	if !w.tooltipTracked {
		sendMessage(w.app.tooltipHwnd, ttmAddToolW, 0, uintptr(unsafe.Pointer(&w.tooltipInfo)))
		w.tooltipTracked = true
	}
	r, ok := getWindowRect(w.hwnd)
	if !ok {
		return
	}
	x := (r.left + r.right) / 2
	y := r.bottom
	pos := uintptr(uint16(x)) | uintptr(uint16(y))<<16
	sendMessage(w.app.tooltipHwnd, ttmTrackPosition, 0, pos)
	sendMessage(w.app.tooltipHwnd, ttmTrackActivate, 1, uintptr(unsafe.Pointer(&w.tooltipInfo)))
	sendMessage(w.app.tooltipHwnd, ttmPopup, 0, 0)
}

func (w *Widget) hideTooltip() {
	if w.app.tooltipHwnd == 0 || !w.tooltipTracked {
		return
	}
	sendMessage(w.app.tooltipHwnd, ttmTrackActivate, 0, uintptr(unsafe.Pointer(&w.tooltipInfo)))
}

func (w *Widget) tooltipVisible() bool {
	if w.app.tooltipHwnd == 0 {
		return false
	}
	return isWindowVisible(w.app.tooltipHwnd)
}

func (w *Widget) centerWindow() {
	r, ok := getWindowRect(w.hwnd)
	if !ok {
		return
	}
	sw, _, _ := pGetSystemMetrics.Call(smCXScreen)
	sh, _, _ := pGetSystemMetrics.Call(smCYScreen)
	width := r.right - r.left
	height := r.bottom - r.top
	x := (int32(sw) - width) / 2
	y := (int32(sh) - height) / 2
	pSetWindowPos.Call(uintptr(w.hwnd), 0, uintptr(x), uintptr(y), 0, 0,
		uintptr(swpNoSize|swpNoZOrder|swpNoActivate))
}

func (w *Widget) setTopmost(on bool) {
	after := uintptr(^uintptr(1)) // HWND_NOTOPMOST
	if on {
		after = ^uintptr(0) // HWND_TOPMOST
	}
	pSetWindowPos.Call(uintptr(w.hwnd), after, 0, 0, 0, 0,
		uintptr(swpNoMove|swpNoSize|swpNoActivate))
}

func (w *Widget) isTopmost() bool {
	return getWindowLong(w.hwnd, gwlExStyle)&wsExTopmost != 0
}

func (w *Widget) setOpacity(alpha float64) error {
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	w.opacity = alpha
	ex := getWindowLong(w.hwnd, gwlExStyle)
	if alpha >= 1 {
		setWindowLong(w.hwnd, gwlExStyle, ex&^wsExLayered)
		return nil
	}
	setWindowLong(w.hwnd, gwlExStyle, ex|wsExLayered)
	pSetLayeredWindowAttributes.Call(uintptr(w.hwnd), 0, uintptr(uint8(alpha*255)), lwaAlpha)
	return nil
}

func (w *Widget) windowState() string {
	if r, _, _ := pIsZoomed.Call(uintptr(w.hwnd)); r != 0 {
		return "maximized"
	}
	if r, _, _ := pIsIconic.Call(uintptr(w.hwnd)); r != 0 {
		return "minimized"
	}
	return "normal"
}

func (w *Widget) setWindowState(state string) error {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "normal", "restore", "normal窗口":
		pShowWindow.Call(uintptr(w.hwnd), swRestore)
	case "max", "maximized", "最大化":
		pShowWindow.Call(uintptr(w.hwnd), swMaximize)
	case "min", "minimized", "最小化":
		pShowWindow.Call(uintptr(w.hwnd), swMinimize)
	default:
		return fmt.Errorf("未知窗口状态 %q（可用 normal/maximized/minimized）", state)
	}
	return nil
}

func (w *Widget) flashWindow() {
	fi := flashInfoW{
		cbSize: uint32(unsafe.Sizeof(flashInfoW{})), hwnd: w.hwnd,
		dwFlags: flashwAll | flashwTimerNoFG, uCount: 3,
	}
	pFlashWindowEx.Call(uintptr(unsafe.Pointer(&fi)))
}
