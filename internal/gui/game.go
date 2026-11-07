package gui

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"agui/internal/lang"
)

// Game Kit: a deliberately small 2D runtime on top of the normal Agui
// window/canvas layer. It favors a fixed-step update, immediate-mode drawing
// and Win32/GDI+ primitives over a full engine.

type gameImage struct {
	img    uintptr
	path   string
	w, h   int32
	loaded bool
}

type particle struct {
	x, y, vx, vy float64
	life, max    float64
	size         float64
	color        uint32
	gravity      float64
}

type gameRuntime struct {
	app *App

	update, draw lang.Value
	running      bool
	paused       bool
	fps          float64
	step         float64
	acc          float64
	time         float64
	frame        int64
	timeScale    float64
	last         time.Time
	timer        uintptr

	keys     map[string]bool
	pressed  map[string]bool
	released map[string]bool
	bindings map[string][]string

	camX, camY, camZoom float64
	shakeAmp            float64
	shakeUntil          time.Time
	shakeX              float64
	shakeY              float64
	hasBounds           bool
	boundX, boundY      float64
	boundW, boundH      float64

	particles []particle
	rng       *rand.Rand

	pad        xinputState
	padOK      bool
	padChecked bool
	padButtons uint16
	padPrev    uint16
}

var (
	gameTimeBegin = winmm.NewProc("timeBeginPeriod")
	gameTimeEnd   = winmm.NewProc("timeEndPeriod")
	xinput14      = syscall.NewLazyDLL("xinput1_4.dll")
	xinput13      = syscall.NewLazyDLL("xinput1_3.dll")
	xinput910     = syscall.NewLazyDLL("xinput9_1_0.dll")
	xinputGet     *syscall.LazyProc
	xinputOnce    sync.Once
)

type xinputGamepad struct {
	Buttons      uint16
	LeftTrigger  uint8
	RightTrigger uint8
	ThumbLX      int16
	ThumbLY      int16
	ThumbRX      int16
	ThumbRY      int16
}

type xinputState struct {
	PacketNumber uint32
	Gamepad      xinputGamepad
}

func (a *App) ensureGame() *gameRuntime {
	if a.game == nil {
		a.game = &gameRuntime{
			app:       a,
			fps:       60,
			step:      1.0 / 60.0,
			timeScale: 1,
			keys:      map[string]bool{},
			pressed:   map[string]bool{},
			released:  map[string]bool{},
			bindings:  map[string][]string{},
			camZoom:   1,
			rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
			last:      time.Now(),
		}
	}
	return a.game
}

func (g *gameRuntime) setKey(name string, down bool) {
	name = normalizeKey(name)
	if name == "" {
		return
	}
	g.app.ensureGame()
	if down {
		if !g.keys[name] {
			g.pressed[name] = true
		}
		g.keys[name] = true
		g.released[name] = false
	} else {
		if g.keys[name] {
			g.released[name] = true
		}
		g.keys[name] = false
	}
}

func normalizeKey(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.ReplaceAll(name, " ", "_")
	switch name {
	case "return", "enter":
		return "enter"
	case "esc", "escape":
		return "escape"
	case "spacebar":
		return "space"
	case "control", "ctrl":
		return "ctrl"
	case "shift":
		return "shift"
	case "alt", "menu":
		return "alt"
	case "up", "down", "left", "right":
		return name
	}
	return name
}

