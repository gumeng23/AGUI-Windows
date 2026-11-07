package gui

import (
	"fmt"
	"syscall"
	"unsafe"

	"agui/internal/lang"
)

// Canvas is a small immediate-mode drawing surface. Drawing commands are kept
// as a display list and replayed on every WM_PAINT, so ordinary window
// invalidation (resize, overlap, move) just works.
type canvasOp struct {
	kind       string
	x1, y1     float64
	x2, y2     float64
	r          float64
	text       string
	color      uint32
	width      int32
	fill       bool
	size       float64
	fontFamily string
	image      string
	sx, sy     float64
	sw, sh     float64
	screen     bool
	outline    uint32
	outlineW   int32
	alpha      float64
	useAlpha   bool
}

func (a *App) createCanvas(x, y, w, h *int32) (*Widget, error) {
	widget, err := a.addChild("canvas", canvasClassName, "", 0, true, a.px(420), a.px(300), x, y, w, h)
	if err != nil {
		return nil, err
	}
	return widget, nil
}

func (w *Widget) canvasAdd(op canvasOp) {
	w.canvasOps = append(w.canvasOps, op)
	if !w.canvasBatch {
		invalidateRect(w.hwnd, nil, true)
	}
}

func (w *Widget) canvasFlip() {
	invalidateRect(w.hwnd, nil, true)
}

func (w *Widget) canvasBatchBegin() { w.canvasBatch = true }
func (w *Widget) canvasBatchEnd() {
	w.canvasBatch = false
	w.canvasFlip()
}

func (w *Widget) clearCanvas() {
	w.canvasOps = w.canvasOps[:0]
	if !w.canvasBatch {
		w.canvasFlip()
	}
}

func (w *Widget) canvasLine(args []lang.Value, kwargs map[string]lang.Value) error {
	if len(args) < 4 {
		return fmt.Errorf("line(x1, y1, x2, y2) 需要四个坐标")
	}
	f := func(i int) float64 {
		v, _ := lang.Pos(args, i, float64(0)).(float64)
		return v
	}
	w.canvasAdd(canvasOp{
		kind:  "line",
		x1:    f(0),
		y1:    f(1),
		x2:    f(2),
		y2:    f(3),
		color: canvasColor(kwargs["color"], 0x1F2937),
		width: canvasWidth(kwargs["width"], 1),
	})
	return nil
}

func (w *Widget) canvasRect(args []lang.Value, kwargs map[string]lang.Value) error {
	if len(args) < 4 {
		return fmt.Errorf("rect(x, y, w, h) 需要四个参数")
	}
	f := func(i int) float64 {
		v, _ := lang.Pos(args, i, float64(0)).(float64)
		return v
	}
	w.canvasAdd(canvasOp{
		kind:  "rect",
		x1:    f(0),
		y1:    f(1),
		x2:    f(2),
		y2:    f(3),
		color: canvasColor(kwargs["color"], 0x2563EB),
		width: canvasWidth(kwargs["width"], 1),
		fill:  lang.Truthy(kwargs["fill"]),
	})
	return nil
}

func (w *Widget) canvasCircle(args []lang.Value, kwargs map[string]lang.Value) error {
	if len(args) < 3 {
		return fmt.Errorf("circle(x, y, r) 需要三个参数")
	}
	f := func(i int) float64 {
		v, _ := lang.Pos(args, i, float64(0)).(float64)
		return v
	}
	w.canvasAdd(canvasOp{
		kind:  "circle",
		x1:    f(0),
		y1:    f(1),
		r:     f(2),
		color: canvasColor(kwargs["color"], 0xDC2626),
		width: canvasWidth(kwargs["width"], 1),
		fill:  lang.Truthy(kwargs["fill"]),
	})
	return nil
}

func (w *Widget) canvasText(args []lang.Value, kwargs map[string]lang.Value) error {
	if len(args) < 3 {
		return fmt.Errorf("text(x, y, 内容) 需要三个参数")
	}
	x, _ := lang.Pos(args, 0, float64(0)).(float64)
	y, _ := lang.Pos(args, 1, float64(0)).(float64)
	w.canvasAdd(canvasOp{
		kind:       "text",
		x1:         x,
		y1:         y,
		text:       lang.ToString(args[2]),
		color:      canvasColor(kwargs["color"], 0x111827),
		size:       canvasFloat(kwargs["size"], 12),
		fontFamily: canvasString(kwargs["font"], "Microsoft YaHei UI"),
		screen:     lang.Truthy(kwargs["screen"]),
		outline:    canvasColor(kwargs["outline"], 0),
		outlineW:   canvasWidth(kwargs["outline_width"], 1),
	})
	return nil
}

