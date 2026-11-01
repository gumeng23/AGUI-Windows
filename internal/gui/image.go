package gui

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// GDI+ is used for image decoding and alpha-aware, high-quality scaling.
// It ships with Windows, so this keeps Agui dependency-free.
var (
	gdiplus = syscall.NewLazyDLL("gdiplus.dll")

	pGdiplusStartup            = gdiplus.NewProc("GdiplusStartup")
	pGdipCreateBitmapFromFile  = gdiplus.NewProc("GdipCreateBitmapFromFile")
	pGdipGetImageWidth         = gdiplus.NewProc("GdipGetImageWidth")
	pGdipGetImageHeight        = gdiplus.NewProc("GdipGetImageHeight")
	pGdipDisposeImage          = gdiplus.NewProc("GdipDisposeImage")
	pGdipCreateFromHDC         = gdiplus.NewProc("GdipCreateFromHDC")
	pGdipDeleteGraphics        = gdiplus.NewProc("GdipDeleteGraphics")
	pGdipSetInterpolationMode  = gdiplus.NewProc("GdipSetInterpolationMode")
	pGdipSetSmoothingMode      = gdiplus.NewProc("GdipSetSmoothingMode")
	pGdipDrawImageRectI        = gdiplus.NewProc("GdipDrawImageRectI")
	pGdipCreateHICONFromBitmap = gdiplus.NewProc("GdipCreateHICONFromBitmap")
	pGdipCreateHBITMAP         = gdiplus.NewProc("GdipCreateHBITMAPFromBitmap")
)

type gdiplusStartupInput struct {
	version                  uint32
	debugEventCallback       uintptr
	suppressBackgroundThread int32
	suppressExternalCodecs   int32
}

var (
	gdiplusOnce  sync.Once
	gdiplusToken uintptr
	gdiplusErr   error
)

func startGdiplus() error {
	gdiplusOnce.Do(func() {
		in := gdiplusStartupInput{version: 1}
		status, _, _ := pGdiplusStartup.Call(
			uintptr(unsafe.Pointer(&gdiplusToken)),
			uintptr(unsafe.Pointer(&in)),
			0,
		)
		if status != 0 {
			gdiplusErr = fmt.Errorf("GDI+ 初始化失败（状态 %d）", status)
		}
	})
	return gdiplusErr
}

func loadGdiImage(path string) (uintptr, int32, int32, error) {
	if err := startGdiplus(); err != nil {
		return 0, 0, 0, err
	}
	var img uintptr
	status, _, _ := pGdipCreateBitmapFromFile.Call(
		uintptr(unsafe.Pointer(u16(path))),
		uintptr(unsafe.Pointer(&img)),
	)
	if status != 0 || img == 0 {
		return 0, 0, 0, fmt.Errorf("无法加载图片 %q（支持 png/jpg/bmp/gif）", path)
	}
	var iw, ih int32
	pGdipGetImageWidth.Call(img, uintptr(unsafe.Pointer(&iw)))
	pGdipGetImageHeight.Call(img, uintptr(unsafe.Pointer(&ih)))
	return img, iw, ih, nil
}

func disposeGdiImage(img uintptr) {
	if img != 0 {
		pGdipDisposeImage.Call(img)
	}
}

func probeImageSize(path string) (int32, int32) {
	img, w, h, err := loadGdiImage(path)
	if err != nil {
		return 0, 0
	}
	disposeGdiImage(img)
	return w, h
}

func drawGdiImage(hdc syscall.Handle, img uintptr, x, y, w, h int32) {
	if img == 0 || w <= 0 || h <= 0 {
		return
	}
	var g uintptr
	if status, _, _ := pGdipCreateFromHDC.Call(uintptr(hdc), uintptr(unsafe.Pointer(&g))); status != 0 || g == 0 {
		return
	}
	defer pGdipDeleteGraphics.Call(g)
	// 7 = HighQualityBicubic, 4 = AntiAlias
	pGdipSetInterpolationMode.Call(g, 7)
	pGdipSetSmoothingMode.Call(g, 4)
	pGdipDrawImageRectI.Call(g, img, uintptr(x), uintptr(y), uintptr(w), uintptr(h))
}

// loadMenuBitmap decodes a png/jpg into an HBITMAP suitable for a menu item.
func loadMenuBitmap(path string) (syscall.Handle, error) {
	img, _, _, err := loadGdiImage(path)
	if err != nil {
		return 0, err
	}
	defer disposeGdiImage(img)
	var hbmp uintptr
	// ARGB background 0 keeps the image's alpha channel.
	status, _, _ := pGdipCreateHBITMAP.Call(img, uintptr(unsafe.Pointer(&hbmp)), 0)
	if status != 0 || hbmp == 0 {
		return 0, fmt.Errorf("无法把 %q 转成菜单图标（状态 %d）", path, status)
	}
	return syscall.Handle(hbmp), nil
}

func (w *Widget) loadImage(path string) error {
	img, iw, ih, err := loadGdiImage(path)
	if err != nil {
		return err
	}
	if w.gdiImage != 0 {
		disposeGdiImage(w.gdiImage)
	}
	w.gdiImage = img
	w.imgW, w.imgH = iw, ih
	w.imgPath = path
	w.imgLoaded = true
	invalidateRect(w.hwnd, nil, true)
	return nil
}

func (w *Widget) drawImage(hdc syscall.Handle) {
	var cr rect
	if ok, _, _ := pGetClientRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&cr))); ok == 0 {
		return
	}
	bg := uint32(0xFFFFFF)
	if w.hasBg {
		bg = w.bg
	}
	fillRect(hdc, &cr, w.app.brushFor(bg))
	if w.gdiImage == 0 {
		return
	}
	x, y := int32(0), int32(0)
	dw, dh := cr.right, cr.bottom
	if !w.imgFit {
		dw, dh = w.imgW, w.imgH
		x = (cr.right - dw) / 2
		y = (cr.bottom - dh) / 2
	}
	drawGdiImage(hdc, w.gdiImage, x, y, dw, dh)
}

func imageWndProc(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) uintptr {
	a := currentApp
	if a == nil {
		return defWindowProc(hwnd, msg, wparam, lparam)
	}
	w := a.widgetByHWND(hwnd)
	switch msg {
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := pBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		if w != nil {
			w.drawImage(syscall.Handle(hdc))
		}
		pEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmEraseBkgnd:
		return 1
	case wmLButtonUp:
		if w != nil {
			w.emit("click")
		}
		return 0
	case wmDblClk:
		if w != nil {
			w.emit("dblclick")
		}
		return 0
	}
	return defWindowProc(hwnd, msg, wparam, lparam)
}

var imageProcCallback = syscall.NewCallback(imageWndProc)
