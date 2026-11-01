package gui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// Screenshot captures the main window (including its frame) and writes a PNG.
// It is used by `agui run --shot` and by the screenshot() built-in.
func (a *App) Screenshot(path string) error {
	if a.hwnd == 0 {
		return fmt.Errorf("还没有创建窗口，无法截图")
	}
	if path == "" {
		path = "screenshot.png"
	}
	r, ok := getWindowRect(a.hwnd)
	if !ok {
		return fmt.Errorf("无法获取窗口尺寸")
	}
	w := int(r.right - r.left)
	h := int(r.bottom - r.top)
	if w <= 0 || h <= 0 {
		return fmt.Errorf("窗口尺寸无效：%dx%d", w, h)
	}

	hdcWindow, _, _ := pGetWindowDC.Call(uintptr(a.hwnd))
	if hdcWindow == 0 {
		return fmt.Errorf("GetWindowDC 失败")
	}
	defer pReleaseDC.Call(uintptr(a.hwnd), hdcWindow)

	hdcMem, _, _ := pCreateCompatibleDC.Call(hdcWindow)
	if hdcMem == 0 {
		return fmt.Errorf("CreateCompatibleDC 失败")
	}
	defer pDeleteDC.Call(hdcMem)

	hbm, _, _ := pCreateCompatibleBitmap.Call(hdcWindow, uintptr(w), uintptr(h))
	if hbm == 0 {
		return fmt.Errorf("CreateCompatibleBitmap 失败")
	}
	defer deleteObject(syscall.Handle(hbm))

	old, _, _ := pSelectObject.Call(hdcMem, hbm)
	printed, _, _ := pPrintWindow.Call(uintptr(a.hwnd), hdcMem, pwRenderFullContent)
	if printed == 0 {
		pBitBlt.Call(hdcMem, 0, 0, uintptr(w), uintptr(h), hdcWindow, 0, 0, srcCopy)
	}
	pSelectObject.Call(hdcMem, old)

	bmi := bitmapInfo{}
	bmi.header.size = uint32(unsafe.Sizeof(bitmapInfoHeader{}))
	bmi.header.width = int32(w)
	bmi.header.height = int32(-h)
	bmi.header.planes = 1
	bmi.header.bitCount = 32
	bmi.header.compression = biRGB

	buf := make([]byte, w*h*4)
	lines, _, _ := pGetDIBits.Call(hdcMem, hbm, 0, uintptr(h),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bmi)), dibRGBColors)
	if lines == 0 {
		return fmt.Errorf("GetDIBits 失败")
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		src := buf[y*w*4 : (y+1)*w*4]
		dst := img.Pix[y*img.Stride : y*img.Stride+w*4]
		for x := 0; x < w; x++ {
			dst[x*4+0] = src[x*4+2]
			dst[x*4+1] = src[x*4+1]
			dst[x*4+2] = src[x*4+0]
			dst[x*4+3] = 0xFF
		}
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(abs); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.Create(abs)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return err
	}
	return nil
}
