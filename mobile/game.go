package mobile

import (
	"fmt"
	"math"
	"sync"
	"time"

	"agui/internal/lang"
)

type gameState struct {
	mu        sync.Mutex
	update    lang.Value
	draw      lang.Value
	fps       float64
	running   bool
	paused    bool
	timeScale float64
	stop      chan struct{}
	once      sync.Once

	cameraSet  bool
	cameraX    float64
	cameraY    float64
	cameraZoom float64
	shakeX     float64
	shakeY     float64
	shakeTime  float64
	shakePower float64

	touchX       float64
	touchY       float64
	touchDown    bool
	touchPressed bool
	gameTime     float64
	frame        int
}

func (a *App) startGame(update, draw lang.Value, fps float64) error {
	if update == nil {
		return fmt.Errorf("game_loop 需要 update 函数")
	}
	if fps <= 0 || fps > 240 {
		fps = 60
	}
	g := &gameState{update: update, draw: draw, fps: fps, running: true, timeScale: 1, stop: make(chan struct{})}
	a.mu.Lock()
	if a.game != nil {
		old := a.game
		a.mu.Unlock()
		old.stopNow()
		a.mu.Lock()
	}
	a.game = g
	a.mu.Unlock()
	go g.loop(a)
	return nil
}

func (g *gameState) stopNow() {
	g.once.Do(func() { close(g.stop) })
	g.mu.Lock()
	g.running = false
	g.mu.Unlock()
}

func (g *gameState) loop(a *App) {
	step := time.Duration(float64(time.Second) / g.fps)
	ticker := time.NewTicker(step)
	defer ticker.Stop()
	last := time.Now()
	for {
		select {
		case <-g.stop:
			return
		case now := <-ticker.C:
			g.mu.Lock()
			paused := g.paused
			scale := g.timeScale
			g.mu.Unlock()
			if paused {
				last = now
				continue
			}
			dt := now.Sub(last).Seconds()
			last = now
			if dt <= 0 || dt > 0.25 {
				dt = 0.25
			}
			dt *= scale
			g.mu.Lock()
			g.gameTime += dt
			g.frame++
			g.shakeTime -= dt
			if g.shakeTime <= 0 {
				g.shakeX, g.shakeY, g.shakePower = 0, 0, 0
			} else {
				g.shakeX = (randFloat()*2 - 1) * g.shakePower
				g.shakeY = (randFloat()*2 - 1) * g.shakePower
			}
			frame := g.frame
			gameTime := g.gameTime
			touchPressed := g.touchPressed
			g.touchPressed = false
			g.mu.Unlock()

			err := a.i.Protect(func() {
				a.i.Define("dt", dt)
				a.i.Define("game_time", gameTime)
				a.i.Define("game_frame", float64(frame))
				a.i.Define("touch_pressed", touchPressed)
				a.i.CallArgs(g.update, []lang.Value{dt})
				if g.draw != nil {
					a.i.CallArgs(g.draw, []lang.Value{0.0})
				}
			})
			if err != nil {
				if a.bridge != nil {
					a.bridge.Toast("游戏循环错误：" + a.i.FormatError(err))
				}
				g.stopNow()
				return
			}
			if c := a.firstCanvas(); c != nil {
				c.refreshCanvas()
			}
		}
	}
}

func (g *gameState) setPointer(x, y float64, action int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.touchX, g.touchY = x, y
	switch action {
	case 0:
		g.touchDown = true
		g.touchPressed = true
	case 1, 3:
		g.touchDown = false
	}
}

func (a *App) stopGame() {
	a.mu.Lock()
	g := a.game
	a.mu.Unlock()
	if g != nil {
		g.stopNow()
	}
}

func randFloat() float64 {
	return float64(time.Now().UnixNano()%1000000) / 1000000
}

