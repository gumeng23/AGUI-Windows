package gui

import (
	"sync"
	"syscall"
	"unsafe"
)

// Minimal MSAA bridge: custom window classes do not get the standard
// accessible object that DefWindowProc creates for system controls, so
// WM_GETOBJECT is answered with CreateStdAccessibleObject. This gives
// screen readers a client object with the window text as its name.

const (
	wmGetObject = 0x003D
	objIDClient = ^uintptr(3) // -4
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// IID_IAccessible {618736E0-3C3D-11CF-810C-00AA00389B71}
var iidIAccessible = guid{
	Data1: 0x618736E0,
	Data2: 0x3C3D,
	Data3: 0x11CF,
	Data4: [8]byte{0x81, 0x0C, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71},
}

var (
	oleacc = syscall.NewLazyDLL("oleacc.dll")
	ole32  = syscall.NewLazyDLL("ole32.dll")

	pCreateStdAccessibleObject = oleacc.NewProc("CreateStdAccessibleObject")
	pLresultFromObject         = oleacc.NewProc("LresultFromObject")
	pCoInitializeEx            = ole32.NewProc("CoInitializeEx")

	oleOnce sync.Once
)

func initOLE() {
	oleOnce.Do(func() {
		// COINIT_APARTMENTTHREADED. RPC_E_CHANGED_MODE is harmless when
		// something already initialized the UI thread as MTA.
		pCoInitializeEx.Call(0, 2)
	})
}

// handleGetObject answers WM_GETOBJECT for a custom window. It returns 0
// when the request is not for the client object or the standard provider
// could not be created.
func handleGetObject(hwnd, wparam uintptr) uintptr {
	if int32(wparam) != -4 {
		return 0
	}
	initOLE()
	var acc unsafe.Pointer
	hr, _, _ := pCreateStdAccessibleObject.Call(
		hwnd,
		objIDClient,
		uintptr(unsafe.Pointer(&iidIAccessible)),
		uintptr(unsafe.Pointer(&acc)),
	)
	if int32(hr) < 0 || acc == nil {
		return 0
	}
	lr, _, _ := pLresultFromObject.Call(
		uintptr(unsafe.Pointer(&iidIAccessible)),
		wparam,
		uintptr(acc),
	)
	releaseUnknown(acc)
	if lr == 0 {
		return 0
	}
	return lr
}

func releaseUnknown(p unsafe.Pointer) {
	if p == nil {
		return
	}
	vtbl := *(*uintptr)(p)
	release := *(*uintptr)(unsafe.Pointer(vtbl + 2*unsafe.Sizeof(uintptr(0))))
	if release != 0 {
		syscall.Syscall(release, 1, uintptr(p), 0, 0)
	}
}
