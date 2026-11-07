package gui

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image/jpeg"
	"image/png"
	"strings"
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
	sendMessage(widget.hwnd, emSetEventMask, enmLink, 0)
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

type richLink struct {
	start, end int32
	url        string
}

type charRange struct {
	cpMin, cpMax int32
}

type enLinkW struct {
	nmhdr  nmhdr
	msg    uint32
	_      uint32
	wParam uintptr
	lParam uintptr
	chrg   charRange
}

type editStreamW struct {
	dwCookie    uintptr
	dwError     uint32
	pfnCallback uintptr
}

type editStreamCookie struct {
	data []byte
	pos  int
}

func editStreamCallback(cookie uintptr, pbBuff uintptr, cb uintptr, pcb uintptr) uintptr {
	c := (*editStreamCookie)(unsafe.Pointer(cookie))
	n := 0
	if c.pos < len(c.data) {
		dst := unsafe.Slice((*byte)(unsafe.Pointer(pbBuff)), int(cb))
		n = copy(dst, c.data[c.pos:])
		c.pos += n
	}
	*(*int32)(unsafe.Pointer(pcb)) = int32(n)
	return 0
}

var editStreamCallbackPtr = syscall.NewCallback(editStreamCallback)

func (w *Widget) richSelection() (int32, int32) {
	r := sendMessage(w.hwnd, emGetSel, 0, 0)
	start := int32(r & 0xFFFF)
	end := int32((r >> 16) & 0xFFFF)
	if start < 0 || start > 0x7FFFFFFF {
		start = 0
	}
	if end < start {
		end = start
	}
	return start, end
}

func (w *Widget) richSelectRange(start, end int32) {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	sendMessage(w.hwnd, emSetSel, uintptr(start), uintptr(end))
	sendMessage(w.hwnd, emScrollCaret, 0, 0)
}

func (w *Widget) richSelectedText() string {
	start, end := w.richSelection()
	txt := getWindowText(w.hwnd)
	runes := []rune(txt)
	if int(end) > len(runes) {
		end = int32(len(runes))
	}
	if int(start) >= len(runes) || start >= end {
		return ""
	}
	return string(runes[start:end])
}

func (w *Widget) richInsertText(text string, replace bool) {
	if replace {
		sendMessage(w.hwnd, emReplaceSel, 1, uintptr(unsafe.Pointer(u16(text))))
		return
	}
	start, _ := w.richSelection()
	sendMessage(w.hwnd, emSetSel, uintptr(start), uintptr(start))
	sendMessage(w.hwnd, emReplaceSel, 1, uintptr(unsafe.Pointer(u16(text))))
}

func (w *Widget) richLink(text, url string) error {
	start, _ := w.richSelection()
	w.richInsertText(text, true)
	end := w.textLen()
	if end <= start {
		return fmt.Errorf("插入链接文字失败")
	}
	cf := charFormat2W{
		cbSize:      uint32(unsafe.Sizeof(charFormat2W{})),
		dwMask:      cfmLink | cfmColor | cfmUnderline,
		dwEffects:   cfeLink | cfeUnderline,
		crTextColor: 0x00FF0000, // COLORREF: blue
	}
	sendMessage(w.hwnd, emSetSel, uintptr(start), uintptr(end))
	sendMessage(w.hwnd, emSetCharFormat, scfSelection, uintptr(unsafe.Pointer(&cf)))
	w.richLinks = append(w.richLinks, richLink{start: start, end: end, url: url})
	sendMessage(w.hwnd, emSetSel, uintptr(end), uintptr(end))
	return nil
}

func (w *Widget) richInsertImage(v lang.Value) error {
	var data []byte
	switch t := v.(type) {
	case *lang.Bytes:
		data = t.Data
	case string:
		b, err := w.app.interp.ReadFile(t)
		if err != nil {
			return err
		}
		data = b
	default:
		return fmt.Errorf("insert_image 需要 bytes 或图片路径")
	}
	if len(data) < 24 || string(data[1:4]) != "PNG" {
		return fmt.Errorf("只支持 PNG 图片")
	}
	wpx := binary.BigEndian.Uint32(data[16:20])
	hpx := binary.BigEndian.Uint32(data[20:24])
	if wpx == 0 || hpx == 0 || wpx > 20000 || hpx > 20000 {
		return fmt.Errorf("PNG 尺寸无效")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("解码 PNG 失败：%v", err)
	}
	var jb bytes.Buffer
	if err := jpeg.Encode(&jb, img, &jpeg.Options{Quality: 90}); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, `{tf1nsi\deff0{\pict\jpegblip\picw%d\pich%d\picwgoal%d\pichgoal%d `,
		wpx, hpx, wpx*15, hpx*15)
	b.WriteString(hex.EncodeToString(jb.Bytes()))
	b.WriteString(`}}`)
	if err := clipboardSetRTF(b.String()); err != nil {
		return err
	}
	selStart, _ := w.richSelection()
	setFocus(w.hwnd)
	before := w.textLen()
	sendMessage(w.hwnd, wmPaste, 0, 0)
	after := w.textLen()
	if after <= before {
		return fmt.Errorf("插入图片失败")
	}
	sendMessage(w.hwnd, emSetSel, uintptr(after), uintptr(after))
	for i := range w.richLinks {
		if w.richLinks[i].start >= selStart {
			w.richLinks[i].start++
			w.richLinks[i].end++
		}
	}
	return nil
}

func (w *Widget) richNotify(code uint32, lparam uintptr) {
	if code != enLink {
		return
	}
	el := (*enLinkW)(unsafe.Pointer(lparam))
	if el.msg != wmLButtonUp {
		return
	}
	pos := el.chrg.cpMin
	for _, l := range w.richLinks {
		if pos >= l.start && pos < l.end {
			openURL(l.url)
			return
		}
	}
}

func clipboardRTFFormat() uint32 {
	name := u16("Rich Text Format")
	r, _, _ := pRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(name)))
	return uint32(r)
}

func clipboardSetRTF(rtf string) error {
	cf := clipboardRTFFormat()
	if cf == 0 {
		return fmt.Errorf("无法注册 RTF 剪贴板格式")
	}
	h := globalAlloc(gmemMoveable, uintptr(len(rtf)+1))
	if h == 0 {
		return fmt.Errorf("剪贴板内存分配失败")
	}
	ptr, _, _ := pGlobalLock.Call(uintptr(h))
	if ptr == 0 {
		pGlobalFree.Call(uintptr(h))
		return fmt.Errorf("剪贴板内存锁定失败")
	}
	dst := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), len(rtf)+1)
	copy(dst, rtf)
	pGlobalUnlock.Call(uintptr(h))
	if r, _, _ := pOpenClipboard.Call(0); r == 0 {
		pGlobalFree.Call(uintptr(h))
		return fmt.Errorf("无法打开剪贴板")
	}
	defer pCloseClipboard.Call()
	pEmptyClipboard.Call()
	if r, _, _ := pSetClipboardData.Call(uintptr(cf), uintptr(h)); r == 0 {
		pGlobalFree.Call(uintptr(h))
		return fmt.Errorf("写入 RTF 剪贴板失败")
	}
	return nil
}

func openURL(url string) {
	op := u16("open")
	target := u16(url)
	pShellExecuteW.Call(0, uintptr(unsafe.Pointer(op)), uintptr(unsafe.Pointer(target)), 0, 0, 1)
}
