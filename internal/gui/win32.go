package gui

import (
	"fmt"
	"syscall"
	"unsafe"
)

// This file holds the thin Win32 bindings used by Agui. Everything is done with
// plain syscalls so the final executable stays a single, dependency-free file.

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	comctl32 = syscall.NewLazyDLL("comctl32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
)

var (
	pRegisterClassExW         = user32.NewProc("RegisterClassExW")
	pCreateWindowExW          = user32.NewProc("CreateWindowExW")
	pDefWindowProcW           = user32.NewProc("DefWindowProcW")
	pDestroyWindow            = user32.NewProc("DestroyWindow")
	pShowWindow               = user32.NewProc("ShowWindow")
	pUpdateWindow             = user32.NewProc("UpdateWindow")
	pGetMessageW              = user32.NewProc("GetMessageW")
	pTranslateMessage         = user32.NewProc("TranslateMessage")
	pDispatchMessageW         = user32.NewProc("DispatchMessageW")
	pPostQuitMessage          = user32.NewProc("PostQuitMessage")
	pPostMessageW             = user32.NewProc("PostMessageW")
	pSendMessageW             = user32.NewProc("SendMessageW")
	pSetWindowTextW           = user32.NewProc("SetWindowTextW")
	pGetWindowTextW           = user32.NewProc("GetWindowTextW")
	pGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	pGetClientRect            = user32.NewProc("GetClientRect")
	pGetWindowRect            = user32.NewProc("GetWindowRect")
	pMoveWindow               = user32.NewProc("MoveWindow")
	pSetWindowPos             = user32.NewProc("SetWindowPos")
	pSetFocus                 = user32.NewProc("SetFocus")
	pGetFocus                 = user32.NewProc("GetFocus")
	pEnableWindow             = user32.NewProc("EnableWindow")
	pIsWindowVisible          = user32.NewProc("IsWindowVisible")
	pIsWindowEnabled          = user32.NewProc("IsWindowEnabled")
	pIsDialogMessageW         = user32.NewProc("IsDialogMessageW")
	pAdjustWindowRectEx       = user32.NewProc("AdjustWindowRectEx")
	pGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	pSetProcessDPIAware       = user32.NewProc("SetProcessDPIAware")
	pGetDpiForSystem          = user32.NewProc("GetDpiForSystem")
	pGetDC                    = user32.NewProc("GetDC")
	pReleaseDC                = user32.NewProc("ReleaseDC")
	pFillRect                 = user32.NewProc("FillRect")
	pSetTimer                 = user32.NewProc("SetTimer")
	pKillTimer                = user32.NewProc("KillTimer")
	pInvalidateRect           = user32.NewProc("InvalidateRect")
	pMessageBoxW              = user32.NewProc("MessageBoxW")
	pLoadCursorW              = user32.NewProc("LoadCursorW")
	pLoadIconW                = user32.NewProc("LoadIconW")
	pSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	pSetCapture               = user32.NewProc("SetCapture")
	pReleaseCapture           = user32.NewProc("ReleaseCapture")
	pGetWindowDC              = user32.NewProc("GetWindowDC")
	pPrintWindow              = user32.NewProc("PrintWindow")
	pMapWindowPoints          = user32.NewProc("MapWindowPoints")
	pGetParent                = user32.NewProc("GetParent")
	pSetWindowTheme           = user32.NewProc("SetWindowTheme")
	pGetWindowLongPtrW        = user32.NewProc("GetWindowLongPtrW")
	pSetWindowLongPtrW        = user32.NewProc("SetWindowLongPtrW")
	pCallWindowProcW          = user32.NewProc("CallWindowProcW")
	pDrawTextW                = user32.NewProc("DrawTextW")
	pDrawEdge                 = user32.NewProc("DrawEdge")
	pDrawFocusRect            = user32.NewProc("DrawFocusRect")
	pFrameRect                = user32.NewProc("FrameRect")
	pBeginPaint               = user32.NewProc("BeginPaint")
	pEndPaint                 = user32.NewProc("EndPaint")
	pSetScrollRange           = user32.NewProc("SetScrollRange")
	pSetScrollPos             = user32.NewProc("SetScrollPos")
	pIsWindow                 = user32.NewProc("IsWindow")
	pGetClipboardData         = user32.NewProc("GetClipboardData")
	pSetClipboardData         = user32.NewProc("SetClipboardData")
	pRegisterClipboardFormatW = user32.NewProc("RegisterClipboardFormatW")
	pOpenClipboard            = user32.NewProc("OpenClipboard")
	pCloseClipboard           = user32.NewProc("CloseClipboard")
	pEmptyClipboard           = user32.NewProc("EmptyClipboard")
	pIsClipboardFormatAvail   = user32.NewProc("IsClipboardFormatAvailable")
	pDragAcceptFiles          = shell32.NewProc("DragAcceptFiles")
	pDragQueryFileW           = shell32.NewProc("DragQueryFileW")
	pDragFinish               = shell32.NewProc("DragFinish")
	pShellExecuteW            = shell32.NewProc("ShellExecuteW")

	pCreateFontW            = gdi32.NewProc("CreateFontW")
	pCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	pCreatePen              = gdi32.NewProc("CreatePen")
	pMoveToEx               = gdi32.NewProc("MoveToEx")
	pLineTo                 = gdi32.NewProc("LineTo")
	pRectangle              = gdi32.NewProc("Rectangle")
	pEllipse                = gdi32.NewProc("Ellipse")
	pTextOutW               = gdi32.NewProc("TextOutW")
	pDeleteObject           = gdi32.NewProc("DeleteObject")
	pSetBkMode              = gdi32.NewProc("SetBkMode")
	pSetTextColor           = gdi32.NewProc("SetTextColor")
	pSetBkColor             = gdi32.NewProc("SetBkColor")
	pGetStockObject         = gdi32.NewProc("GetStockObject")
	pCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	pSelectObject           = gdi32.NewProc("SelectObject")
	pBitBlt                 = gdi32.NewProc("BitBlt")
	pDeleteDC               = gdi32.NewProc("DeleteDC")
	pGetTextExtentPoint32W  = gdi32.NewProc("GetTextExtentPoint32W")
	pGetDIBits              = gdi32.NewProc("GetDIBits")
	pGetDeviceCaps          = gdi32.NewProc("GetDeviceCaps")
	pCreateDIBSection       = gdi32.NewProc("CreateDIBSection")
	pSetStretchBltMode      = gdi32.NewProc("SetStretchBltMode")
	pStretchBlt             = gdi32.NewProc("StretchBlt")
	pCreateBitmap           = gdi32.NewProc("CreateBitmap")
	pCreateIconIndirect     = user32.NewProc("CreateIconIndirect")
	pDestroyIcon            = user32.NewProc("DestroyIcon")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	pGlobalAlloc      = kernel32.NewProc("GlobalAlloc")
	pGlobalFree       = kernel32.NewProc("GlobalFree")
	pGlobalLock       = kernel32.NewProc("GlobalLock")
	pGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
	pGlobalSize       = kernel32.NewProc("GlobalSize")
	pRtlMoveMemory    = kernel32.NewProc("RtlMoveMemory")
	pCreateActCtxW    = kernel32.NewProc("CreateActCtxW")
	pActivateActCtx   = kernel32.NewProc("ActivateActCtx")
	pDeactivateActCtx = kernel32.NewProc("DeactivateActCtx")
	pReleaseActCtx    = kernel32.NewProc("ReleaseActCtx")

	pInitCommonControlsEx = comctl32.NewProc("InitCommonControlsEx")

	pGetOpenFileNameW     = comdlg32.NewProc("GetOpenFileNameW")
	pGetSaveFileNameW     = comdlg32.NewProc("GetSaveFileNameW")
	pCommDlgExtendedError = comdlg32.NewProc("CommDlgExtendedError")
)

