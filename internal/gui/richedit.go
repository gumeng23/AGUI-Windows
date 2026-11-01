package gui

import (
	"fmt"
	"syscall"
	"unsafe"

	"agui/internal/lang"
)

// RICHEDIT50W comes from msftedit.dll. Loading the DLL registers the class.
var richEditLib syscall.Handle

func initRichEdit() error {
	if richEditLib != 0 {
		return nil
	}
	h, err := syscall.LoadLibrary("msftedit.dll")
	if err != nil {
		return fmt.Errorf("无法加载 msftedit.dll：%v", err)
	}
	richEditLib = h
	return nil
}

type charFormat2W struct {
	cbSize          uint32
	dwMask          uint32
	dwEffects       uint32
	yHeight         int32
	yOffset         int32
	crTextColor     uint32
	bCharSet        byte
	bPitchAndFamily byte
	szFaceName      [32]uint16
	wWeight         uint16
	sSpacing        int16
	crBackColor     uint32
	lcid            uint32
	dwReserved      uint32
	sStyle          int16
	wKerning        uint16
	bUnderlineType  byte
	bAnimation      byte
	bRevAuthor      byte
	bReserved1      byte
}

type paraFormat2 struct {
	cbSize           uint32
	dwMask           uint32
	wNumbering       uint16
	wEffects         uint16
	dxStartIndent    int32
	dxRightIndent    int32
	dxOffset         int32
	wAlignment       uint16
	cTabCount        int16
	rgxTabs          [32]int32
	dySpaceBefore    int32
	dySpaceAfter     int32
	dyLineSpacing    int32
	sStyle           int16
	bLineSpacingRule byte
	bOutlineLevel    byte
	wShadingWeight   uint16
	wShadingStyle    uint16
	wNumberingStart  uint16
	wNumberingStyle  uint16
	wNumberingTab    uint16
	wBorderSpace     uint16
	wBorderWidth     uint16
	wBorders         uint16
}

func (a *App) createRichEdit(text string, readonly bool, x, y, w, h *int32) (*Widget, error) {
	style := uint32(wsBorder | esMultiLine | esAutoVScroll | wsVScroll | wsTabStop)
	if readonly {
		style |= esReadOnly
	}
	widget, err := a.addChild("richedit", "RICHEDIT50W", text, style, true, 0, a.px(180), x, y, w, h)
	if err != nil {
		return nil, err
	}
	return widget, nil
}

func (w *Widget) textLen() int32 {
	return int32(sendMessage(w.hwnd, wmGetTextLength, 0, 0))
}

func (w *Widget) richFormat(cf *charFormat2W, start, end int32) {
	sendMessage(w.hwnd, emSetSel, uintptr(start), uintptr(end))
	sendMessage(w.hwnd, emSetCharFormat, scfSelection, uintptr(unsafe.Pointer(cf)))
}

func (w *Widget) applyRichFormat(kwargs map[string]lang.Value, all bool) error {
	cf := charFormat2W{cbSize: uint32(unsafe.Sizeof(charFormat2W{}))}
	var mask, effects uint32
	hasEffects := false
	if v, ok := kwargs["bold"]; ok {
		mask |= cfmBold
		if lang.Truthy(v) {
			effects |= cfeBold
		}
		hasEffects = true
	}
	if v, ok := kwargs["italic"]; ok {
		mask |= cfmItalic
		if lang.Truthy(v) {
			effects |= cfeItalic
		}
		hasEffects = true
	}
	if v, ok := kwargs["underline"]; ok {
		mask |= cfmUnderline
		if lang.Truthy(v) {
			effects |= cfeUnderline
		}
		hasEffects = true
	}
	if hasEffects {
		cf.dwMask |= mask
		cf.dwEffects |= effects
	}
	if v, ok := kwargs["color"]; ok {
		c, ok := parseColor(lang.ToString(v))
		if !ok {
			return fmt.Errorf("无法识别颜色 %q", lang.ToString(v))
		}
		cf.dwMask |= cfmColor
		cf.crTextColor = c
	}
	if v, ok := kwargs["size"]; ok {
		f, ok := v.(float64)
		if !ok || f <= 0 {
			return fmt.Errorf("size 必须是大于 0 的数字")
		}
		cf.dwMask |= cfmSize | cfmWeight
		cf.yHeight = int32(f * 20)
		cf.wWeight = 400
	}
	if v, ok := kwargs["font"]; ok {
		face := lang.ToString(v)
		cf.dwMask |= cfmFace
		cf.bCharSet = 1 // DEFAULT_CHARSET
		copy(cf.szFaceName[:], syscall.StringToUTF16(face))
	}
	if cf.dwMask == 0 {
		return nil
	}
	if all {
		sendMessage(w.hwnd, emSetSel, 0, ^uintptr(0))
		sendMessage(w.hwnd, emSetCharFormat, scfAll, uintptr(unsafe.Pointer(&cf)))
		return nil
	}
	// Apply to the current selection; if empty, it affects subsequent typing.
	sendMessage(w.hwnd, emSetCharFormat, scfSelection, uintptr(unsafe.Pointer(&cf)))
	return nil
}

