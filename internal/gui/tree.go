package gui

import (
	"fmt"
	"syscall"
	"unsafe"

	"agui/internal/lang"
)

const (
	tvsHasButtons    = 0x0001
	tvsHasLines      = 0x0002
	tvsLinesAtRoot   = 0x0004
	tvsShowSelAlways = 0x0010
	tvsCheckBoxes    = 0x0100

	tvmEnsureVisible = 0x1013
	tvmInsertItemW   = 0x1132
	tvmDeleteItem    = 0x1101
	tvmExpand        = 0x1102
	tvmGetCount      = 0x1105
	tvmGetNextItem   = 0x110A
	tvmSelectItem    = 0x110B
	tvmGetItemW      = 0x110C
	tvmSetItemW      = 0x110D

	tvifText     = 0x0001
	tvifHandle   = 0x0010
	tvifChildren = 0x0040
	tvgnRoot     = 0
	tvgnNext     = 1
	tvgnChild    = 4
	tvgnCaret    = 9
	tveExpand    = 2
	tveCollapse  = 1
	tviRoot      = 0xFFFF0000
	tviLast      = 0xFFFF0002

	tvnFirst         = 0xFFFFFE70 // -400
	tvnSelChangedW   = 0xFFFFFE6E // -402
	tvnItemExpandedW = 0xFFFFFE5D // -419
)

type tvItemW struct {
	mask           uint32
	hItem          syscall.Handle
	state          uint32
	stateMask      uint32
	pszText        *uint16
	cchTextMax     int32
	iImage         int32
	iSelectedImage int32
	iChildren      int32
	lParam         uintptr
}

type tvInsertStructW struct {
	hParent      syscall.Handle
	hInsertAfter syscall.Handle
	item         tvItemW
}

func (a *App) createTree(items lang.Value, x, y, w, h *int32) (*Widget, error) {
	style := uint32(tvsHasButtons | tvsHasLines | tvsLinesAtRoot | tvsShowSelAlways | wsBorder | wsTabStop)
	t, err := a.addChild("tree", "SysTreeView32", "", style, true, 0, a.px(220), x, y, w, h)
	if err != nil {
		return nil, err
	}
	if err := t.setTreeItems(items); err != nil {
		return nil, err
	}
	return t, nil
}

func (w *Widget) setTreeItems(items lang.Value) error {
	if w.kind != "tree" {
		return fmt.Errorf("items 只能用于 tree")
	}
	sendMessage(w.hwnd, tvmDeleteItem, 0, 0)
	w.treeLabels = map[syscall.Handle]string{}
	w.treePaths = map[string]syscall.Handle{}
	list, ok := items.(*lang.List)
	if !ok {
		return fmt.Errorf("tree 的项目必须是列表")
	}
	for _, it := range list.Items {
		if _, err := w.insertTreeNode(it, tviRoot); err != nil {
			return err
		}
	}
	return nil
}

func treeText(item lang.Value) (string, []lang.Value) {
	switch v := item.(type) {
	case string:
		return v, nil
	case *lang.List:
		if len(v.Items) == 0 {
			return "", nil
		}
		text := lang.ToString(v.Items[0])
		var kids []lang.Value
		if len(v.Items) > 1 {
			if k, ok := v.Items[1].(*lang.List); ok {
				kids = k.Items
			}
		}
		return text, kids
	case *lang.Map:
		textVal, _ := v.Get("text")
		text := lang.ToString(textVal)
		if text == "" {
			label, _ := v.Get("label")
			text = lang.ToString(label)
		}
		var kids []lang.Value
		for _, key := range []string{"children", "items"} {
			val, ok := v.Get(key)
			if !ok {
				continue
			}
			if k, ok := val.(*lang.List); ok {
				kids = k.Items
				break
			}
		}
		return text, kids
	}
	return lang.ToString(item), nil
}

