package winres

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func utf16le(s string) []byte {
	out := make([]byte, 0, len(s)*2)
	for _, r := range s {
		if r > 0xffff {
			continue
		}
		out = append(out, byte(r), byte(r>>8))
	}
	return out
}

func TestBuildVersionContainsFields(t *testing.T) {
	b := buildVersion(Options{
		CompanyName:     "Agui Studio",
		ProductName:     "Agui 演示",
		FileDescription: "测试程序",
		FileVersion:     "1.2.3.4",
		ProductVersion:  "5.6.7.8",
		LegalCopyright:  "MIT",
	})
	for _, s := range []string{"VS_VERSION_INFO", "CompanyName", "Agui Studio", "ProductName", "Agui 演示", "FileVersion", "1.2.3.4", "ProductVersion", "5.6.7.8", "LegalCopyright"} {
		if !bytes.Contains(b, utf16le(s)) {
			t.Fatalf("版本资源缺少 %q", s)
		}
	}
	if len(b) < 200 {
		t.Fatalf("版本资源太小：%d", len(b))
	}
}

func TestParseICOGroupsImages(t *testing.T) {
	data := make([]byte, 22+8)
	binary.LittleEndian.PutUint16(data[2:], 1)
	binary.LittleEndian.PutUint16(data[4:], 1)
	data[6] = 16
	data[7] = 16
	binary.LittleEndian.PutUint16(data[10:], 1)
	binary.LittleEndian.PutUint16(data[12:], 32)
	binary.LittleEndian.PutUint32(data[14:], 8)
	binary.LittleEndian.PutUint32(data[18:], 22)
	copy(data[22:], []byte("icondata"))
	images, group, err := parseICO(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 1 || string(images[0]) != "icondata" {
		t.Fatalf("图标图像 = %q", images)
	}
	if len(group) != 6+14 || binary.LittleEndian.Uint16(group[4:]) != 1 {
		t.Fatalf("图标组长度 = %d", len(group))
	}
}

func TestResourceDataEntriesUseAbsoluteRVA(t *testing.T) {
	ico := make([]byte, 22+8)
	binary.LittleEndian.PutUint16(ico[2:], 1)
	binary.LittleEndian.PutUint16(ico[4:], 1)
	ico[6], ico[7] = 16, 16
	binary.LittleEndian.PutUint16(ico[10:], 1)
	binary.LittleEndian.PutUint16(ico[12:], 32)
	binary.LittleEndian.PutUint32(ico[14:], 8)
	binary.LittleEndian.PutUint32(ico[18:], 22)
	copy(ico[22:], []byte("icondata"))

	const rva = 0x5000
	sec, err := buildResourceSection(Options{Icon: ico, FileVersion: "1.0.0.0"}, rva)
	if err != nil {
		t.Fatal(err)
	}
	if len(sec) < 32 {
		t.Fatalf("资源节太小：%d", len(sec))
	}
	// 根目录 -> 第一个类型 -> 第一个 ID -> 语言目录 -> data entry。
	root := 0
	firstType := int(binary.LittleEndian.Uint32(sec[root+16+4:]) & 0x7fffffff)
	firstID := int(binary.LittleEndian.Uint32(sec[firstType+16+4:]) & 0x7fffffff)
	langDir := int(binary.LittleEndian.Uint32(sec[firstID+16+4:]) & 0x7fffffff)
	dataEntry := int(binary.LittleEndian.Uint32(sec[langDir+16+4:]))
	got := binary.LittleEndian.Uint32(sec[dataEntry:])
	if got < rva {
		t.Fatalf("data entry OffsetToData = %#x，应包含 RVA %#x", got, rva)
	}
	if got != rva+uint32(dataEntry) && got < rva {
		t.Fatalf("data entry OffsetToData = %#x，RVA 无效", got)
	}
}