func blendColor(fg, bg uint32, alpha float64) uint32 {
	if alpha >= 1 {
		return fg
	}
	if alpha <= 0 {
		return bg
	}
	blend := func(f, b uint32) uint32 {
		return uint32(float64(f)*alpha + float64(b)*(1-alpha) + 0.5)
	}
	return blend(fg&0xFF, bg&0xFF) |
		blend((fg>>8)&0xFF, (bg>>8)&0xFF)<<8 |
		blend((fg>>16)&0xFF, (bg>>16)&0xFF)<<16
}

func canvasColor(v lang.Value, def uint32) uint32 {
	if v == nil {
		return def
	}
	if c, ok := parseColor(lang.ToString(v)); ok {
		return c
	}
	return def
}

func canvasWidth(v lang.Value, def int32) int32 {
	if v == nil {
		return def
	}
	if n, ok := v.(float64); ok && n > 0 {
		return int32(n)
	}
	return def
}

func canvasFloat(v lang.Value, def float64) float64 {
	if n, ok := v.(float64); ok && n > 0 {
		return n
	}
	return def
}

func canvasString(v lang.Value, def string) string {
	if v == nil {
		return def
	}
	if s := lang.ToString(v); s != "" {
		return s
	}
	return def
}

func (w *Widget) paintCanvas(hdc syscall.Handle) {
	cr, _ := getClientRect(w.hwnd)
	if cr.right <= 0 || cr.bottom <= 0 {
		w.paintCanvasDirect(hdc)
		return
	}
	mem := createCompatibleDC(hdc)
	if mem == 0 {
		w.paintCanvasDirect(hdc)
		return
	}
	bmp := createCompatibleBitmap(hdc, cr.right, cr.bottom)
	if bmp == 0 {
		pDeleteDC.Call(uintptr(mem))
		w.paintCanvasDirect(hdc)
		return
	}
	old := selectObject(mem, bmp)
	w.paintCanvasDirect(mem)
	bitBlt(hdc, 0, 0, cr.right, cr.bottom, mem, 0, 0)
	selectObject(mem, old)
	deleteObject(bmp)
	pDeleteDC.Call(uintptr(mem))
}

