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
	invalidateRect(w.hwnd, nil, true)
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
	})
	return nil
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
	a := w.app
	var bg uint32 = 0xFFFFFF
	if w.hasBg {
		bg = w.bg
	}
	brush := createBrush(bg)
	cr, _ := getClientRect(w.hwnd)
	r := rect{left: 0, top: 0, right: cr.right, bottom: cr.bottom}
	fillRect(hdc, &r, brush)
	deleteObject(brush)

	for _, op := range w.canvasOps {
		x1, y1 := a.px(op.x1), a.px(op.y1)
		switch op.kind {
		case "line":
			pen := createPen(op.color, a.px(float64(op.width)))
			old := selectObject(hdc, pen)
			pMoveToEx.Call(uintptr(hdc), uintptr(x1), uintptr(y1), 0)
			pLineTo.Call(uintptr(hdc), uintptr(a.px(op.x2)), uintptr(a.px(op.y2)))
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
			pRectangle.Call(uintptr(hdc), uintptr(x1), uintptr(y1),
				uintptr(x1+a.px(op.x2)), uintptr(y1+a.px(op.y2)))
			selectObject(hdc, oldBrush)
			if op.fill {
				deleteObject(b)
			}
			selectObject(hdc, oldPen)
			deleteObject(pen)
		case "circle":
			rr := a.px(op.r)
			pen := createPen(op.color, a.px(float64(op.width)))
			oldPen := selectObject(hdc, pen)
			var b syscall.Handle
			if op.fill {
				b = createBrush(op.color)
			} else {
				b = stockObject(nullBrush)
			}
			oldBrush := selectObject(hdc, b)
			pEllipse.Call(uintptr(hdc), uintptr(x1-rr), uintptr(y1-rr),
				uintptr(x1+rr), uintptr(y1+rr))
			selectObject(hdc, oldBrush)
			if op.fill {
				deleteObject(b)
			}
			selectObject(hdc, oldPen)
			deleteObject(pen)
		case "text":
			setBkMode(hdc, transparent)
			setTextColor(hdc, op.color)
			hf := createFont(op.fontFamily, op.size, false, false, a.dpi)
			old := selectObject(hdc, hf)
			ptr := u16(op.text)
			pTextOutW.Call(uintptr(hdc), uintptr(x1), uintptr(y1),
				uintptr(unsafe.Pointer(ptr)), uintptr(len([]rune(op.text))))
			selectObject(hdc, old)
			deleteObject(hf)
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
	case wmLButtonDown:
		if w != nil {
			w.mouseX = float64(int16(lparam&0xFFFF)) / a.scale
			w.mouseY = float64(int16((lparam>>16)&0xFFFF)) / a.scale
			w.emit("click")
		}
		return 0
	}
	return defWindowProc(hwnd, msg, wparam, lparam)
}

var canvasProcCallback = syscall.NewCallback(canvasWndProc)