const (
	wsOverlappedWindow = 0x00CF0000
	wsPopup            = 0x80000000
	wsChild            = 0x40000000
	wsVisible          = 0x10000000
	wsCaption          = 0x00C00000
	wsSysMenu          = 0x00080000
	wsThickFrame       = 0x00040000
	wsMinimizeBox      = 0x00020000
	wsMaximizeBox      = 0x00010000
	wsBorder           = 0x00800000
	wsVScroll          = 0x00200000
	wsTabStop          = 0x00010000
	wsClipChildren     = 0x02000000

	wsExDialogModalFrame = 0x00000001
	wsExControlParent    = 0x00010000
	wsExAppWindow        = 0x00040000

	cwUseDefault = -2147483648

	swHide       = 0
	swShow       = 5
	swMinimize   = 6
	swMaximize   = 3
	swRestore    = 9
	swShowNormal = 1

	wmDestroy         = 0x0002
	wmSize            = 0x0005
	wmPaint           = 0x000F
	wmPaste           = 0x0302
	wmSetIcon         = 0x0080
	iconSmall         = 0
	iconBig           = 1
	wmLButtonDown     = 0x0201
	wmLButtonUp       = 0x0202
	wmRButtonDown     = 0x0204
	wmRButtonUp       = 0x0205
	wmMButtonDown     = 0x0207
	wmMButtonUp       = 0x0208
	wmMouseMove       = 0x0200
	wmDblClk          = 0x0203
	wmSetFocus        = 0x0007
	wmKillFocus       = 0x0008
	wmClose           = 0x0010
	wmQuit            = 0x0012
	wmEraseBkgnd      = 0x0014
	wmSetFont         = 0x0030
	wmGetTextLength   = 0x000E
	wmSetText         = 0x000C
	wmGetText         = 0x000D
	wmGetFont         = 0x0031
	wmCommand         = 0x0111
	wmTimer           = 0x0113
	wmHScroll         = 0x0114
	wmVScroll         = 0x0115
	wmMouseWheel      = 0x020A
	sbLineUp          = 0
	sbLineDown        = 1
	sbPageUp          = 2
	sbPageDown        = 3
	sbThumbTrack      = 5
	sbThumbPosition   = 4
	sbTop             = 6
	sbBottom          = 7
	sbVert            = 1
	wmCtlColorBtn     = 0x0135
	wmCtlColorEdit    = 0x0133
	wmCtlColorListBox = 0x0134
	wmCtlColorStatic  = 0x0138
	wmGetMinMaxInfo   = 0x0024
	wmDrawItem        = 0x002B
	psSolid           = 0
	nullBrush         = 5
	transparent       = 1
	wmNotify          = 0x004E
	wmKeyDown         = 0x0100
	wmKeyUp           = 0x0101
	wmSysKeyDown      = 0x0104
	wmSysKeyUp        = 0x0105
	wmDropFiles       = 0x0233
	wmSetCursor       = 0x0020
	swpNoOwnerZOrder  = 0x0200
	swpFrameChanged   = 0x0020
	wmApp             = 0x8000
	wmAppAsync        = 0x8001
	wmNull            = 0x0000

	bnClicked     = 0
	bsOwnerDraw   = 0x0000000B
	gwlStyle      = -16
	gwlWndProc    = -4
	odtButton     = 4
	odsSelected   = 0x0001
	odsDisabled   = 0x0004
	odsFocus      = 0x0010
	odsDefault    = 0x0020
	dtCenter      = 0x0001
	dtVCenter     = 0x0004
	dtSingleLine  = 0x0020
	dtEndEllipsis = 0x8000
	edgeRaised    = 5
	edgeSunken    = 10
	bfRect        = 0x000F
	enChange      = 0x0300
	lbnSelChange  = 1
	lbnDblClk     = 2
	cbnSelChange  = 1
	cbnDblClk     = 2
	cbnEditChange = 5
	stnClicked    = 0
	stnDblClk     = 1

	bsPushButton   = 0x00000000
	bsGroupBox     = 0x00000007
	bsAutoCheckBox = 0x00000003

	esMultiLine   = 0x0004
	esAutoVScroll = 0x0040
	esAutoHScroll = 0x0080
	esReadOnly    = 0x0800

	lbsNotify           = 0x0001
	lbsNoIntegralHeight = 0x0100

	cbsDropDownList = 0x0003

	pbsSmooth = 0x0001

	tbsAutoTicks = 0x0001

	ssLeft     = 0x0000
	ssNoPrefix = 0x0080
	ssNotify   = 0x0100

	bmSetCheck = 0x00F1
	bmGetCheck = 0x00F0
	bmClick    = 0x00F5
	bstChecked = 1

	emSetSel          = 0x00B1
	emGetSel          = 0x00B0
	emScrollCaret     = 0x00B7
	emReplaceSel      = 0x00C2
	emSetEventMask    = 0x0445
	emStreamIn        = 0x0449
	emPasteSpecial    = 0x0440
	emSetReadOnly     = 0x00CF
	emSetPasswordChar = 0x00CC
	emSetCharFormat   = 0x0444
	emSetParaFormat   = 0x0447
	emSetBkColor      = 0x0443
	emGetCharFormat   = 0x0434
	scfSelection      = 0x0001
	scfAll            = 0x0004
	cfmBold           = 0x00000001
	cfmItalic         = 0x00000002
	cfmUnderline      = 0x00000004
	cfmBackColor      = 0x04000000
	cfmColor          = 0x40000000
	cfmFace           = 0x20000000
	cfmSize           = 0x80000000
	cfmWeight         = 0x00400000
	cfmLink           = 0x00000020
	cfeLink           = 0x00000020
	enmLink           = 0x04000000
	sfRTF             = 0x00000002
	sffSelection      = 0x00008000
	enLink            = 0x070B
	cfeBold           = 0x00000001
	cfeItalic         = 0x00000002
	cfeUnderline      = 0x00000004
	pfmAlignment      = 0x00000008
	pfaLeft           = 1
	pfaCenter         = 2
	pfaRight          = 3

	lbAddString    = 0x0180
	lbDeleteString = 0x0182
	lbResetContent = 0x0184
	lbSetCurSel    = 0x0186
	lbGetCurSel    = 0x0188
	lbGetText      = 0x0189
	lbGetTextLen   = 0x018A
	lbGetCount     = 0x018B

	cbAddString    = 0x0143
	cbDeleteString = 0x0144
	cbResetContent = 0x014B
	cbGetCurSel    = 0x0147
	cbSetCurSel    = 0x014E
	cbGetLBText    = 0x0148
	cbGetLBTextLen = 0x0149
	cbShowDropDown = 0x014F

	pbmSetPos     = 0x0402
	pbmSetRange32 = 0x0406
	pbmGetPos     = 0x0408

	tbmGetPos      = 0x0400
	tbmSetPos      = 0x0405
	tbmSetRange    = 0x0406
	tbmSetTicFreq  = 0x0414
	tbmSetPageSize = 0x0415
	tbmSetLineSize = 0x0417

	mbOK              = 0x00000000
	mbOKCancel        = 0x00000001
	mbYesNo           = 0x00000004
	mbIconError       = 0x00000010
	mbIconQuestion    = 0x00000020
	mbIconWarning     = 0x00000030
	mbIconInformation = 0x00000040
	idOK              = 1
	idCancel          = 2
	idYes             = 6
	idNo              = 7

	ofnReadOnly        = 0x00000001
	ofnOverwritePrompt = 0x00000002
	ofnNoChangeDir     = 0x00000008
	ofnPathMustExist   = 0x00000800
	ofnFileMustExist   = 0x00001000
	ofnExplorer        = 0x00080000

	iccWin95Classes  = 0x000000FF
	iccBarClasses    = 0x00000004
	iccProgressClass = 0x00000020

	smCXScreen = 0
	smCYScreen = 1

	idcArrow       = 32512
	idiApplication = 32512

	fwNormal         = 400
	fwBold           = 700
	defaultCharset   = 1
	cleartypeQuality = 5
	logPixelsX       = 88

	srcCopy      = 0x00CC0020
	dibRGBColors = 0
	biRGB        = 0

	pwRenderFullContent = 0x00000002

	cfUnicodeText = 13
	gmemMoveable  = 0x0002

	vkReturn = 0x0D
	vkEscape = 0x1B

	colorWindow = 5
)

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       syscall.Handle
}

