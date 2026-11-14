package gui

import "testing"

func TestParseColorHex(t *testing.T) {
	cases := map[string]uint32{
		"#FFFFFF": 0xFFFFFF,
		"#000000": 0x000000,
		"#FF0000": 0x0000FF, // COLORREF 是 BGR
		"#00FF00": 0x00FF00,
		"#0000FF": 0xFF0000,
		"#f00":    0x0000FF,
	}
	for in, want := range cases {
		got, ok := parseColor(in)
		if !ok || got != want {
			t.Fatalf("parseColor(%q) = 0x%X, %v；想要 0x%X", in, got, ok, want)
		}
	}
}

func TestParseColorNames(t *testing.T) {
	if c, ok := parseColor("white"); !ok || c != 0xFFFFFF {
		t.Fatalf("white = 0x%X, %v", c, ok)
	}
	if _, ok := parseColor("transparent"); ok {
		t.Fatal("transparent 不应是有效颜色")
	}
	if _, ok := parseColor("#GG0000"); ok {
		t.Fatal("非法十六进制颜色不应通过")
	}
}

func TestColorString(t *testing.T) {
	cases := map[uint32]string{
		0xFFFFFF: "#FFFFFF",
		0x0000FF: "#FF0000", // COLORREF BGR -> RGB
		0x2563EB: "#EB6325",
	}
	for in, want := range cases {
		if got := colorString(in); got != want {
			t.Fatalf("colorString(0x%X) = %q；想要 %q", in, got, want)
		}
	}
}

func TestWidgetKwIncludesGeometry(t *testing.T) {
	allowed := widgetKw("text")
	for _, must := range []string{"x", "y", "w", "h", "font", "text"} {
		found := false
		for _, a := range allowed {
			if a == must {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("允许列表缺少 %q：%v", must, allowed)
		}
	}
}