func (w *Widget) applyRichPara(kwargs map[string]lang.Value, all bool) error {
	v, ok := kwargs["align"]
	if !ok {
		return nil
	}
	align := 0
	switch lang.ToString(v) {
	case "left", "左":
		align = pfaLeft
	case "center", "居中", "中":
		align = pfaCenter
	case "right", "右":
		align = pfaRight
	default:
		return fmt.Errorf("align 只能是 left/center/right")
	}
	pf := paraFormat2{cbSize: uint32(unsafe.Sizeof(paraFormat2{})), dwMask: pfmAlignment, wAlignment: uint16(align)}
	if all {
		sendMessage(w.hwnd, emSetSel, 0, ^uintptr(0))
		sendMessage(w.hwnd, emSetParaFormat, uintptr(unsafe.Pointer(&pf)), 0)
		return nil
	}
	sendMessage(w.hwnd, emSetParaFormat, uintptr(unsafe.Pointer(&pf)), 0)
	return nil
}

func (w *Widget) appendRich(text string, kwargs map[string]lang.Value) error {
	start := w.textLen()
	sendMessage(w.hwnd, emSetSel, ^uintptr(0), ^uintptr(0))
	sendMessage(w.hwnd, emReplaceSel, 1, uintptr(unsafe.Pointer(u16(text))))
	end := w.textLen()
	cf := charFormat2W{cbSize: uint32(unsafe.Sizeof(charFormat2W{}))}
	var mask, effects uint32
	if v, ok := kwargs["bold"]; ok {
		mask |= cfmBold
		if lang.Truthy(v) {
			effects |= cfeBold
		}
	}
	if v, ok := kwargs["italic"]; ok {
		mask |= cfmItalic
		if lang.Truthy(v) {
			effects |= cfeItalic
		}
	}
	if v, ok := kwargs["underline"]; ok {
		mask |= cfmUnderline
		if lang.Truthy(v) {
			effects |= cfeUnderline
		}
	}
	if v, ok := kwargs["color"]; ok {
		c, ok := parseColor(lang.ToString(v))
		if !ok {
			return fmt.Errorf("无法识别颜色 %q", lang.ToString(v))
		}
		mask |= cfmColor
		cf.crTextColor = c
	}
	if v, ok := kwargs["size"]; ok {
		f, ok := v.(float64)
		if !ok || f <= 0 {
			return fmt.Errorf("size 必须是大于 0 的数字")
		}
		mask |= cfmSize | cfmWeight
		cf.yHeight = int32(f * 20)
		cf.wWeight = 400
	}
	if v, ok := kwargs["font"]; ok {
		mask |= cfmFace
		cf.bCharSet = 1
		copy(cf.szFaceName[:], syscall.StringToUTF16(lang.ToString(v)))
	}
	if mask != 0 {
		cf.dwMask = mask
		cf.dwEffects = effects
		w.richFormat(&cf, start, end)
	}
	sendMessage(w.hwnd, emSetSel, uintptr(end), uintptr(end))
	return nil
}