func (w *Widget) paintCanvasDirect(hdc syscall.Handle) {
	a := w.app
	g := a.game
	var bg uint32 = 0xFFFFFF
	if w.hasBg {
		bg = w.bg
	}
	brush := createBrush(bg)
	cr, _ := getClientRect(w.hwnd)
	r := rect{left: 0, top: 0, right: cr.right, bottom: cr.bottom}
	fillRect(hdc, &r, brush)
	deleteObject(brush)

	zoom := 1.0
	if g != nil && g.camZoom > 0 {
		zoom = g.camZoom
	}
	for _, op := range w.canvasOps {
		x1, y1 := op.x1, op.y1
		x2, y2 := op.x2, op.y2
		rr := op.r
		ww := op.x2
		hh := op.y2
		if !op.screen && g != nil {
			x1, y1 = g.screen(x1, y1)
			x2, y2 = g.screen(x2, y2)
			rr *= zoom
			ww *= zoom
			hh *= zoom
		}
		px1, py1 := a.px(x1), a.px(y1)
		px2, py2 := a.px(x2), a.px(y2)
		prr := a.px(rr)
		pww, phh := a.px(ww), a.px(hh)
		switch op.kind {
		case "line":
			pen := createPen(op.color, a.px(float64(op.width)))
			old := selectObject(hdc, pen)
			pMoveToEx.Call(uintptr(hdc), uintptr(px1), uintptr(py1), 0)
			pLineTo.Call(uintptr(hdc), uintptr(px2), uintptr(py2))
			selectObject(hdc, old)
			deleteObject(pen)
		case "rect":
			pen := createPen(op.color, a.px(float64(op.width)))
			oldPen := selectObject(hdc, pen)
			var b syscall.Handle
			if op.fill {
				b = createBrush(op.color)
			} else {
				b = stockObject(nullBrush)
			}
			oldBrush := selectObject(hdc, b)
			pRectangle.Call(uintptr(hdc), uintptr(px1), uintptr(py1),
				uintptr(px1+pww), uintptr(py1+phh))
			selectObject(hdc, oldBrush)
			if op.fill {
				deleteObject(b)
			}
			selectObject(hdc, oldPen)
			deleteObject(pen)
		case "circle":
			col := op.color
			if op.useAlpha {
				col = blendColor(col, bg, op.alpha)
			}
			pen := createPen(col, a.px(float64(op.width)))
			oldPen := selectObject(hdc, pen)
			var b syscall.Handle
			if op.fill {
				b = createBrush(col)
			} else {
				b = stockObject(nullBrush)
			}
			oldBrush := selectObject(hdc, b)
			pEllipse.Call(uintptr(hdc), uintptr(px1-prr), uintptr(py1-prr),
				uintptr(px1+prr), uintptr(py1+prr))
			selectObject(hdc, oldBrush)
			if op.fill {
				deleteObject(b)
			}
			selectObject(hdc, oldPen)
			deleteObject(pen)
		case "text":
			setBkMode(hdc, transparent)
			setTextColor(hdc, op.color)
			size := op.size * zoom
			if size < 6 {
				size = 6
			}
			var hf syscall.Handle
			if g != nil {
				hf = g.gameFont(op.fontFamily, size, false, false)
			}
			if hf == 0 {
				hf = createFont(op.fontFamily, size, false, false, a.dpi)
			}
			old := selectObject(hdc, hf)
			ptr := u16(op.text)
			if op.outline != 0 {
				setTextColor(hdc, op.outline)
				ow := a.px(float64(op.outlineW))
				if ow < 1 {
					ow = 1
				}
				for _, d := range [][2]int32{{-ow, -ow}, {0, -ow}, {ow, -ow}, {-ow, 0}, {ow, 0}, {-ow, ow}, {0, ow}, {ow, ow}} {
					pTextOutW.Call(uintptr(hdc), uintptr(px1+d[0]), uintptr(py1+d[1]),
						uintptr(unsafe.Pointer(ptr)), uintptr(len([]rune(op.text))))
				}
				setTextColor(hdc, op.color)
			}
			pTextOutW.Call(uintptr(hdc), uintptr(px1), uintptr(py1),
				uintptr(unsafe.Pointer(ptr)), uintptr(len([]rune(op.text))))
			selectObject(hdc, old)
			if g == nil {
				deleteObject(hf)
			}
		case "image":
			if g == nil {
				continue
			}
			gi, err := g.loadGameImage(op.image)
			if err != nil || gi == nil {
				continue
			}
			sx, sy, sw, sh := op.sx, op.sy, op.sw, op.sh
			if sw <= 0 {
				sw = float64(gi.w)
			}
			if sh <= 0 {
				sh = float64(gi.h)
			}
			dw, dh := ww, hh
			if dw <= 0 {
				dw = float64(a.px(sw * zoom))
			}
			if dh <= 0 {
				dh = float64(a.px(sh * zoom))
			}
			drawGdiImageRect(hdc, gi.img,
				int32(sx), int32(sy), int32(sw), int32(sh),
				px1, py1, int32(dw), int32(dh))
		}
	}
}

func createPen(color uint32, width int32) syscall.Handle {
	if width < 1 {
		width = 1
	}
	h, _, _ := pCreatePen.Call(psSolid, uintptr(width), uintptr(color))
	return syscall.Handle(h)
}

func selectObject(hdc, obj syscall.Handle) syscall.Handle {
	r, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(obj))
	return syscall.Handle(r)
}

func canvasWndProc(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) uintptr {
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
			w.paintCanvas(syscall.Handle(hdc))
		}
		pEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmEraseBkgnd:
		return 1
	case wmMouseMove:
		if w != nil {
			w.mouseX = float64(int16(lparam&0xFFFF)) / a.scale
			w.mouseY = float64(int16((lparam>>16)&0xFFFF)) / a.scale
			w.emit("mousemove")
		}
		return 0
	case wmLButtonDown, wmRButtonDown, wmMButtonDown:
		if w != nil {
			w.mouseX = float64(int16(lparam&0xFFFF)) / a.scale
			w.mouseY = float64(int16((lparam>>16)&0xFFFF)) / a.scale
			w.mouseDown = true
			switch msg {
			case wmRButtonDown:
				w.mouseButton = "right"
			case wmMButtonDown:
				w.mouseButton = "middle"
			default:
				w.mouseButton = "left"
			}
			pSetCapture.Call(uintptr(hwnd))
			w.emit("mousedown")
			if msg == wmLButtonDown {
				w.emit("click")
			}
		}
		return 0
	case wmLButtonUp, wmRButtonUp, wmMButtonUp:
		if w != nil {
			w.mouseX = float64(int16(lparam&0xFFFF)) / a.scale
			w.mouseY = float64(int16((lparam>>16)&0xFFFF)) / a.scale
			w.mouseDown = false
			pReleaseCapture.Call()
			w.emit("mouseup")
		}
		return 0
	}
	return defWindowProc(hwnd, msg, wparam, lparam)
}

var canvasProcCallback = syscall.NewCallback(canvasWndProc)