func vkName(vk uint32) string {
	if vk >= 'A' && vk <= 'Z' {
		return strings.ToLower(string(rune(vk)))
	}
	if vk >= '0' && vk <= '9' {
		return string(rune(vk))
	}
	if vk >= 0x70 && vk <= 0x87 {
		return "f" + strconv.Itoa(int(vk-0x6F))
	}
	switch vk {
	case 0x08:
		return "backspace"
	case 0x09:
		return "tab"
	case 0x0D:
		return "enter"
	case 0x10:
		return "shift"
	case 0x11:
		return "ctrl"
	case 0x12:
		return "alt"
	case 0x1B:
		return "escape"
	case 0x20:
		return "space"
	case 0x25:
		return "left"
	case 0x26:
		return "up"
	case 0x27:
		return "right"
	case 0x28:
		return "down"
	case 0x2C:
		return "printscreen"
	case 0x2D:
		return "insert"
	case 0x2E:
		return "delete"
	case 0x24:
		return "home"
	case 0x23:
		return "end"
	case 0x21:
		return "pageup"
	case 0x22:
		return "pagedown"
	case 0xBA:
		return ";"
	case 0xBB:
		return "="
	case 0xBC:
		return ","
	case 0xBD:
		return "-"
	case 0xBE:
		return "."
	case 0xBF:
		return "/"
	case 0xC0:
		return "`"
	case 0xDB:
		return "["
	case 0xDC:
		return `\`
	case 0xDD:
		return "]"
	case 0xDE:
		return "'"
	}
	return "vk" + strconv.Itoa(int(vk))
}

// handleKeyMessage updates the game input state and emits window events.
func (a *App) handleKeyMessage(hwnd syscall.Handle, vk uint32, down bool) {
	w := a.widgets[hwnd]
	if w == nil {
		return
	}
	name := vkName(vk)
	w.key = name
	g := a.ensureGame()
	g.setKey(name, down)
	if down {
		w.emit("keydown")
	} else {
		w.emit("keyup")
	}
}

func (g *gameRuntime) tick() error {
	if !g.running || g.paused {
		return nil
	}
	now := time.Now()
	dt := now.Sub(g.last).Seconds()
	g.last = now
	if dt < 0 {
		dt = 0
	}
	if dt > 0.25 {
		dt = 0.25
	}
	dt *= g.timeScale
	g.acc += dt
	g.padChecked = false
	steps := 0
	for g.acc >= g.step && steps < 8 {
		g.time += g.step
		g.frame++
		g.defineTime()
		g.clearTransient()
		if g.update != nil {
			g.app.interp.CallArgs(g.update, []lang.Value{g.step})
		}
		g.acc -= g.step
		steps++
	}
	if steps == 0 && g.draw != nil {
		g.defineTime()
	}
	g.updateShake(now)
	if g.draw != nil {
		g.app.interp.CallArgs(g.draw, []lang.Value{g.acc / g.step})
	}
	g.endFrame()
	return nil
}

func (g *gameRuntime) defineTime() {
	g.app.interp.Define("dt", g.step)
	g.app.interp.Define("game_time", g.time)
	g.app.interp.Define("game_frame", float64(g.frame))
}

func (g *gameRuntime) clearTransient() {
	// pressed/released are kept for the whole visual frame; they are cleared
	// in endFrame, after draw has had a chance to inspect them.
}

func (g *gameRuntime) endFrame() {
	for k := range g.pressed {
		delete(g.pressed, k)
	}
	for k := range g.released {
		delete(g.released, k)
	}
}

func (g *gameRuntime) start(fps float64) {
	if fps < 1 {
		fps = 60
	}
	if fps > 240 {
		fps = 240
	}
	g.fps = fps
	g.step = 1 / fps
	g.running = true
	g.paused = false
	g.last = time.Now()
	g.acc = 0
	if g.timer == 0 {
		gameTimeBegin.Call(1)
		g.timer = g.app.addTimer(5, true, g.tick)
	}
}

func (g *gameRuntime) stop() {
	g.running = false
	if g.timer != 0 {
		g.app.cancelTimer(g.timer)
		g.timer = 0
		gameTimeEnd.Call(1)
	}
}

func (g *gameRuntime) updateShake(now time.Time) {
	g.shakeX, g.shakeY = 0, 0
	if g.shakeUntil.IsZero() || now.After(g.shakeUntil) || g.shakeAmp <= 0 {
		return
	}
	g.shakeX = (g.rng.Float64()*2 - 1) * g.shakeAmp
	g.shakeY = (g.rng.Float64()*2 - 1) * g.shakeAmp
}

func (g *gameRuntime) screen(x, y float64) (float64, float64) {
	sx := (x-g.camX)*g.camZoom + g.shakeX
	sy := (y-g.camY)*g.camZoom + g.shakeY
	return sx, sy
}

func (g *gameRuntime) clampCamera() {
	if !g.hasBounds {
		return
	}
	maxX := g.boundX + g.boundW - 1/g.camZoom
	maxY := g.boundY + g.boundH - 1/g.camZoom
	if g.camX < g.boundX {
		g.camX = g.boundX
	}
	if g.camY < g.boundY {
		g.camY = g.boundY
	}
	if g.camX > maxX {
		g.camX = maxX
	}
	if g.camY > maxY {
		g.camY = maxY
	}
}

func (g *gameRuntime) addParticle(p particle) {
	if p.max <= 0 {
		p.max = 0.8
	}
	p.life = p.max
	g.particles = append(g.particles, p)
}

func (g *gameRuntime) updateParticles(dt float64) {
	if dt <= 0 {
		return
	}
	out := g.particles[:0]
	for _, p := range g.particles {
		p.life -= dt
		if p.life <= 0 {
			continue
		}
		p.vy += p.gravity * dt
		p.x += p.vx * dt
		p.y += p.vy * dt
		out = append(out, p)
	}
	g.particles = out
}

func (g *gameRuntime) loadGameImage(path string) (*gameImage, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("图片路径为空")
	}
	if gi, ok := g.app.gameImages[path]; ok {
		return gi, nil
	}
	resolved, err := g.app.interp.AssetPath(path)
	if err != nil {
		return nil, err
	}
	img, w, h, err := loadGdiImage(resolved)
	if err != nil {
		return nil, err
	}
	gi := &gameImage{img: img, path: path, w: w, h: h, loaded: true}
	g.app.gameImages[path] = gi
	return gi, nil
}

func (g *gameRuntime) gameFont(family string, size float64, bold, italic bool) syscall.Handle {
	if size <= 0 {
		size = 16
	}
	if family == "" {
		family = "Segoe UI"
	}
	key := fmt.Sprintf("%s|%.1f|%t|%t", family, size, bold, italic)
	if h, ok := g.app.gameFonts[key]; ok {
		return h
	}
	h := createFont(family, size, bold, italic, g.app.dpi)
	if h != 0 {
		g.app.gameFonts[key] = h
	}
	return h
}

func pollXInput() (xinputState, bool) {
	xinputOnce.Do(func() {
		for _, dll := range []*syscall.LazyDLL{xinput14, xinput13, xinput910} {
			if err := dll.Load(); err == nil {
				xinputGet = dll.NewProc("XInputGetState")
				return
			}
		}
	})
	if xinputGet == nil {
		return xinputState{}, false
	}
	var st xinputState
	r, _, _ := xinputGet.Call(0, uintptr(unsafe.Pointer(&st)))
	return st, r == 0
}

func gamepadButtons(mask uint16) map[string]bool {
	m := map[string]bool{}
	for bit := uint16(1); bit != 0; bit <<= 1 {
		if mask&bit == 0 {
			continue
		}
		switch bit {
		case 0x0001:
			m["up"] = true
		case 0x0002:
			m["down"] = true
		case 0x0004:
			m["left"] = true
		case 0x0008:
			m["right"] = true
		case 0x0010:
			m["start"] = true
		case 0x0020:
			m["back"] = true
		case 0x1000:
			m["a"] = true
		case 0x2000:
			m["b"] = true
		case 0x4000:
			m["x"] = true
		case 0x8000:
			m["y"] = true
		}
	}
	return m
}

func (g *gameRuntime) padDown(name string) bool {
	name = normalizeKey(name)
	if name == "" {
		return false
	}
	if !g.padChecked {
		g.pad, g.padOK = pollXInput()
		g.padChecked = true
	}
	if !g.padOK {
		return false
	}
	return gamepadButtons(g.pad.Gamepad.Buttons)[name]
}

func (g *gameRuntime) keyDown(name string) bool {
	name = normalizeKey(name)
	if g.keys[name] {
		return true
	}
	for _, b := range g.bindings[name] {
		if g.keys[normalizeKey(b)] || g.padDown(b) {
			return true
		}
	}
	return g.padDown(name)
}

func valueStrings(v lang.Value) []string {
	switch t := v.(type) {
	case *lang.List:
		out := make([]string, 0, len(t.Items))
		for _, it := range t.Items {
			out = append(out, lang.ToString(it))
		}
		return out
	case *lang.Bytes:
		return []string{string(t.Data)}
	case nil:
		return nil
	default:
		return []string{lang.ToString(v)}
	}
}

func (g *gameRuntime) saveJSON(path string, v lang.Value) error {
	data, err := lang.EncodeJSON(v)
	if err != nil {
		return err
	}
	return os.WriteFile(g.app.interp.ResolvePath(path), data, 0644)
}

func (g *gameRuntime) loadJSON(path string, def lang.Value) (lang.Value, error) {
	data, err := os.ReadFile(g.app.interp.ResolvePath(path))
	if err != nil {
		if os.IsNotExist(err) {
			return def, nil
		}
		return nil, err
	}
	return lang.DecodeJSON(data)
}

func (w *Widget) setFullscreen(on bool) {
	if on == w.fullscreen {
		return
	}
	if on {
		w.windowedStyle = uint32(getWindowLong(w.hwnd, gwlStyle))
		if r, ok := getWindowRect(w.hwnd); ok {
			w.windowedRect = r
		}
		style := (w.windowedStyle &^ (wsCaption | wsThickFrame)) | wsPopup | wsVisible
		setWindowLong(w.hwnd, gwlStyle, uintptr(style))
		setWindowMenu(w.hwnd, 0)
		sw, sh := screenSize()
		moveWindow(w.hwnd, 0, 0, sw, sh, true)
		w.fullscreen = true
	} else {
		setWindowLong(w.hwnd, gwlStyle, uintptr(w.windowedStyle))
		if w.menuBar != 0 {
			setWindowMenu(w.hwnd, w.menuBar)
		}
		moveWindow(w.hwnd, w.windowedRect.left, w.windowedRect.top,
			w.windowedRect.right-w.windowedRect.left,
			w.windowedRect.bottom-w.windowedRect.top, true)
		w.fullscreen = false
	}
	invalidateRect(w.hwnd, nil, true)
}

func (w *Widget) setBorderless(on bool) {
	style := getWindowLong(w.hwnd, gwlStyle)
	if on {
		w.windowedStyle = uint32(style)
		style &^= wsCaption | wsThickFrame
		setWindowLong(w.hwnd, gwlStyle, style)
		setWindowMenu(w.hwnd, 0)
	} else {
		style |= wsCaption | wsThickFrame
		setWindowLong(w.hwnd, gwlStyle, style)
		if w.menuBar != 0 {
			setWindowMenu(w.hwnd, w.menuBar)
		}
	}
	invalidateRect(w.hwnd, nil, true)
}

func (w *Widget) setWindowOpacity(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	if v >= 1 {
		setWindowLong(w.hwnd, gwlExStyle, getWindowLong(w.hwnd, gwlExStyle)&^wsExLayered)
		w.opacity = 1
		return
	}
	setWindowLong(w.hwnd, gwlExStyle, getWindowLong(w.hwnd, gwlExStyle)|wsExLayered)
	pSetLayeredWindowAttributes.Call(uintptr(w.hwnd), 0, uintptr(uint8(v*255)), lwaAlpha)
	w.opacity = v
}

func (w *Widget) toggleFullscreen()  { w.setFullscreen(!w.fullscreen) }
func (w *Widget) isFullscreen() bool { return w.fullscreen }
func (w *Widget) keyValue() string   { return w.key }

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// --- Canvas image/sprite/tilemap -------------------------------------------

func gameNumber(v lang.Value) float64 {
	if n, ok := v.(float64); ok {
		return n
	}
	s := strings.TrimSpace(lang.ToString(v))
	if s == "" {
		return 0
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return n
}

func gameArg(args []lang.Value, i int, def float64) float64 {
	if i < 0 || i >= len(args) || args[i] == nil {
		return def
	}
	return gameNumber(args[i])
}

func (w *Widget) canvasImage(args []lang.Value, kwargs map[string]lang.Value) error {
	if len(args) < 3 {
		return fmt.Errorf("image(path, x, y, w=, h=) 至少需要路径和坐标")
	}
	path := lang.ToString(args[0])
	x, y := gameArg(args, 1, 0), gameArg(args, 2, 0)
	dw, dh := gameArg(args, 3, 0), gameArg(args, 4, 0)
	if v, ok := kwargs["w"]; ok {
		dw = gameNumber(v)
	}
	if v, ok := kwargs["h"]; ok {
		dh = gameNumber(v)
	}
	g := w.app.ensureGame()
	gi, err := g.loadGameImage(path)
	if err != nil {
		return err
	}
	sx := gameArg(args, 5, 0)
	sy := gameArg(args, 6, 0)
	sw := gameArg(args, 7, 0)
	sh := gameArg(args, 8, 0)
	if v, ok := kwargs["sx"]; ok {
		sx = gameNumber(v)
	}
	if v, ok := kwargs["sy"]; ok {
		sy = gameNumber(v)
	}
	if v, ok := kwargs["sw"]; ok {
		sw = gameNumber(v)
	}
	if v, ok := kwargs["sh"]; ok {
		sh = gameNumber(v)
	}
	frame := int(gameArg(args, 9, 0))
	if v, ok := kwargs["frame"]; ok {
		frame = int(gameNumber(v))
	}
	frames := int(gameArg(args, 10, 0))
	if v, ok := kwargs["frames"]; ok {
		frames = int(gameNumber(v))
	}
	cols := frames
	if v, ok := kwargs["cols"]; ok {
		cols = int(gameNumber(v))
	}
	rows := 1
	if v, ok := kwargs["rows"]; ok {
		rows = int(gameNumber(v))
	}
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	if frames > 0 && sw <= 0 && sh <= 0 {
		sw = float64(gi.w) / float64(cols)
		sh = float64(gi.h) / float64(rows)
		frame %= frames
		if frame < 0 {
			frame += frames
		}
		sx = float64(frame%cols) * sw
		sy = float64(frame/cols) * sh
	}
	if sw <= 0 {
		sw = float64(gi.w)
	}
	if sh <= 0 {
		sh = float64(gi.h)
	}
	if dw <= 0 {
		dw = sw
	}
	if dh <= 0 {
		dh = sh
	}
	w.canvasAdd(canvasOp{
		kind: "image", image: path,
		x1: x, y1: y, x2: dw, y2: dh,
		sx: sx, sy: sy, sw: sw, sh: sh,
		screen: lang.Truthy(kwargs["screen"]),
	})
	return nil
}

func (w *Widget) canvasSprite(args []lang.Value, kwargs map[string]lang.Value) error {
	return w.canvasImage(args, kwargs)
}

func (w *Widget) canvasTilemap(args []lang.Value, kwargs map[string]lang.Value) error {
	if len(args) < 4 {
		return fmt.Errorf("tilemap(tiles, path, tile_w, tile_h) 需要四个参数")
	}
	tiles, ok := args[0].(*lang.List)
	if !ok {
		return fmt.Errorf("tilemap 第一参数必须是二维列表")
	}
	path := lang.ToString(args[1])
	tw, th := gameArg(args, 2, 32), gameArg(args, 3, 32)
	ox, oy := 0.0, 0.0
	if v, ok := kwargs["x"]; ok {
		ox = gameNumber(v)
	}
	if v, ok := kwargs["y"]; ok {
		oy = gameNumber(v)
	}
	g := w.app.ensureGame()
	gi, err := g.loadGameImage(path)
	if err != nil {
		return err
	}
	srcW, srcH := tw, th
	if v, ok := kwargs["src_w"]; ok {
		srcW = gameNumber(v)
	}
	if v, ok := kwargs["src_h"]; ok {
		srcH = gameNumber(v)
	}
	cols := 1
	if v, ok := kwargs["cols"]; ok {
		cols = int(gameNumber(v))
	}
	if cols < 1 {
		cols = 1
	}
	if srcW <= 0 {
		srcW = float64(gi.w) / float64(cols)
	}
	screen := lang.Truthy(kwargs["screen"])
	for row, rv := range tiles.Items {
		rowList, ok := rv.(*lang.List)
		if !ok {
			continue
		}
		for col, cv := range rowList.Items {
			idx := int(gameNumber(cv))
			if idx <= 0 {
				continue
			}
			frame := idx - 1
			sx := float64(frame%cols) * srcW
			sy := float64(frame/cols) * srcH
			w.canvasAdd(canvasOp{
				kind: "image", image: path,
				x1: ox + float64(col)*tw, y1: oy + float64(row)*th,
				x2: tw, y2: th,
				sx: sx, sy: sy, sw: srcW, sh: srcH,
				screen: screen,
			})
		}
	}
	return nil
}

// --- Particles -------------------------------------------------------------

func (g *gameRuntime) emitParticles(x, y float64, count int, speed, life, size float64, color uint32, gravity float64) {
	if count < 1 {
		count = 1
	}
	if count > 2000 {
		count = 2000
	}
	for i := 0; i < count; i++ {
		ang := g.rng.Float64() * math.Pi * 2
		sp := speed * (0.25 + g.rng.Float64()*0.75)
		p := particle{
			x: x, y: y,
			vx: math.Cos(ang) * sp, vy: math.Sin(ang) * sp,
			life: life, max: life, size: size,
			color: color, gravity: gravity,
		}
		g.addParticle(p)
	}
}

func (g *gameRuntime) drawParticles(w *Widget) {
	for _, p := range g.particles {
		alpha := p.life / p.max
		if alpha <= 0 {
			continue
		}
		rad := p.size * (0.4 + alpha*0.8)
		w.canvasAdd(canvasOp{kind: "circle", x1: p.x, y1: p.y, r: rad, color: p.color, fill: true, alpha: alpha, useAlpha: true})
	}
}

// --- Input -----------------------------------------------------------------

func (a *App) nativeGameLoop(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("game_loop(update, draw=, fps=) 需要 update 函数")
	}
	g := a.ensureGame()
	g.update = args[0]
	if len(args) > 1 {
		g.draw = args[1]
	}
	fps := 60.0
	if v, ok := kwargs["fps"]; ok {
		fps = gameNumber(v)
	}
	if len(args) > 2 {
		fps = gameNumber(args[2])
	}
	if v, ok := kwargs["draw"]; ok {
		g.draw = v
	}
	g.defineTime()
	g.start(fps)
	return nil, nil
}

func (a *App) nativeKeyDown(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	name := ""
	if len(args) > 0 {
		name = lang.ToString(args[0])
	} else if v, ok := kwargs["key"]; ok {
		name = lang.ToString(v)
	}
	return a.ensureGame().keyDown(name), nil
}

func (a *App) nativeKeyPressed(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	name := ""
	if len(args) > 0 {
		name = lang.ToString(args[0])
	}
	g := a.ensureGame()
	if g.pressed[normalizeKey(name)] {
		return true, nil
	}
	for _, b := range g.bindings[normalizeKey(name)] {
		if g.pressed[normalizeKey(b)] {
			return true, nil
		}
	}
	return false, nil
}

func (a *App) nativeKeyReleased(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	name := ""
	if len(args) > 0 {
		name = lang.ToString(args[0])
	}
	g := a.ensureGame()
	if g.released[normalizeKey(name)] {
		return true, nil
	}
	for _, b := range g.bindings[normalizeKey(name)] {
		if g.released[normalizeKey(b)] {
			return true, nil
		}
	}
	return false, nil
}

func (a *App) nativeBind(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("bind(action, key...) 至少需要一个动作和一个按键")
	}
	g := a.ensureGame()
	action := normalizeKey(lang.ToString(args[0]))
	for _, v := range args[1:] {
		for _, s := range valueStrings(v) {
			g.bindings[action] = append(g.bindings[action], normalizeKey(s))
		}
	}
	return nil, nil
}

func (a *App) nativeActionDown(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("action_down(action) 需要一个动作名")
	}
	return a.ensureGame().keyDown(lang.ToString(args[0])), nil
}

func (a *App) nativeActionPressed(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("action_pressed(action) 需要一个动作名")
	}
	return a.nativeKeyPressed(args, kwargs)
}

func (a *App) nativeGamepadButton(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("gamepad_button(name) 需要一个按键名")
	}
	return a.ensureGame().padDown(lang.ToString(args[0])), nil
}

func (a *App) nativeGamepadAxis(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	name := "lx"
	if len(args) > 0 {
		name = normalizeKey(lang.ToString(args[0]))
	}
	g := a.ensureGame()
	if !g.padOK {
		g.pad, g.padOK = pollXInput()
	}
	if !g.padOK {
		return 0.0, nil
	}
	pad := g.pad.Gamepad
	switch name {
	case "lx", "left_x", "move_x":
		return axis(pad.ThumbLX), nil
	case "ly", "left_y", "move_y":
		return axis(pad.ThumbLY), nil
	case "rx", "right_x":
		return axis(pad.ThumbRX), nil
	case "ry", "right_y":
		return axis(pad.ThumbRY), nil
	case "lt", "left_trigger":
		return float64(pad.LeftTrigger) / 255, nil
	case "rt", "right_trigger":
		return float64(pad.RightTrigger) / 255, nil
	}
	return 0.0, nil
}

func axis(v int16) float64 {
	f := float64(v) / 32767
	if math.Abs(f) < 0.2 {
		return 0
	}
	if f < -1 {
		f = -1
	}
	if f > 1 {
		f = 1
	}
	return f
}

// --- Camera, save, fullscreen ----------------------------------------------

func (a *App) nativeCamera(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	g := a.ensureGame()
	if len(args) == 0 && len(kwargs) == 0 {
		m := lang.NewMap()
		m.Set("x", g.camX)
		m.Set("y", g.camY)
		m.Set("zoom", g.camZoom)
		return m, nil
	}
	if v, ok := kwargs["x"]; ok {
		g.camX = gameNumber(v)
	} else if len(args) > 0 {
		g.camX = gameNumber(args[0])
	}
	if v, ok := kwargs["y"]; ok {
		g.camY = gameNumber(v)
	} else if len(args) > 1 {
		g.camY = gameNumber(args[1])
	}
	if v, ok := kwargs["zoom"]; ok {
		g.camZoom = gameNumber(v)
	} else if len(args) > 2 {
		g.camZoom = gameNumber(args[2])
	}
	if g.camZoom < 0.05 {
		g.camZoom = 0.05
	}
	g.clampCamera()
	return nil, nil
}

func (a *App) nativeCameraShake(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	g := a.ensureGame()
	amp := 8.0
	dur := 0.3
	if len(args) > 0 {
		amp = gameNumber(args[0])
	}
	if len(args) > 1 {
		dur = gameNumber(args[1])
	}
	g.shakeAmp = amp
	g.shakeUntil = time.Now().Add(time.Duration(dur * float64(time.Second)))
	return nil, nil
}

func (a *App) nativeCameraBounds(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	g := a.ensureGame()
	if len(args) < 4 {
		return nil, fmt.Errorf("camera_bounds(x, y, w, h) 需要四个参数")
	}
	g.boundX, g.boundY = gameNumber(args[0]), gameNumber(args[1])
	g.boundW, g.boundH = gameNumber(args[2]), gameNumber(args[3])
	g.hasBounds = true
	g.clampCamera()
	return nil, nil
}

func (a *App) nativeSaveJSON(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("save_json(path, value) 需要两个参数")
	}
	return nil, a.ensureGame().saveJSON(lang.ToString(args[0]), args[1])
}

func (a *App) nativeLoadJSON(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("load_json(path, default=null) 需要路径")
	}
	def := lang.Value(nil)
	if len(args) > 1 {
		def = args[1]
	}
	if v, ok := kwargs["default"]; ok {
		def = v
	}
	return a.ensureGame().loadJSON(lang.ToString(args[0]), def)
}

func (w *Widget) nativeFullscreen(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) == 0 {
		return w.fullscreen, nil
	}
	w.setFullscreen(lang.Truthy(args[0]))
	return w.fullscreen, nil
}

func (w *Widget) nativeBorderless(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	on := true
	if len(args) > 0 {
		on = lang.Truthy(args[0])
	}
	w.setBorderless(on)
	return nil, nil
}

// --- Particles, tile collision, ray ---------------------------------------

func (a *App) findCanvas() *Widget {
	for _, w := range a.widgets {
		if w.kind == "canvas" {
			return w
		}
	}
	return nil
}

func (a *App) nativeParticlesEmit(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	g := a.ensureGame()
	x, y := 0.0, 0.0
	if len(args) > 0 {
		x = gameNumber(args[0])
	}
	if len(args) > 1 {
		y = gameNumber(args[1])
	}
	count := 12
	if len(args) > 2 {
		count = int(gameNumber(args[2]))
	}
	if v, ok := kwargs["count"]; ok {
		count = int(gameNumber(v))
	}
	speed := 120.0
	if v, ok := kwargs["speed"]; ok {
		speed = gameNumber(v)
	}
	life := 0.7
	if v, ok := kwargs["life"]; ok {
		life = gameNumber(v)
	}
	size := 4.0
	if v, ok := kwargs["size"]; ok {
		size = gameNumber(v)
	}
	gravity := 240.0
	if v, ok := kwargs["gravity"]; ok {
		gravity = gameNumber(v)
	}
	color := canvasColor(kwargs["color"], 0xF59E0B)
	g.emitParticles(x, y, count, speed, life, size, color, gravity)
	return nil, nil
}

func (a *App) nativeParticlesUpdate(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	g := a.ensureGame()
	dt := g.step
	if len(args) > 0 {
		dt = gameNumber(args[0])
	}
	g.updateParticles(dt)
	return nil, nil
}

func (a *App) nativeParticlesDraw(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	var c *Widget
	if len(args) > 0 {
		if w, ok := args[0].(*Widget); ok {
			c = w
		}
	}
	if c == nil {
		c = a.findCanvas()
	}
	if c == nil {
		return nil, fmt.Errorf("particles_draw 需要 canvas")
	}
	a.ensureGame().drawParticles(c)
	return nil, nil
}

func (a *App) nativeParticlesClear(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	a.ensureGame().particles = nil
	return nil, nil
}

func tileAt(tiles *lang.List, col, row int) int {
	if row < 0 || row >= len(tiles.Items) {
		return 0
	}
	r, ok := tiles.Items[row].(*lang.List)
	if !ok || col < 0 || col >= len(r.Items) {
		return 0
	}
	return int(gameNumber(r.Items[col]))
}

func (a *App) nativeTilemapCollide(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) < 5 {
		return nil, fmt.Errorf("tilemap_collide(tiles, x, y, tile_w, tile_h) 需要五个参数")
	}
	tiles, ok := args[0].(*lang.List)
	if !ok {
		return nil, fmt.Errorf("tilemap_collide 需要二维列表")
	}
	x, y := gameNumber(args[1]), gameNumber(args[2])
	tw, th := gameNumber(args[3]), gameNumber(args[4])
	if tw <= 0 {
		tw = 32
	}
	if th <= 0 {
		th = 32
	}
	return tileAt(tiles, int(x/tw), int(y/th)) > 0, nil
}

func (a *App) nativeAABB(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) < 8 {
		return nil, fmt.Errorf("aabb(x1,y1,w1,h1,x2,y2,w2,h2) 需要八个参数")
	}
	x1, y1 := gameNumber(args[0]), gameNumber(args[1])
	w1, h1 := gameNumber(args[2]), gameNumber(args[3])
	x2, y2 := gameNumber(args[4]), gameNumber(args[5])
	w2, h2 := gameNumber(args[6]), gameNumber(args[7])
	return x1 < x2+w2 && x1+w1 > x2 && y1 < y2+h2 && y1+h1 > y2, nil
}

func (a *App) nativeRayRect(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	if len(args) < 8 {
		return nil, fmt.Errorf("ray_rect(x,y,dx,dy,rx,ry,rw,rh) 需要八个参数")
	}
	x, y := gameNumber(args[0]), gameNumber(args[1])
	dx, dy := gameNumber(args[2]), gameNumber(args[3])
	rx, ry := gameNumber(args[4]), gameNumber(args[5])
	rw, rh := gameNumber(args[6]), gameNumber(args[7])
	tmin, tmax := 0.0, math.Inf(1)
	nx, ny := 0.0, 0.0
	if math.Abs(dx) < 1e-9 {
		if x < rx || x > rx+rw {
			return rayHit(false, 0, 0, 0), nil
		}
	} else {
		t1 := (rx - x) / dx
		t2 := (rx + rw - x) / dx
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > tmin {
			tmin = t1
			nx = -1
			ny = 0
		}
		if t2 < tmax {
			tmax = t2
		}
	}
	if math.Abs(dy) < 1e-9 {
		if y < ry || y > ry+rh {
			return rayHit(false, 0, 0, 0), nil
		}
	} else {
		t1 := (ry - y) / dy
		t2 := (ry + rh - y) / dy
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > tmin {
			tmin = t1
			nx = 0
			ny = -1
		}
		if t2 < tmax {
			tmax = t2
		}
	}
	if tmax < tmin || tmax < 0 {
		return rayHit(false, 0, 0, 0), nil
	}
	return rayHit(true, tmin, nx, ny), nil
}

func rayHit(hit bool, t, nx, ny float64) *lang.Map {
	m := lang.NewMap()
	m.Set("hit", hit)
	m.Set("t", t)
	m.Set("nx", nx)
	m.Set("ny", ny)
	return m
}

// --- Native registration ---------------------------------------------------

func registerGame(i *lang.Interp, a *App) {
	reg := func(name string, fn func(*lang.Interp, []lang.Value, map[string]lang.Value) (lang.Value, error)) {
		i.RegisterNative(name, fn)
	}
	regA := func(name string, fn func([]lang.Value, map[string]lang.Value) (lang.Value, error)) {
		reg(name, func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
			return fn(args, kwargs)
		})
	}
	reg("game_loop", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		return a.nativeGameLoop(args, kwargs)
	})
	reg("game_stop", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		a.ensureGame().stop()
		return nil, nil
	})
	reg("game_pause", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		a.ensureGame().paused = true
		return nil, nil
	})
	reg("game_resume", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		g := a.ensureGame()
		g.paused = false
		g.last = time.Now()
		return nil, nil
	})
	reg("game_set_time_scale", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		g := a.ensureGame()
		if len(args) > 0 {
			g.timeScale = gameNumber(args[0])
		} else if v, ok := kwargs["scale"]; ok {
			g.timeScale = gameNumber(v)
		}
		if g.timeScale < 0 {
			g.timeScale = 0
		}
		return g.timeScale, nil
	})
	reg("game_running", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		return a.ensureGame().running, nil
	})
	reg("game_paused", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		return a.ensureGame().paused, nil
	})
	reg("time_scale", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		g := a.ensureGame()
		if len(args) > 0 {
			g.timeScale = gameNumber(args[0])
		}
		return g.timeScale, nil
	})
	regA("anim", a.nativeAnim)
	regA("key_down", a.nativeKeyDown)
	regA("key_pressed", a.nativeKeyPressed)
	regA("key_released", a.nativeKeyReleased)
	regA("action_down", a.nativeActionDown)
	regA("action_pressed", a.nativeActionPressed)
	regA("bind", a.nativeBind)
	reg("keys_down", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		g := a.ensureGame()
		out := lang.NewList()
		for _, k := range sortedKeys(g.keys) {
			out.Items = append(out.Items, k)
		}
		return out, nil
	})
	reg("gamepad_connected", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		g := a.ensureGame()
		if !g.padChecked {
			g.pad, g.padOK = pollXInput()
			g.padChecked = true
		}
		return g.padOK, nil
	})
	regA("gamepad_button", a.nativeGamepadButton)
	regA("gamepad_axis", a.nativeGamepadAxis)
	regA("camera", a.nativeCamera)
	regA("camera_shake", a.nativeCameraShake)
	regA("camera_bounds", a.nativeCameraBounds)
	regA("save_json", a.nativeSaveJSON)
	regA("load_json", a.nativeLoadJSON)
	regA("tilemap_collide", a.nativeTilemapCollide)
	regA("aabb", a.nativeAABB)
	regA("ray_rect", a.nativeRayRect)
	regA("particles_emit", a.nativeParticlesEmit)
	regA("particles_update", a.nativeParticlesUpdate)
	regA("particles_draw", a.nativeParticlesDraw)
	regA("particles_clear", a.nativeParticlesClear)
	reg("fullscreen", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		w := a.EnsureWindow()
		return w.nativeFullscreen(args, kwargs)
	})
	reg("borderless", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		w := a.EnsureWindow()
		return w.nativeBorderless(args, kwargs)
	})
	reg("audio_play", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("audio_play(path, channel=\"bgm\", loop=false, volume=100) 需要路径")
		}
		channel := "bgm"
		if len(args) > 1 {
			channel = lang.ToString(args[1])
		}
		loop := false
		volume := 100
		if len(args) > 2 {
			loop = lang.Truthy(args[2])
		}
		if v, ok := kwargs["loop"]; ok {
			loop = lang.Truthy(v)
		}
		if v, ok := kwargs["volume"]; ok {
			volume = int(gameNumber(v))
		}
		if v, ok := kwargs["channel"]; ok {
			channel = lang.ToString(v)
		}
		return nil, a.playAudio(lang.ToString(args[0]), channel, loop, volume)
	})
	reg("audio_stop", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		channel := "bgm"
		if len(args) > 0 {
			channel = lang.ToString(args[0])
		}
		a.stopAudio(channel)
		return nil, nil
	})
	reg("audio_playing", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		channel := "bgm"
		if len(args) > 0 {
			channel = lang.ToString(args[0])
		}
		return a.audioPlaying(channel), nil
	})
	reg("audio_volume", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		channel := "bgm"
		volume := 100
		if len(args) > 0 {
			channel = lang.ToString(args[0])
		}
		if len(args) > 1 {
			volume = int(gameNumber(args[1]))
		}
		a.audioVolume(channel, volume)
		return nil, nil
	})
	reg("audio_stop_all", func(_ *lang.Interp, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
		for channel := range a.audio {
			a.stopAudio(channel)
		}
		return nil, nil
	})

	i.Define("dt", 0.0)
	i.Define("game_time", 0.0)
	i.Define("game_frame", 0.0)
}