func (a *App) registerGame() {
	reg := func(name string, fn func(*lang.Interp, []lang.Value, map[string]lang.Value) (lang.Value, error)) {
		a.i.RegisterNative(name, fn)
	}
	reg("game_loop", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, fmt.Errorf("game_loop 需要 update[, draw]")
		}
		var draw lang.Value
		if len(args) == 2 {
			draw = args[1]
		}
		return nil, a.startGame(args[0], draw, kwNumber(kw, "fps", 60))
	})
	reg("game_stop", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		a.stopGame()
		return nil, nil
	})
	reg("game_pause", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		a.mu.Lock()
		g := a.game
		a.mu.Unlock()
		if g != nil {
			g.mu.Lock()
			g.paused = true
			g.mu.Unlock()
		}
		return nil, nil
	})
	reg("game_resume", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		a.mu.Lock()
		g := a.game
		a.mu.Unlock()
		if g != nil {
			g.mu.Lock()
			g.paused = false
			g.mu.Unlock()
		}
		return nil, nil
	})
	reg("game_set_time_scale", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("game_set_time_scale 需要数值")
		}
		a.mu.Lock()
		g := a.game
		a.mu.Unlock()
		if g != nil {
			g.mu.Lock()
			g.timeScale = number(args[0])
			g.mu.Unlock()
		}
		return nil, nil
	})
	reg("camera", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		a.mu.Lock()
		g := a.game
		a.mu.Unlock()
		if g == nil {
			return nil, fmt.Errorf("camera 只能在 game_loop 中使用")
		}
		if len(args) >= 3 {
			g.mu.Lock()
			g.cameraX, g.cameraY, g.cameraZoom = number(args[0]), number(args[1]), number(args[2])
			g.cameraSet = true
			g.mu.Unlock()
		}
		m := lang.NewMap()
		g.mu.Lock()
		m.Set("x", g.cameraX)
		m.Set("y", g.cameraY)
		m.Set("zoom", g.cameraZoom)
		g.mu.Unlock()
		return m, nil
	})
	reg("camera_bounds", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		return nil, nil
	})
	reg("camera_shake", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("camera_shake 需要 power,duration")
		}
		a.mu.Lock()
		g := a.game
		a.mu.Unlock()
		if g != nil {
			g.mu.Lock()
			g.shakePower, g.shakeTime = number(args[0]), number(args[1])
			g.mu.Unlock()
		}
		return nil, nil
	})
	reg("screen", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("screen 需要 x,y")
		}
		a.mu.Lock()
		g := a.game
		a.mu.Unlock()
		if g == nil {
			return lang.NewList([]lang.Value{args[0], args[1]}), nil
		}
		g.mu.Lock()
		x := (number(args[0])-g.cameraX)*g.cameraZoom + g.shakeX
		y := (number(args[1])-g.cameraY)*g.cameraZoom + g.shakeY
		g.mu.Unlock()
		return lang.NewList([]lang.Value{x, y}), nil
	})
	reg("aabb", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if len(args) < 8 {
			return nil, fmt.Errorf("aabb 需要 8 个数值")
		}
		return number(args[0]) < number(args[4])+number(args[6]) &&
			number(args[0])+number(args[2]) > number(args[4]) &&
			number(args[1]) < number(args[5])+number(args[7]) &&
			number(args[1])+number(args[3]) > number(args[5]), nil
	})
	reg("save_json", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("save_json 需要 name,value")
		}
		data, err := lang.EncodeJSON(args[1])
		if err != nil {
			return nil, err
		}
		if a.bridge != nil {
			a.bridge.SaveData(lang.ToString(args[0]), string(data))
		}
		return nil, nil
	})
	reg("load_json", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("load_json 需要 name")
		}
		def := lang.Value(nil)
		if len(args) > 1 {
			def = args[1]
		}
		if a.bridge == nil {
			return def, nil
		}
		raw := a.bridge.LoadData(lang.ToString(args[0]))
		if raw == "" {
			return def, nil
		}
		return lang.DecodeJSON([]byte(raw))
	})
	reg("touch_x", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		a.mu.Lock()
		g := a.game
		a.mu.Unlock()
		if g == nil {
			return 0.0, nil
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		return g.touchX, nil
	})
	reg("touch_y", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		a.mu.Lock()
		g := a.game
		a.mu.Unlock()
		if g == nil {
			return 0.0, nil
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		return g.touchY, nil
	})
	reg("touch_down", func(i *lang.Interp, args []lang.Value, kw map[string]lang.Value) (lang.Value, error) {
		a.mu.Lock()
		g := a.game
		a.mu.Unlock()
		if g == nil {
			return false, nil
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		return g.touchDown, nil
	})
	_ = math.Abs
}