func (w *Widget) insertTreeNode(item lang.Value, parent syscall.Handle) (syscall.Handle, error) {
	text, kids := treeText(item)
	ptr := u16(text)
	flags := uint32(tvifText)
	if len(kids) > 0 {
		flags |= tvifChildren
	}
	tv := tvInsertStructW{
		hParent:      parent,
		hInsertAfter: tviLast,
		item: tvItemW{
			mask:       flags,
			pszText:    ptr,
			cchTextMax: int32(len(text) + 1),
			iChildren:  boolInt(len(kids) > 0),
		},
	}
	if parent == tviRoot {
		tv.hParent = 0
	}
	h, _, callErr := pSendMessageW.Call(uintptr(w.hwnd), tvmInsertItemW, 0, uintptr(unsafe.Pointer(&tv)))
	if h == 0 {
		return 0, fmt.Errorf("插入树节点 %q 失败（%v）", text, callErr)
	}
	if w.treeLabels == nil {
		w.treeLabels = map[syscall.Handle]string{}
		w.treePaths = map[string]syscall.Handle{}
	}
	w.treeLabels[syscall.Handle(h)] = text
	w.treePaths[text] = syscall.Handle(h)
	for _, kid := range kids {
		if _, err := w.insertTreeNode(kid, syscall.Handle(h)); err != nil {
			return 0, err
		}
	}
	return syscall.Handle(h), nil
}

func boolInt(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

func (w *Widget) treeSelected() string {
	h, _, _ := pSendMessageW.Call(uintptr(w.hwnd), tvmGetNextItem, tvgnCaret, 0)
	if h == 0 {
		return ""
	}
	return w.treeLabels[syscall.Handle(h)]
}

func (w *Widget) treeCount() int {
	r, _, _ := pSendMessageW.Call(uintptr(w.hwnd), tvmGetCount, 0, 0)
	return int(r)
}

func (w *Widget) treeFind(text string) syscall.Handle {
	return w.treePaths[text]
}

func (w *Widget) selectTreeHandle(h syscall.Handle) {
	pSendMessageW.Call(uintptr(w.hwnd), tvmEnsureVisible, 0, uintptr(h))
	pSendMessageW.Call(uintptr(w.hwnd), tvmSelectItem, tvgnCaret, uintptr(h))
	setFocus(w.hwnd)
}

func (w *Widget) treeRoot() syscall.Handle {
	h, _, _ := pSendMessageW.Call(uintptr(w.hwnd), tvmGetNextItem, tvgnRoot, 0)
	return syscall.Handle(h)
}

func (w *Widget) treeChild(h syscall.Handle) syscall.Handle {
	r, _, _ := pSendMessageW.Call(uintptr(w.hwnd), tvmGetNextItem, tvgnChild, uintptr(h))
	return syscall.Handle(r)
}

func (w *Widget) treeNext(h syscall.Handle) syscall.Handle {
	r, _, _ := pSendMessageW.Call(uintptr(w.hwnd), tvmGetNextItem, tvgnNext, uintptr(h))
	return syscall.Handle(r)
}

func (w *Widget) treeAdd(text string, parent syscall.Handle) error {
	_, err := w.insertTreeNode(text, parent)
	return err
}

func (w *Widget) treeExpandAll(expand bool) {
	var walk func(h syscall.Handle)
	walk = func(h syscall.Handle) {
		for h != 0 {
			flag := uintptr(tveCollapse)
			if expand {
				flag = tveExpand
			}
			pSendMessageW.Call(uintptr(w.hwnd), tvmExpand, flag, uintptr(h))
			if child := w.treeChild(h); child != 0 {
				walk(child)
			}
			h = w.treeNext(h)
		}
	}
	walk(w.treeRoot())
}

func (w *Widget) treeNotify(code uint32, lparam uintptr) bool {
	if w.kind != "tree" {
		return false
	}
	switch code {
	case tvnSelChangedW:
		w.emit("select")
		return true
	case tvnItemExpandedW:
		w.emit("expand")
		return true
	case nmDblClk:
		w.emit("dblclick")
		return true
	}
	return false
}
