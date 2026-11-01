// Package winres adds Windows resources (icon and version information) to a
// PE executable without requiring windres or any external tool.
package winres

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"
)

// Options describes the resources to embed. Icon is the raw contents of a
// .ico file. Empty version strings are omitted from the version resource.
type Options struct {
	Icon            []byte
	CompanyName     string
	ProductName     string
	FileDescription string
	FileVersion     string
	ProductVersion  string
	LegalCopyright  string
}

const (
	rtIcon      = 3
	rtGroupIcon = 14
	rtVersion   = 16
	langID      = 0x0409
)

// AddResources appends a .rsrc section to data and points the PE resource data
// directory at it. The input must be a PE32/PE32+ executable.
func AddResources(data []byte, opts Options) ([]byte, error) {
	if len(data) < 0x40 {
		return nil, errors.New("不是有效的 PE 文件")
	}
	peOff := int(binary.LittleEndian.Uint32(data[0x3c:]))
	if peOff < 0 || peOff+24 > len(data) || string(data[peOff:peOff+4]) != "PE\x00\x00" {
		return nil, errors.New("不是有效的 PE 文件")
	}
	numSec := int(binary.LittleEndian.Uint16(data[peOff+6:]))
	optSize := int(binary.LittleEndian.Uint16(data[peOff+20:]))
	optOff := peOff + 24
	if optOff+optSize > len(data) {
		return nil, errors.New("PE 可选头无效")
	}
	magic := binary.LittleEndian.Uint16(data[optOff:])
	var sectionAlign, fileAlign, sizeOfImage uint32
	var dataDirOff int
	switch magic {
	case 0x10b:
		sectionAlign = binary.LittleEndian.Uint32(data[optOff+32:])
		fileAlign = binary.LittleEndian.Uint32(data[optOff+36:])
		sizeOfImage = binary.LittleEndian.Uint32(data[optOff+56:])
		dataDirOff = optOff + 96
	case 0x20b:
		sectionAlign = binary.LittleEndian.Uint32(data[optOff+32:])
		fileAlign = binary.LittleEndian.Uint32(data[optOff+36:])
		sizeOfImage = binary.LittleEndian.Uint32(data[optOff+56:])
		dataDirOff = optOff + 112
	default:
		return nil, errors.New("不支持的 PE 格式")
	}
	if sectionAlign == 0 || fileAlign == 0 {
		return nil, errors.New("PE 对齐参数无效")
	}
	if dataDirOff+16*8 > len(data) {
		return nil, errors.New("PE 数据目录无效")
	}
	if rva := binary.LittleEndian.Uint32(data[dataDirOff+2*8:]); rva != 0 {
		return nil, errors.New("该 exe 已经包含资源节")
	}

	secTab := optOff + optSize
	if secTab+numSec*40 > len(data) {
		return nil, errors.New("PE 节表无效")
	}

	type section struct {
		headerOff        int
		virtualSize      uint32
		virtualAddress   uint32
		sizeOfRawData    uint32
		pointerToRawData uint32
	}
	secs := make([]section, numSec)
	firstRaw := uint32(0xffffffff)
	var last section
	for i := 0; i < numSec; i++ {
		off := secTab + i*40
		s := section{
			headerOff:        off,
			virtualSize:      binary.LittleEndian.Uint32(data[off+8:]),
			virtualAddress:   binary.LittleEndian.Uint32(data[off+12:]),
			sizeOfRawData:    binary.LittleEndian.Uint32(data[off+16:]),
			pointerToRawData: binary.LittleEndian.Uint32(data[off+20:]),
		}
		secs[i] = s
		if s.sizeOfRawData > 0 && s.pointerToRawData > 0 && s.pointerToRawData < firstRaw {
			firstRaw = s.pointerToRawData
		}
		last = s
	}
	if numSec == 0 {
		return nil, errors.New("PE 文件没有节")
	}
	// There is usually a small zero-filled gap between the section table and
	// the first raw section. Use it for the new section header.
	headerOff := secTab + numSec*40
	if headerOff+40 > len(data) || (firstRaw != 0xffffffff && uint32(headerOff+40) > firstRaw) {
		return nil, errors.New("PE 节表后没有空间写入新节头")
	}

	// Lay out the new section first: resource data entries store absolute RVAs.
	newVA := align(last.virtualAddress+max32(last.virtualSize, last.sizeOfRawData), sectionAlign)
	res, err := buildResourceSection(opts, newVA)
	if err != nil {
		return nil, err
	}
	newRaw := align32(uint32(len(data)), fileAlign)
	newRawSize := align32(uint32(len(res)), fileAlign)

	out := make([]byte, newRaw+newRawSize)
	copy(out, data)
	copy(out[newRaw:], res)

	// Section header.
	copy(out[headerOff:], []byte(".rsrc"))
	binary.LittleEndian.PutUint32(out[headerOff+8:], uint32(len(res)))
	binary.LittleEndian.PutUint32(out[headerOff+12:], newVA)
	binary.LittleEndian.PutUint32(out[headerOff+16:], newRawSize)
	binary.LittleEndian.PutUint32(out[headerOff+20:], newRaw)
	binary.LittleEndian.PutUint32(out[headerOff+36:], 0x40000040) // initialized data | read

	binary.LittleEndian.PutUint16(out[peOff+6:], uint16(numSec+1))
	binary.LittleEndian.PutUint32(out[dataDirOff+2*8:], newVA)
	binary.LittleEndian.PutUint32(out[dataDirOff+2*8+4:], uint32(len(res)))
	newSize := align(newVA+uint32(len(res)), sectionAlign)
	if newSize < sizeOfImage {
		newSize = sizeOfImage
	}
	binary.LittleEndian.PutUint32(out[optOff+56:], newSize)
	return out, nil
}