type msgT struct {
	hwnd    syscall.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	private uint32
}

type rect struct {
	left, top, right, bottom int32
}

type point struct {
	x, y int32
}

type minMaxInfo struct {
	reserved     point
	maxSize      point
	maxPosition  point
	minTrackSize point
	maxTrackSize point
}

type drawItemStruct struct {
	ctlType    uint32
	ctlID      uint32
	itemID     uint32
	itemAction uint32
	itemState  uint32
	hwndItem   syscall.Handle
	hDC        syscall.Handle
	rcItem     rect
	itemData   uintptr
}

type paintStruct struct {
	hdc         syscall.Handle
	fErase      int32
	rcPaint     rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  syscall.Handle
	hbmColor syscall.Handle
}

type openFileNameW struct {
	structSize    uint32
	hwndOwner     syscall.Handle
	hInstance     syscall.Handle
	filter        *uint16
	customFilter  *uint16
	maxCustFilter uint32
	filterIndex   uint32
	file          *uint16
	maxFile       uint32
	fileTitle     *uint16
	maxFileTitle  uint32
	initialDir    *uint16
	title         *uint16
	flags         uint32
	fileOffset    uint16
	fileExtension uint16
	defExt        *uint16
	custData      uintptr
	hook          uintptr
	templateName  *uint16
	reservedPtr   unsafe.Pointer
	reservedDword uint32
	flagsEx       uint32
}

