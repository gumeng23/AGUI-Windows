package gui

import (
	"fmt"
	"unicode/utf16"
	"unsafe"

	"agui/internal/lang"
)

// dropFilesW 是 Windows 的 DROPFILES 头（20 字节）。
type dropFilesW struct {
	pFiles uint32
	ptX    int32
	ptY    int32
	fNC    int32
	fWide  int32
}

// simulateDrop 构造一个兼容 WM_DROPFILES 的 HDROP，并直接走 handleDrop。
// 主要用于自动化测试；真实拖放仍由 DragAcceptFiles/WM_DROPFILES 处理。
func (w *Widget) simulateDrop(paths []string) error {
	if w.kind != "window" {
		return fmt.Errorf("simulate_drop 只适用于窗口")
	}
	size := uintptr(20 + 2)
	for _, p := range paths {
		size += uintptr(len(utf16.Encode([]rune(p)))*2 + 2)
	}
	h := globalAlloc(gmemMoveable, size)
	if h == 0 {
		return fmt.Errorf("无法分配拖放测试内存")
	}
	ptr, _, _ := pGlobalLock.Call(uintptr(h))
	if ptr == 0 {
		return fmt.Errorf("无法锁定拖放测试内存")
	}
	header := (*dropFilesW)(unsafe.Pointer(ptr))
	header.pFiles = 20
	header.fWide = 1
	off := uintptr(20)
	put := func(u uint16) {
		*(*uint16)(unsafe.Pointer(ptr + off)) = u
		off += 2
	}
	for _, p := range paths {
		for _, u := range utf16.Encode([]rune(p)) {
			put(u)
		}
		put(0)
	}
	put(0)
	pGlobalUnlock.Call(uintptr(h))
	w.handleDrop(h)
	return nil
}

func stringsFromValue(v lang.Value) []string {
	if l, ok := v.(*lang.List); ok {
		out := make([]string, 0, len(l.Items))
		for _, it := range l.Items {
			out = append(out, lang.ToString(it))
		}
		return out
	}
	return []string{lang.ToString(v)}
}