func align(v, a uint32) uint32 {
	if a == 0 {
		return v
	}
	return (v + a - 1) / a * a
}

func align32(v, a uint32) uint32 { return align(v, a) }

func max32(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

type resEntry struct {
	id         uint32
	child      *resDir
	data       []byte
	dirOffset  uint32
	dataOffset uint32
	blobOffset uint32
}

type resDir struct {
	entries []*resEntry
	offset  uint32
}

func (d *resDir) ensure(id uint32) *resEntry {
	for _, e := range d.entries {
		if e.id == id {
			return e
		}
	}
	e := &resEntry{id: id}
	d.entries = append(d.entries, e)
	return e
}

func buildResourceSection(opts Options, resourceRVA uint32) ([]byte, error) {
	root := &resDir{}
	add := func(typ, id uint32, payload []byte) {
		t := root.ensure(typ)
		if t.child == nil {
			t.child = &resDir{}
		}
		n := t.child.ensure(id)
		n.child = &resDir{entries: []*resEntry{{id: langID, data: payload}}}
	}

	if len(opts.Icon) > 0 {
		icons, group, err := parseICO(opts.Icon)
		if err != nil {
			return nil, err
		}
		for i, img := range icons {
			add(rtIcon, uint32(i+1), img)
		}
		add(rtGroupIcon, 1, group)
	}
	ver := buildVersion(opts)
	add(rtVersion, 1, ver)

	sortDir(root)
	// Assign directory offsets depth-first.
	var cur uint32
	var layoutDir func(d *resDir)
	layoutDir = func(d *resDir) {
		d.offset = cur
		cur += 16 + uint32(8*len(d.entries))
		for _, e := range d.entries {
			if e.child != nil {
				layoutDir(e.child)
			}
		}
	}
	layoutDir(root)

	// Data entries follow all directories.
	var leaves []*resEntry
	var collect func(d *resDir)
	collect = func(d *resDir) {
		for _, e := range d.entries {
			if e.child != nil {
				collect(e.child)
			} else {
				leaves = append(leaves, e)
			}
		}
	}
	collect(root)
	for _, e := range leaves {
		e.dataOffset = cur
		cur += 16
	}
	// Payloads follow data entries, 8-byte aligned.
	cur = align(cur, 8)
	for _, e := range leaves {
		e.blobOffset = cur
		cur += uint32(len(e.data))
		cur = align(cur, 8)
	}
	buf := make([]byte, cur)

	var writeDir func(d *resDir)
	writeDir = func(d *resDir) {
		off := d.offset
		binary.LittleEndian.PutUint16(buf[off+12:], 0)
		binary.LittleEndian.PutUint16(buf[off+14:], uint16(len(d.entries)))
		for i, e := range d.entries {
			eo := off + 16 + uint32(i*8)
			binary.LittleEndian.PutUint32(buf[eo:], e.id)
			if e.child != nil {
				binary.LittleEndian.PutUint32(buf[eo+4:], 0x80000000|e.child.offset)
				writeDir(e.child)
			} else {
				binary.LittleEndian.PutUint32(buf[eo+4:], e.dataOffset)
				do := e.dataOffset
				binary.LittleEndian.PutUint32(buf[do:], resourceRVA+e.blobOffset)
				binary.LittleEndian.PutUint32(buf[do+4:], uint32(len(e.data)))
				binary.LittleEndian.PutUint32(buf[do+8:], 0)
				binary.LittleEndian.PutUint32(buf[do+12:], 0)
				copy(buf[e.blobOffset:], e.data)
			}
		}
	}
	writeDir(root)
	return buf, nil
}

func sortDir(d *resDir) {
	sort.Slice(d.entries, func(i, j int) bool { return d.entries[i].id < d.entries[j].id })
	for _, e := range d.entries {
		if e.child != nil {
			sortDir(e.child)
		}
	}
}

func parseICO(data []byte) ([][]byte, []byte, error) {
	if len(data) < 6 {
		return nil, nil, errors.New("图标文件太小")
	}
	reserved := binary.LittleEndian.Uint16(data[0:])
	typ := binary.LittleEndian.Uint16(data[2:])
	count := int(binary.LittleEndian.Uint16(data[4:]))
	if reserved != 0 || typ != 1 || count == 0 {
		return nil, nil, errors.New("不是有效的 .ico 图标文件")
	}
	if 6+count*16 > len(data) {
		return nil, nil, errors.New("图标目录无效")
	}
	images := make([][]byte, 0, count)
	group := make([]byte, 6+count*14)
	binary.LittleEndian.PutUint16(group[2:], 1)
	binary.LittleEndian.PutUint16(group[4:], uint16(count))
	for i := 0; i < count; i++ {
		off := 6 + i*16
		size := int(binary.LittleEndian.Uint32(data[off+8:]))
		imgOff := int(binary.LittleEndian.Uint32(data[off+12:]))
		if size <= 0 || imgOff < 0 || imgOff+size > len(data) {
			return nil, nil, fmt.Errorf("图标图像 %d 越界", i+1)
		}
		images = append(images, append([]byte(nil), data[imgOff:imgOff+size]...))
		ge := 6 + i*14
		copy(group[ge:ge+14], data[off:off+14])
		binary.LittleEndian.PutUint16(group[ge+12:], uint16(i+1))
	}
	return images, group, nil
}

func buildVersion(opts Options) []byte {
	fv := parseVersion(opts.FileVersion)
	pv := parseVersion(opts.ProductVersion)
	if opts.FileVersion == "" {
		fv = [4]uint32{1, 0, 0, 0}
	}
	if opts.ProductVersion == "" {
		pv = fv
	}
	fixed := make([]byte, 52)
	put := func(off int, v uint32) { binary.LittleEndian.PutUint32(fixed[off:], v) }
	put(0, 0xFEEF04BD)
	put(4, 0x00010000)
	put(8, (fv[0]<<16)|fv[1])
	put(12, (fv[2]<<16)|fv[3])
	put(16, (pv[0]<<16)|pv[1])
	put(20, (pv[2]<<16)|pv[3])
	put(24, 0x3F)
	put(28, 0)
	put(32, 0x40004)
	put(36, 1)
	put(40, 0)
	put(44, 0)
	put(48, 0)

	var strTable []byte
	addString := func(name, value string) {
		if value == "" {
			return
		}
		u := utf16.Encode([]rune(value))
		raw := make([]byte, 0, len(u)*2+2)
		for _, r := range u {
			raw = binary.LittleEndian.AppendUint16(raw, r)
		}
		raw = append(raw, 0, 0)
		strTable = append(strTable, versionBlock(name, raw, len(u)+1, 1, nil)...)
	}
	addString("CompanyName", opts.CompanyName)
	addString("FileDescription", opts.FileDescription)
	addString("FileVersion", versionString(fv))
	addString("LegalCopyright", opts.LegalCopyright)
	addString("ProductName", opts.ProductName)
	addString("ProductVersion", versionString(pv))
	table := versionBlock("040904B0", nil, 0, 1, strTable)
	sfi := versionBlock("StringFileInfo", nil, 0, 1, table)
	trans := make([]byte, 4)
	binary.LittleEndian.PutUint32(trans, langID<<16|1200)
	varf := versionBlock("Translation", trans, 4, 0, nil)
	vfi := versionBlock("VarFileInfo", nil, 0, 1, varf)
	children := append(sfi, vfi...)
	return versionBlock("VS_VERSION_INFO", fixed, 52, 0, children)
}

func versionBlock(key string, value []byte, valueLen int, typ uint16, children []byte) []byte {
	b := make([]byte, 6)
	binary.LittleEndian.PutUint16(b[4:], typ)
	for _, r := range utf16.Encode([]rune(key)) {
		b = binary.LittleEndian.AppendUint16(b, r)
	}
	b = append(b, 0, 0)
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	b = append(b, value...)
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	b = append(b, children...)
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	binary.LittleEndian.PutUint16(b[0:], uint16(len(b)))
	binary.LittleEndian.PutUint16(b[2:], uint16(valueLen))
	return b
}

func parseVersion(s string) [4]uint32 {
	var out [4]uint32
	parts := strings.Split(strings.TrimSpace(s), ".")
	for i := 0; i < len(parts) && i < 4; i++ {
		var n uint32
		fmt.Sscanf(strings.TrimSpace(parts[i]), "%d", &n)
		out[i] = n
	}
	return out
}

func versionString(v [4]uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", v[0], v[1], v[2], v[3])
}
