package gui

import (
	"fmt"

	"agui/internal/lang"
)

// Anim is a tiny sprite-sheet frame clock with a frame event.
// It is not a control; it is a language object returned by anim().
type Anim struct {
	app      *App
	frames   int
	fps      float64
	loop     bool
	time     float64
	frame    int
	playing  bool
	done     bool
	handlers map[string][]func() error
}

func (a *App) newAnim(frames int, fps float64, loop bool) *Anim {
	if frames < 1 {
		frames = 1
	}
	if fps <= 0 {
		fps = 12
	}
	return &Anim{app: a, frames: frames, fps: fps, loop: loop, playing: true, handlers: map[string][]func() error{}}
}

func (an *Anim) Kind() string     { return "anim" }
func (an *Anim) TypeName() string { return "anim" }

func (an *Anim) Listen(event string, fn func() error) error {
	switch event {
	case "frame", "finish":
		an.handlers[event] = append(an.handlers[event], fn)
		return nil
	}
	return fmt.Errorf("动画没有事件 %q", event)
}

func (an *Anim) emit(event string) {
	for _, fn := range an.handlers[event] {
		_ = fn()
	}
}

func (an *Anim) GetProp(name string) (lang.Value, bool) {
	switch name {
	case "frames":
		return float64(an.frames), true
	case "fps":
		return an.fps, true
	case "frame":
		return float64(an.frame), true
	case "playing":
		return an.playing, true
	case "done":
		return an.done, true
	case "loop":
		return an.loop, true
	}
	return nil, false
}

func (an *Anim) SetProp(name string, v lang.Value) error {
	switch name {
	case "fps":
		if n, ok := v.(float64); ok && n > 0 {
			an.fps = n
			return nil
		}
		return fmt.Errorf("fps 必须是正数")
	case "frame":
		if n, ok := v.(float64); ok {
			an.frame = int(n)
			if an.frame < 0 {
				an.frame = 0
			}
			if an.frame >= an.frames {
				an.frame = an.frames - 1
			}
			return nil
		}
		return fmt.Errorf("frame 必须是数字")
	case "loop":
		an.loop = lang.Truthy(v)
		return nil
	case "playing":
		an.playing = lang.Truthy(v)
		return nil
	}
	return fmt.Errorf("动画没有可写属性 %q", name)
}

func (an *Anim) update(dt float64) {
	if !an.playing || an.done || dt <= 0 {
		return
	}
	an.time += dt * an.fps
	for an.time >= 1 {
		an.time -= 1
		an.frame++
		if an.frame >= an.frames {
			if an.loop {
				an.frame = 0
			} else {
				an.frame = an.frames - 1
				an.playing = false
				an.done = true
				an.emit("frame")
				an.emit("finish")
				return
			}
		}
		an.emit("frame")
	}
}

func (an *Anim) CallMethod(name string, args []lang.Value, kwargs map[string]lang.Value) (lang.Value, bool, error) {
	switch name {
	case "update":
		dt := 1.0 / an.fps
		if len(args) > 0 {
			dt, _ = lang.Pos(args, 0, dt).(float64)
		}
		an.update(dt)
		return float64(an.frame), true, nil
	case "reset":
		an.frame = 0
		an.time = 0
		an.done = false
		an.playing = true
		return nil, true, nil
	case "play":
		an.playing = true
		an.done = false
		return nil, true, nil
	case "pause":
		an.playing = false
		return nil, true, nil
	case "stop":
		an.playing = false
		an.frame = 0
		an.time = 0
		return nil, true, nil
	}
	return nil, false, nil
}

func (a *App) nativeAnim(args []lang.Value, kwargs map[string]lang.Value) (lang.Value, error) {
	frames := 8
	if len(args) > 0 {
		frames = int(gameNumber(args[0]))
	}
	fps := 12.0
	if v, ok := kwargs["fps"]; ok {
		fps = gameNumber(v)
	}
	loop := true
	if v, ok := kwargs["loop"]; ok {
		loop = lang.Truthy(v)
	}
	return a.newAnim(frames, fps, loop), nil
}