type initCommonControlsEx struct {
	size uint32
	icc  uint32
}

type bitmapInfoHeader struct {
	size          uint32
	width         int32
	height        int32
	planes        uint16
	bitCount      uint16
	compression   uint32
	sizeImage     uint32
	xPelsPerMeter int32
	yPelsPerMeter int32
	clrUsed       uint32
	clrImportant  uint32
}

type bitmapInfo struct {
	header bitmapInfoHeader
	colors [1]uint32
}

func u16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}

func intResource(id uintptr) *uint16 {
	return (*uint16)(unsafe.Pointer(id))
}

func getModuleHandle() syscall.Handle {
	h, _, _ := pGetModuleHandleW.Call(0)
	return syscall.Handle(h)
}

func registerWindowClass(name string, proc uintptr) error {
	icon := loadAppIcon()
	if icon == 0 {
		icon = loadIcon(idiApplication)
	}
	wc := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		lpfnWndProc:   proc,
		hInstance:     getModuleHandle(),
		hCursor:       loadCursor(idcArrow),
		hIcon:         icon,
		hIconSm:       icon,
		lpszClassName: u16(name),
	}
	if wc.hCursor == 0 {
		wc.hCursor = syscall.Handle(^uintptr(0))
	}
	atom, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		if errno, ok := err.(syscall.Errno); ok && errno == 1410 {
			return nil
		}
		return fmt.Errorf("注册窗口类失败: %v", err)
	}
	return nil
}

