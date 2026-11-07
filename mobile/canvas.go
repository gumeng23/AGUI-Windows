package mobile

import (
	"encoding/json"
	"fmt"

	"agui/internal/lang"
)

// canvasOp is the wire format sent to the Java CanvasView. Keeping the whole
// frame in one JSON string means one JNI call per repaint instead of one per
// primitive.
type canvasOp struct {
	Kind   string  `json:"kind"`
	X1     float64 `json:"x1"`
	Y1     float64 `json:"y1"`
	X2     float64 `json:"x2"`
	Y2     float64 `json:"y2"`
	X3     float64 `json:"x3"`
	Y3     float64 `json:"y3"`
	Radius float64 `json:"r"`
	Text   string  `json:"text"`
	Color  string  `json:"color"`
	Width  float64 `json:"width"`
	Size   float64 `json:"size"`
	Fill   bool    `json:"fill"`
	Src    string  `json:"src"`
	SX     float64 `json:"sx"`
	SY     float64 `json:"sy"`
	SW     float64 `json:"sw"`
	SH     float64 `json:"sh"`
	Screen bool    `json:"screen,omitempty"`
}

func (w *Widget) addCanvasOp(op canvasOp) {
	if w == nil || w.kind != "canvas" {
		return
	}
	w.app.transformCanvasOp(&op)
	w.ops = append(w.ops, op)
}

func (w *Widget) refreshCanvas() {
	if w == nil || w.kind != "canvas" || w.batch || w.app.bridge == nil {
		return
	}
	data, _ := json.Marshal(w.ops)
	w.app.bridge.CanvasOps(w.id, string(data))
}

func (w *Widget) canvasClear() {
	w.ops = nil
	w.refreshCanvas()
}

// DispatchTouch is called by the Java CanvasView. action follows
// MotionEvent: 0=down, 1=up, 2=move, 3=cancel.
func DispatchTouch(id int, action int, x float64, y float64) {
	currentMu.Lock()
	a := current
	currentMu.Unlock()
	if a == nil {
		return
	}
	w := a.get(id)
	if w == nil || w.kind != "canvas" {
		return
	}
	w.mouseX, w.mouseY = x, y
	if a.game != nil {
		a.game.setPointer(x, y, action)
	}
	switch action {
	case 0:
		w.mouseDown = true
		w.mouseBtn = "left"
		w.emit("mousedown")
	case 1:
		w.mouseDown = false
		w.emit("mouseup")
		w.emit("click")
		w.mouseBtn = ""
	case 2:
		w.emit("mousemove")
	}
}

func (w *Widget) canvasCall(name string, args []lang.Value, kw map[string]lang.Value) (lang.Value, bool, error) {
	arg := func(i int) float64 {
		if i < len(args) {
			return number(args[i])
		}
		return 0
	}
	str := func(i int) string {
		if i < len(args) {
			return lang.ToString(args[i])
		}
		return ""
	}
	screen := kwBool(kw, "screen", false)
	add := func(op canvasOp) {
		op.Screen = screen
		w.addCanvasOp(op)
	}
	color := kwString(kw, "color", "#38bdf8")
	width := kwNumber(kw, "width", 2)
	switch name {
	case "line":
		if len(args) < 4 {
			return nil, true, fmt.Errorf("line 需要 x1,y1,x2,y2")
		}
		add(canvasOp{Kind: "line", X1: arg(0), Y1: arg(1), X2: arg(2), Y2: arg(3), Color: color, Width: width})
	case "rect":
		if len(args) < 4 {
			return nil, true, fmt.Errorf("rect 需要 x,y,w,h")
		}
		add(canvasOp{Kind: "rect", X1: arg(0), Y1: arg(1), X2: arg(2), Y2: arg(3), Color: color, Width: width, Fill: kwBool(kw, "fill", true)})
	case "circle":
		if len(args) < 3 {
			return nil, true, fmt.Errorf("circle 需要 x,y,r")
		}
		add(canvasOp{Kind: "circle", X1: arg(0), Y1: arg(1), Radius: arg(2), Color: color, Width: width, Fill: kwBool(kw, "fill", true)})
	case "text":
		if len(args) < 3 {
			return nil, true, fmt.Errorf("text 需要 x,y,文字")
		}
		add(canvasOp{Kind: "text", X1: arg(0), Y1: arg(1), Text: str(2), Color: color, Size: kwNumber(kw, "size", 18)})
	case "image", "sprite":
		if len(args) < 3 {
			return nil, true, fmt.Errorf("%s 需要 图片,x,y", name)
		}
		op := canvasOp{Kind: "image", Src: str(0), X1: arg(1), Y1: arg(2), X2: kwNumber(kw, "w", 0), Y2: kwNumber(kw, "h", 0)}
		if op.X2 == 0 {
			op.X2 = 64
		}
		if op.Y2 == 0 {
			op.Y2 = 64
		}
		op.SX = kwNumber(kw, "sx", 0)
		op.SY = kwNumber(kw, "sy", 0)
		op.SW = kwNumber(kw, "sw", 0)
		op.SH = kwNumber(kw, "sh", 0)
		add(op)
	case "tilemap":
		if len(args) < 1 {
			return nil, true, fmt.Errorf("tilemap 需要二维图块列表")
		}
		rows, ok := args[0].(*lang.List)
		if !ok {
			return nil, true, fmt.Errorf("tilemap 第一个参数必须是列表")
		}
		tileW := kwNumber(kw, "tile_w", 32)
		tileH := kwNumber(kw, "tile_h", 32)
		ox := kwNumber(kw, "x", 0)
		oy := kwNumber(kw, "y", 0)
		for y, rv := range rows.Items {
			row, ok := rv.(*lang.List)
			if !ok {
				continue
			}
			for x, tv := range row.Items {
				n := number(tv)
				if n == 0 {
					continue
				}
				add(canvasOp{Kind: "rect", X1: ox + float64(x)*tileW, Y1: oy + float64(y)*tileH, X2: tileW, Y2: tileH, Color: color, Fill: true})
			}
		}
	case "clear":
		w.ops = nil
		w.refreshCanvas()
	case "flip":
		w.batch = false
		w.refreshCanvas()
	case "batch_begin":
		w.batch = true
	case "batch_end":
		w.batch = false
		w.refreshCanvas()
	case "redraw":
		w.refreshCanvas()
	default:
		return nil, false, nil
	}
	return nil, true, nil
}

func (a *App) firstCanvas() *Widget {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, w := range a.widgets {
		if w.kind == "canvas" {
			return w
		}
	}
	return nil
}

func (a *App) transformCanvasOp(op *canvasOp) {
	g := a.game
	if g == nil || !g.cameraSet || op.Screen {
		return
	}
	z := g.cameraZoom
	if z <= 0 {
		z = 1
	}
	tx := func(x, y float64) (float64, float64) {
		return (x-g.cameraX)*z + g.shakeX, (y-g.cameraY)*z + g.shakeY
	}
	switch op.Kind {
	case "line":
		op.X1, op.Y1 = tx(op.X1, op.Y1)
		op.X2, op.Y2 = tx(op.X2, op.Y2)
		op.Width *= z
	case "rect", "image", "text":
		op.X1, op.Y1 = tx(op.X1, op.Y1)
		op.X2 *= z
		op.Y2 *= z
		op.Width *= z
		op.Size *= z
	case "circle":
		op.X1, op.Y1 = tx(op.X1, op.Y1)
		op.Radius *= z
		op.Width *= z
	}
}
