package gui

import (
	"testing"

	"agui/internal/lang"
)

func TestVKName(t *testing.T) {
	cases := map[uint32]string{0x41: "a", 0x30: "0", 0x20: "space", 0x25: "left", 0x26: "up", 0x1B: "escape"}
	for vk, want := range cases {
		if got := vkName(vk); got != want {
			t.Errorf("vkName(%#x)=%q want %q", vk, got, want)
		}
	}
}

func TestNormalizeKey(t *testing.T) {
	if got := normalizeKey("A"); got != "a" {
		t.Fatalf("normalizeKey(A)=%q", got)
	}
	if got := normalizeKey("Space"); got != "space" {
		t.Fatalf("normalizeKey(Space)=%q", got)
	}
}

func TestGameKeyState(t *testing.T) {
	g := &gameRuntime{keys: map[string]bool{}, pressed: map[string]bool{}, released: map[string]bool{}, app: &App{}}
	g.setKey("a", true)
	if !g.keyDown("a") {
		t.Fatal("a not down")
	}
	if !g.pressed["a"] {
		t.Fatal("a not pressed")
	}
	g.endFrame()
	if g.pressed["a"] {
		t.Fatal("pressed not cleared")
	}
	g.setKey("a", false)
	if g.keyDown("a") {
		t.Fatal("a still down")
	}
	if !g.released["a"] {
		t.Fatal("a not released")
	}
}

func TestCameraScreen(t *testing.T) {
	g := &gameRuntime{camX: 100, camY: 50, camZoom: 2}
	x, y := g.screen(110, 60)
	if x != 20 || y != 20 {
		t.Fatalf("screen=%v,%v want 20,20", x, y)
	}
}

func TestGamepadButtons(t *testing.T) {
	m := gamepadButtons(0x1000 | 0x0010)
	if !m["a"] || !m["start"] {
		t.Fatalf("buttons=%v", m)
	}
}

func TestGameBindings(t *testing.T) {
	g := &gameRuntime{keys: map[string]bool{}, pressed: map[string]bool{}, released: map[string]bool{}, bindings: map[string][]string{"jump": {"space"}}, app: &App{}}
	g.keys["space"] = true
	if !g.keyDown("jump") {
		t.Fatal("binding jump should be down")
	}
	g.keys["space"] = false
	if g.keyDown("jump") {
		t.Fatal("binding jump should be up")
	}
}

func TestTileAt(t *testing.T) {
	tiles := &lang.List{Items: []lang.Value{
		&lang.List{Items: []lang.Value{1.0, 0.0}},
		&lang.List{Items: []lang.Value{0.0, 2.0}},
	}}
	if tileAt(tiles, 1, 1) != 2 || tileAt(tiles, 0, 0) != 1 || tileAt(tiles, 9, 9) != 0 {
		t.Fatal("tileAt mismatch")
	}
}

func TestAnimFrameEvent(t *testing.T) {
	a := (&App{}).newAnim(2, 10, true)
	var got []int
	if err := a.Listen("frame", func() error {
		got = append(got, a.frame)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	a.update(0.1)
	a.update(0.1)
	a.update(0.1)
	if len(got) != 3 || got[0] != 1 || got[1] != 0 || got[2] != 1 {
		t.Fatalf("frames=%v", got)
	}
	var _ lang.Widget = a
	if lang.TypeName(a) != "anim" {
		t.Fatalf("type=%q", lang.TypeName(a))
	}
}

func TestAnimFinish(t *testing.T) {
	a := (&App{}).newAnim(2, 10, false)
	a.update(0.1)
	a.update(0.1)
	if !a.done || a.playing {
		t.Fatalf("done=%v playing=%v frame=%d", a.done, a.playing, a.frame)
	}
}