func loadCursor(id uintptr) syscall.Handle {
	h, _, _ := pLoadCursorW.Call(0, uintptr(unsafe.Pointer(intResource(id))))
	return syscall.Handle(h)
}

func loadIcon(id uintptr) syscall.Handle {
	h, _, _ := pLoadIconW.Call(0, uintptr(unsafe.Pointer(intResource(id))))
	return syscall.Handle(h)
}

// loadAppIcon loads icon resource ID 1 from the running executable. The Agui
// build command stores a user icon there; 0 means no icon was embedded.
func loadAppIcon() syscall.Handle {
	h, _, _ := pLoadIconW.Call(uintptr(getModuleHandle()), uintptr(unsafe.Pointer(intResource(1))))
	return syscall.Handle(h)
}

func createWindow(exStyle uint32, class, title string, style uint32, x, y, w, h int32, parent syscall.Handle, id int) syscall.Handle {
	hwnd, _, _ := pCreateWindowExW.Call(
		uintptr(exStyle), uintptr(unsafe.Pointer(u16(class))), uintptr(unsafe.Pointer(u16(title))),
		uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent), uintptr(id), uintptr(getModuleHandle()), 0)
	return syscall.Handle(hwnd)
}

func defWindowProc(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) uintptr {
	r, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
	return r
}

func sendMessage(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) uintptr {
	r, _, _ := pSendMessageW.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
	return r
}

func setWindowText(hwnd syscall.Handle, text string) {
	pSetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(u16(text))))
}

func getWindowText(hwnd syscall.Handle) string {
	n, _, _ := pGetWindowTextLengthW.Call(uintptr(hwnd))
	buf := make([]uint16, int(n)+1)
	pGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}

func getClientRect(hwnd syscall.Handle) (rect, bool) {
	var r rect
	ok, _, _ := pGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)))
	return r, ok != 0
}

