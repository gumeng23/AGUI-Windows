package gui

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"time"
	"unsafe"
)

// clipboardImagePNG reads CF_DIB/CF_DIBV5 from the Windows clipboard and
// converts the common 24/32-bit uncompressed DIB layouts to PNG bytes. PNG is
// used because RichEdit can embed it directly and bytes values stay portable.
func clipboardImagePNG() ([]byte, error) {
	opened := false
	for attempt := 0; attempt < 5; attempt++ {
		r, _, _ := pOpenClipboard.Call(0)
		if r != 0 {
			opened = true
			defer pCloseClipboard.Call()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !opened {
		return nil, fmt.Errorf("无法打开剪贴板")
	}
	if r, _, _ := pIsClipboardFormatAvail.Call(8); r == 0 { // CF_DIB
		if r2, _, _ := pIsClipboardFormatAvail.Call(17); r2 == 0 { // CF_DIBV5
			return nil, fmt.Errorf("剪贴板中没有图片")
		}
	}
	var handle uintptr
	if r, _, _ := pGetClipboardData.Call(17); r != 0 {
		handle = r
	}
	if handle == 0 {
		r, _, _ := pGetClipboardData.Call(8)
		if r == 0 {
			return nil, fmt.Errorf("剪贴板中没有 DIB 图片")
		}
		handle = r
	}
	ptr, _, _ := pGlobalLock.Call(handle)
	if ptr == 0 {
		return nil, fmt.Errorf("锁定剪贴板图片失败")
	}
	defer pGlobalUnlock.Call(handle)
	size, _, _ := pGlobalSize.Call(handle)
	if size < 40 || size > 200<<20 {
		return nil, fmt.Errorf("剪贴板图片大小异常: %d", size)
	}
	data := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(size))
	return dibToPNG(data)
}

func dibToPNG(data []byte) ([]byte, error) {
	if len(data) < 40 {
		return nil, fmt.Errorf("DIB 头不完整")
	}
	headerSize := int(binary.LittleEndian.Uint32(data[0:4]))
	if headerSize < 40 || headerSize > len(data) {
		return nil, fmt.Errorf("不支持的 DIB 头大小: %d", headerSize)
	}
	width := int(int32(binary.LittleEndian.Uint32(data[4:8])))
	rawH := int(int32(binary.LittleEndian.Uint32(data[8:12])))
	planes := int(binary.LittleEndian.Uint16(data[12:14]))
	bpp := int(binary.LittleEndian.Uint16(data[14:16]))
	compression := binary.LittleEndian.Uint32(data[16:20])
	if width <= 0 || rawH == 0 || width > 20000 {
		return nil, fmt.Errorf("DIB 尺寸无效: %dx%d", width, rawH)
	}
	if planes != 1 || (bpp != 24 && bpp != 32) {
		return nil, fmt.Errorf("只支持 24/32 位 DIB，当前 %d 位", bpp)
	}
	if compression != 0 && !(compression == 3 && bpp == 32) { // BI_RGB / BI_BITFIELDS(32bpp)
		return nil, fmt.Errorf("只支持未压缩或 32 位 BITFIELDS DIB（compression=%d）", compression)
	}
	height := rawH
	topDown := false
	if height < 0 {
		height = -height
		topDown = true
	}
	if height > 20000 {
		return nil, fmt.Errorf("DIB 高度过大")
	}
	// Palette entries follow the header; 24/32bpp DIBs should have none.
	offset := headerSize
	if compression == 3 && headerSize == 40 {
		offset += 12
	}
	if offset+(width*height*bpp+7)/8 > len(data) {
		return nil, fmt.Errorf("DIB 像素数据不完整")
	}
	stride := ((width*bpp + 31) / 32) * 4
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		srcY := y
		if !topDown {
			srcY = height - 1 - y
		}
		row := data[offset+srcY*stride:]
		for x := 0; x < width; x++ {
			var b, g, r, a byte
			if bpp == 32 {
				p := row[x*4:]
				b, g, r, a = p[0], p[1], p[2], p[3]
				if a == 0 {
					a = 255
				}
			} else {
				p := row[x*3:]
				b, g, r = p[0], p[1], p[2]
				a = 255
			}
			if bpp == 32 && a != 255 && a != 0 {
				// Premultiplied clipboard pixels are uncommon; keep as-is.
				_ = b
			}
			img.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: a})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