func getWindowRect(hwnd syscall.Handle) (rect, bool) {
	var r rect
	ok, _, _ := pGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)))
	return r, ok != 0
}

func moveWindow(hwnd syscall.Handle, x, y, w, h int32, repaint bool) {
	repaintVal := uintptr(0)
	if repaint {
		repaintVal = 1
	}
	pMoveWindow.Call(uintptr(hwnd), uintptr(x), uintptr(y), uintptr(w), uintptr(h), repaintVal)
}

func setWindowPos(hwnd syscall.Handle, after syscall.Handle, x, y, w, h int32, flags uint32) {
	pSetWindowPos.Call(uintptr(hwnd), uintptr(after), uintptr(x), uintptr(y), uintptr(w), uintptr(h), uintptr(flags))
}

// firstShowDone 标记进程内是否已经完成过第一次 ShowWindow 调用。
var firstShowDone bool

// showWindow 显示窗口。Windows 规定：若启动本进程时 STARTUPINFO 里带有
// SW_HIDE（例如 VS Code 扩展用 windowsHide 启动子进程、计划任务以隐藏窗口
// 方式启动），该值会覆盖进程内**第一次** ShowWindow 调用的参数，窗口因此
// 永远不可见。这里让第一次调用先按正常方式显示一次，再应用调用方要求的
// 显示方式，避免被宿主"藏"起来。
func showWindow(hwnd syscall.Handle, cmd int32) {
	if !firstShowDone {
		// 这次调用可能被 STARTUPINFO 的 SW_HIDE 覆盖，只是"消耗"掉那次覆盖。
		firstShowDone = true
		pShowWindow.Call(uintptr(hwnd), uintptr(swShowNormal))
	}
	pShowWindow.Call(uintptr(hwnd), uintptr(cmd))
}

func setFocus(hwnd syscall.Handle) {
	pSetFocus.Call(uintptr(hwnd))
}

func getWindowLong(hwnd syscall.Handle, index int32) uintptr {
	r, _, _ := pGetWindowLongPtrW.Call(uintptr(hwnd), uintptr(index))
	return r
}

func setWindowLong(hwnd syscall.Handle, index int32, value uintptr) uintptr {
	r, _, _ := pSetWindowLongPtrW.Call(uintptr(hwnd), uintptr(index), value)
	return r
}

func drawText(hdc syscall.Handle, text string, r *rect, flags uint32) {
	pDrawTextW.Call(uintptr(hdc), uintptr(unsafe.Pointer(u16(text))), ^uintptr(0),
		uintptr(unsafe.Pointer(r)), uintptr(flags))
}

func drawEdge(hdc syscall.Handle, r *rect, edge, flags uint32) {
	pDrawEdge.Call(uintptr(hdc), uintptr(unsafe.Pointer(r)), uintptr(edge), uintptr(flags))
}

func drawFocusRect(hdc syscall.Handle, r *rect) {
	pDrawFocusRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(r)))
}

func frameRect(hdc syscall.Handle, r *rect, brush syscall.Handle) {
	pFrameRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(r)), uintptr(brush))
}

func isWindow(h syscall.Handle) bool {
	r, _, _ := pIsWindow.Call(uintptr(h))
	return r != 0
}

func enableWindow(hwnd syscall.Handle, enabled bool) {
	v := uintptr(0)
	if enabled {
		v = 1
	}
	pEnableWindow.Call(uintptr(hwnd), v)
}

func isWindowVisible(hwnd syscall.Handle) bool {
	r, _, _ := pIsWindowVisible.Call(uintptr(hwnd))
	return r != 0
}

func isWindowEnabled(hwnd syscall.Handle) bool {
	r, _, _ := pIsWindowEnabled.Call(uintptr(hwnd))
	return r != 0
}

func invalidateRect(hwnd syscall.Handle, r *rect, erase bool) {
	e := uintptr(0)
	if erase {
		e = 1
	}
	pInvalidateRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(r)), e)
}

func setTimer(hwnd syscall.Handle, id uintptr, ms uint32) bool {
	r, _, _ := pSetTimer.Call(uintptr(hwnd), id, uintptr(ms), 0)
	return r != 0
}

func killTimer(hwnd syscall.Handle, id uintptr) {
	pKillTimer.Call(uintptr(hwnd), id)
}

func fillRect(hdc syscall.Handle, r *rect, brush syscall.Handle) {
	pFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(r)), uintptr(brush))
}

func adjustWindowRect(r *rect, style uint32, menu bool, exStyle uint32) {
	m := uintptr(0)
	if menu {
		m = 1
	}
	pAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(r)), uintptr(style), m, uintptr(exStyle))
}

func screenSize() (int32, int32) {
	w, _, _ := pGetSystemMetrics.Call(smCXScreen)
	h, _, _ := pGetSystemMetrics.Call(smCYScreen)
	return int32(w), int32(h)
}

func messageBox(owner syscall.Handle, text, title string, flags uint32) int {
	r, _, _ := pMessageBoxW.Call(uintptr(owner), uintptr(unsafe.Pointer(u16(text))),
		uintptr(unsafe.Pointer(u16(title))), uintptr(flags))
	return int(r)
}

func createFont(family string, pt float64, bold, italic bool, dpi int32) syscall.Handle {
	weight := uintptr(fwNormal)
	if bold {
		weight = fwBold
	}
	italicVal := uintptr(0)
	if italic {
		italicVal = 1
	}
	height := int32(-(pt*float64(dpi)/72.0 + 0.5))
	h, _, _ := pCreateFontW.Call(
		uintptr(height), 0, 0, 0, weight, italicVal, 0, 0,
		defaultCharset, 0, 0, cleartypeQuality, 0,
		uintptr(unsafe.Pointer(u16(family))))
	return syscall.Handle(h)
}

func createBrush(color uint32) syscall.Handle {
	h, _, _ := pCreateSolidBrush.Call(uintptr(color))
	return syscall.Handle(h)
}

func deleteObject(h syscall.Handle) {
	if h != 0 {
		pDeleteObject.Call(uintptr(h))
	}
}

func setBkMode(hdc syscall.Handle, mode int32) {
	pSetBkMode.Call(uintptr(hdc), uintptr(mode))
}

func setTextColor(hdc syscall.Handle, color uint32) {
	pSetTextColor.Call(uintptr(hdc), uintptr(color))
}

func setBkColor(hdc syscall.Handle, color uint32) {
	pSetBkColor.Call(uintptr(hdc), uintptr(color))
}

func stockObject(id int32) syscall.Handle {
	h, _, _ := pGetStockObject.Call(uintptr(id))
	return syscall.Handle(h)
}

func systemDPI() int32 {
	if pSetProcessDPIAware.Find() == nil {
		pSetProcessDPIAware.Call()
	}
	if pGetDpiForSystem.Find() == nil {
		if dpi, _, _ := pGetDpiForSystem.Call(); dpi >= 72 {
			return int32(dpi)
		}
	}
	hdc, _, _ := pGetDC.Call(0)
	if hdc != 0 {
		defer pReleaseDC.Call(0, hdc)
		if dpi, _, _ := pGetDeviceCaps.Call(hdc, logPixelsX); dpi >= 72 {
			return int32(dpi)
		}
	}
	return 96
}

func initCommonControls() {
	init := initCommonControlsEx{size: uint32(unsafe.Sizeof(initCommonControlsEx{})), icc: iccWin95Classes | iccProgressClass | iccBarClasses}
	pInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&init)))
}

func setWindowTheme(hwnd syscall.Handle, theme, list string) {
	if pSetWindowTheme.Find() != nil {
		return
	}
	pSetWindowTheme.Call(uintptr(hwnd), uintptr(unsafe.Pointer(u16(theme))), uintptr(unsafe.Pointer(u16(list))))
}

func moveMemory(dst, src unsafe.Pointer, n uintptr) {
	pRtlMoveMemory.Call(uintptr(dst), uintptr(src), n)
}

func globalAlloc(flags uint32, bytes uintptr) syscall.Handle {
	h, _, _ := pGlobalAlloc.Call(uintptr(flags), bytes)
	return syscall.Handle(h)
}

func globalLock(h syscall.Handle) unsafe.Pointer {
	p, _, _ := pGlobalLock.Call(uintptr(h))
	return unsafe.Pointer(p)
}

func globalUnlock(h syscall.Handle) {
	pGlobalUnlock.Call(uintptr(h))
}

func globalFree(h syscall.Handle) {
	pGlobalFree.Call(uintptr(h))
}

func clipboardSet(text string) error {
	r, _, _ := pOpenClipboard.Call(0)
	if r == 0 {
		return fmt.Errorf("无法打开剪贴板")
	}
	defer pCloseClipboard.Call()
	pEmptyClipboard.Call()
	data, err := syscall.UTF16FromString(text)
	if err != nil {
		return err
	}
	size := uintptr(len(data) * 2)
	h := globalAlloc(gmemMoveable, size)
	if h == 0 {
		return fmt.Errorf("无法分配剪贴板内存")
	}
	ptr := globalLock(h)
	if ptr == nil {
		globalFree(h)
		return fmt.Errorf("无法锁定剪贴板内存")
	}
	moveMemory(ptr, unsafe.Pointer(&data[0]), size)
	globalUnlock(h)
	if r, _, _ := pSetClipboardData.Call(cfUnicodeText, uintptr(h)); r == 0 {
		globalFree(h)
		return fmt.Errorf("无法写入剪贴板")
	}
	return nil
}

func clipboardGet() (string, error) {
	if r, _, _ := pOpenClipboard.Call(0); r == 0 {
		return "", fmt.Errorf("无法打开剪贴板")
	}
	defer pCloseClipboard.Call()
	h, _, _ := pGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return "", nil
	}
	ptr := globalLock(syscall.Handle(h))
	if ptr == nil {
		return "", fmt.Errorf("无法锁定剪贴板内存")
	}
	defer globalUnlock(syscall.Handle(h))
	// Find the terminating NUL without trusting an unbounded buffer.
	n := 0
	for {
		ch := *(*uint16)(unsafe.Pointer(uintptr(ptr) + uintptr(n*2)))
		if ch == 0 {
			break
		}
		n++
		if n > 1<<20 {
			return "", fmt.Errorf("剪贴板文本过长")
		}
	}
	buf := unsafe.Slice((*uint16)(ptr), n)
	return syscall.UTF16ToString(buf), nil
}

func filterSpec(spec string) []uint16 {
	if spec == "" {
		spec = "所有文件|*.*"
	}
	parts := splitFilter(spec)
	var out []uint16
	for _, p := range parts {
		out = append(out, syscall.StringToUTF16(p)...)
	}
	out = append(out, 0)
	return out
}

func splitFilter(spec string) []string {
	var parts []string
	var cur []rune
	for _, r := range spec {
		if r == '|' {
			parts = append(parts, string(cur))
			cur = cur[:0]
			continue
		}
		cur = append(cur, r)
	}
	parts = append(parts, string(cur))
	if len(parts)%2 == 1 {
		parts = append(parts, "*.*")
	}
	return parts
}

func setWindowMenu(hwnd, menu syscall.Handle) {
	pSetMenu.Call(uintptr(hwnd), uintptr(menu))
}

func createCompatibleDC(hdc syscall.Handle) syscall.Handle {
	r, _, _ := pCreateCompatibleDC.Call(uintptr(hdc))
	return syscall.Handle(r)
}

func createCompatibleBitmap(hdc syscall.Handle, w, h int32) syscall.Handle {
	r, _, _ := pCreateCompatibleBitmap.Call(uintptr(hdc), uintptr(w), uintptr(h))
	return syscall.Handle(r)
}

func bitBlt(dst syscall.Handle, x, y, w, h int32, src syscall.Handle, sx, sy int32) {
	pBitBlt.Call(uintptr(dst), uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(src), uintptr(sx), uintptr(sy), srcCopy)
}

func textExtent(hdc, font syscall.Handle, text string) (int32, int32) {
	old := selectObject(hdc, font)
	defer selectObject(hdc, old)
	ptr := u16(text)
	var sz struct{ cx, cy int32 }
	pGetTextExtentPoint32W.Call(uintptr(hdc), uintptr(unsafe.Pointer(ptr)),
		uintptr(len([]rune(text))), uintptr(unsafe.Pointer(&sz)))
	return sz.cx, sz.cy
}
