package gui

import (
	"fmt"
	"sort"
	"strconv"
	"syscall"
	"unsafe"

	"agui/internal/lang"
)

const (
	lvsReport        = 0x0001
	lvsSingleSel     = 0x0004
	lvsShowSelAlways = 0x0008
	lvsExGridLines   = 0x00000001
	lvsExFullRowSel  = 0x00000020
	lvsExDoubleBuf   = 0x00010000

	lvmSetExtendedStyle = 0x1036
	lvmInsertColumnW    = 0x1061
	lvmDeleteColumn     = 0x101C
	lvmInsertItemW      = 0x104D
	lvmSetItemTextW     = 0x1074
	lvmGetItemTextW     = 0x1073
	lvmDeleteAllItems   = 0x1009
	lvmDeleteItem       = 0x1008
	lvmGetItemCount     = 0x1004
	lvmGetNextItem      = 0x100C
	lvmSetItemState     = 0x102B
	lvmSetTextColor     = 0x1024
	lvmSetBkColor       = 0x1001
	lvmSetTextBkColor   = 0x1026
	lvmEnsureVisible    = 0x1013
	lvmGetSubitemRect   = 0x1038
	lvirBounds          = 0

	lvcfFmt      = 0x0001
	lvcfWidth    = 0x0002
	lvcfText     = 0x0004
	lvcfSubItem  = 0x0008
	lvifText     = 0x0001
	lvifState    = 0x0008
	lvniSelected = 0x0002
	lvisSelected = 0x0002
	lvisFocused  = 0x0001
	lvscFocused  = 0x0001
	lvscSelected = 0x0002

	lvnItemChanged = 0xFFFFFF9B // -101
	lvnColumnClick = 0xFFFFFF94 // -108
	nmClick        = 0xFFFFFFFE // -2
	nmDblClk       = 0xFFFFFFFD // -3
)

var tableEditCallback = syscall.NewCallback(tableEditProc)

type lvColumnW struct {
	mask      uint32
	fmt       int32
	cx        int32
	pszText   *uint16
	cchText   int32
	iSubItem  int32
	iImage    int32
	iOrder    int32
	cxMin     int32
	cxDefault int32
	cxIdeal   int32
}

type lvItemW struct {
	mask       uint32
	iItem      int32
	iSubItem   int32
	state      uint32
	stateMask  uint32
	pszText    *uint16
	cchTextMax int32
	iImage     int32
	lParam     uintptr
	iIndent    int32
	iGroupID   int32
	cColumns   uint32
	puColumns  *uint32
	piColFmt   *int32
	iGroup     int32
}

type nmhdr struct {
	hwndFrom syscall.Handle
	idFrom   uintptr
	code     uint32
}

type nmItemActivate struct {
	hdr       nmhdr
	iItem     int32
	iSubItem  int32
	uNewState uint32
	uOldState uint32
	uChanged  uint32
	ptAction  point
	lParam    uintptr
}

func (a *App) createTable(headers []string, rows [][]string, x, y, w, h *int32) (*Widget, error) {
	style := uint32(lvsReport | lvsSingleSel | lvsShowSelAlways | wsBorder | wsTabStop)
	t, err := a.addChild("table", "SysListView32", "", style, true, 0, a.px(220), x, y, w, h)
	if err != nil {
		return nil, err
	}
	sendMessage(t.hwnd, lvmSetExtendedStyle, uintptr(lvsExGridLines|lvsExFullRowSel|lvsExDoubleBuf), uintptr(lvsExGridLines|lvsExFullRowSel|lvsExDoubleBuf))
	t.editable = true
	t.setHeaders(headers)
	for _, row := range rows {
		t.addRow(row)
	}
	return t, nil
}

func (w *Widget) setHeaders(headers []string) {
	if w.kind != "table" {
		return
	}
	sendMessage(w.hwnd, lvmDeleteAllItems, 0, 0)
	for i := w.cols - 1; i >= 0; i-- {
		sendMessage(w.hwnd, lvmDeleteColumn, uintptr(i), 0)
	}
	w.cols = len(headers)
	w.headers = append([]string(nil), headers...)
	width := int32(120)
	if w.w > 0 && len(headers) > 0 {
		width = w.w / int32(len(headers))
		if width < 70 {
			width = 70
		}
	}
	for i, h := range headers {
		text := u16(h)
		col := lvColumnW{
			mask:     lvcfText | lvcfWidth | lvcfSubItem,
			cx:       width,
			pszText:  text,
			iSubItem: int32(i),
		}
		sendMessage(w.hwnd, lvmInsertColumnW, uintptr(i), uintptr(unsafe.Pointer(&col)))
	}
}

func (w *Widget) addRow(cells []string) {
	if w.kind != "table" || len(cells) == 0 {
		return
	}
	text := u16(cells[0])
	item := lvItemW{mask: lvifText, iItem: int32(sendMessage(w.hwnd, lvmGetItemCount, 0, 0)), pszText: text}
	sendMessage(w.hwnd, lvmInsertItemW, 0, uintptr(unsafe.Pointer(&item)))
	for i := 1; i < len(cells); i++ {
		w.setTableCell(item.iItem, int32(i), cells[i])
	}
}

func (w *Widget) setTableCell(row, col int32, text string) {
	if w.kind != "table" || row < 0 || col < 0 {
		return
	}
	item := lvItemW{mask: lvifText, iItem: row, iSubItem: col, pszText: u16(text)}
	sendMessage(w.hwnd, lvmSetItemTextW, uintptr(row), uintptr(unsafe.Pointer(&item)))
}

func (w *Widget) tableCell(row, col int) string {
	if w.kind != "table" {
		return ""
	}
	buf := make([]uint16, 1024)
	item := lvItemW{
		mask:       lvifText,
		iItem:      int32(row),
		iSubItem:   int32(col),
		pszText:    &buf[0],
		cchTextMax: int32(len(buf)),
	}
	sendMessage(w.hwnd, lvmGetItemTextW, uintptr(row), uintptr(unsafe.Pointer(&item)))
	return syscall.UTF16ToString(buf)
}

func (w *Widget) tableSelected() int {
	r := sendMessage(w.hwnd, lvmGetNextItem, ^uintptr(0), lvniSelected)
	if int32(r) < 0 {
		return -1
	}
	return int(int32(r))
}

func (w *Widget) tableSelect(idx int) {
	if idx < 0 {
		item := lvItemW{mask: lvifState, iItem: -1, state: 0, stateMask: lvisSelected | lvisFocused}
		sendMessage(w.hwnd, lvmSetItemState, ^uintptr(0), uintptr(unsafe.Pointer(&item)))
		return
	}
	item := lvItemW{mask: lvifState, iItem: int32(idx), state: lvisSelected | lvisFocused, stateMask: lvisSelected | lvisFocused}
	sendMessage(w.hwnd, lvmSetItemState, uintptr(idx), uintptr(unsafe.Pointer(&item)))
	sendMessage(w.hwnd, lvmEnsureVisible, uintptr(idx), 1)
	setFocus(w.hwnd)
}

func (w *Widget) tableRows() *lang.List {
	n := int(sendMessage(w.hwnd, lvmGetItemCount, 0, 0))
	out := &lang.List{Items: make([]lang.Value, 0, n)}
	for r := 0; r < n; r++ {
		row := &lang.List{}
		for c := 0; c < w.cols; c++ {
			row.Items = append(row.Items, w.tableCell(r, c))
		}
		out.Items = append(out.Items, row)
	}
	return out
}

func (w *Widget) tableHeaders() *lang.List {
	out := &lang.List{}
	for _, h := range w.headers {
		out.Items = append(out.Items, h)
	}
	return out
}

func tableRowString(v lang.Value, col int) string {
	row, ok := v.(*lang.List)
	if !ok || col < 0 || col >= len(row.Items) {
		return ""
	}
	return lang.ToString(row.Items[col])
}

func tableLess(a, b string) bool {
	na, ea := strconv.ParseFloat(a, 64)
	nb, eb := strconv.ParseFloat(b, 64)
	if ea == nil && eb == nil && na != nb {
		return na < nb
	}
	return a < b
}

// tableSort sorts rows by a column, keeping the table as the source of truth.
func (w *Widget) tableSort(col int, asc bool) {
	if w.kind != "table" || col < 0 || col >= w.cols {
		return
	}
	w.commitCellEdit(true)
	rows := w.tableRows()
	sort.SliceStable(rows.Items, func(i, j int) bool {
		a := tableRowString(rows.Items[i], col)
		b := tableRowString(rows.Items[j], col)
		if asc {
			return tableLess(a, b)
		}
		return tableLess(b, a)
	})
	sendMessage(w.hwnd, lvmDeleteAllItems, 0, 0)
	for _, rv := range rows.Items {
		row, ok := rv.(*lang.List)
		if !ok {
			continue
		}
		cells := make([]string, 0, len(row.Items))
		for _, c := range row.Items {
			cells = append(cells, lang.ToString(c))
		}
		w.addRow(cells)
	}
	w.sortCol = col
	w.sortAsc = asc
}

func (w *Widget) tableSelectedCell() (int32, int32) {
	return -1, -1
}

// beginCellEdit overlays an EDIT control on a table cell.
func (w *Widget) beginCellEdit(row, col int32) {
	if w.kind != "table" || !w.editable || row < 0 || col < 0 || col >= int32(w.cols) {
		return
	}
	w.commitCellEdit(true)
	var rc rect
	rc.left = col
	rc.top = lvirBounds
	sendMessage(w.hwnd, lvmGetSubitemRect, uintptr(row), uintptr(unsafe.Pointer(&rc)))
	cw := rc.right - rc.left
	ch := rc.bottom - rc.top
	if cw <= 0 {
		cw = 100
	}
	if ch <= 0 {
		ch = w.app.px(24)
	}
	parent := w.hwnd
	if w.parent != nil {
		parent = w.parent.hwnd
	}
	x := w.x + rc.left
	y := w.y + rc.top
	h := createWindow(0, "EDIT", w.tableCell(int(row), int(col)),
		uint32(wsChild|wsVisible|wsBorder|wsTabStop|esAutoHScroll), x, y, cw, ch, parent, 0)
	if h == 0 {
		return
	}
	old := setWindowLong(h, gwlWndProc, tableEditCallback)
	w.editHwnd = h
	w.editOldProc = old
	w.editRow = row
	w.editCol = col
	w.app.tableEdits[h] = w
	sendMessage(h, emSetSel, 0, ^uintptr(0))
	setFocus(h)
}

// finishCellEdit commits or cancels the floating editor.
func (w *Widget) finishCellEdit(save bool) {
	if w.editHwnd == 0 {
		return
	}
	h := w.editHwnd
	row, col := w.editRow, w.editCol
	txt := getWindowText(h)
	w.editHwnd = 0
	if w.app != nil {
		delete(w.app.tableEdits, h)
	}
	if save && w.kind == "table" {
		w.setTableCell(row, col, txt)
	}
	pDestroyWindow.Call(uintptr(h))
	if w.hwnd != 0 {
		setFocus(w.hwnd)
	}
}

// commitCellEdit is the no-op-safe wrapper used by sorting/selection.
func (w *Widget) commitCellEdit(save bool) { w.finishCellEdit(save) }

func tableEditProc(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) uintptr {
	a := currentApp
	if a == nil {
		return 0
	}
	w := a.tableEdits[hwnd]
	if w == nil {
		return 0
	}
	switch msg {
	case wmKeyDown:
		switch wparam {
		case 0x0D: // Enter
			w.finishCellEdit(true)
			return 0
		case 0x1B: // Esc
			w.finishCellEdit(false)
			return 0
		}
	case wmKillFocus:
		w.finishCellEdit(true)
		return 0
	}
	if w.editOldProc != 0 {
		r, _, _ := pCallWindowProcW.Call(w.editOldProc, uintptr(hwnd), uintptr(msg), wparam, lparam)
		return r
	}
	return 0
}

func (w *Widget) editCell(row, col int32) error {
	if w.kind != "table" {
		return fmt.Errorf("edit_cell 只能用于表格")
	}
	if row < 0 || int(row) >= int(sendMessage(w.hwnd, lvmGetItemCount, 0, 0)) {
		return fmt.Errorf("行号 %d 超出表格范围", row)
	}
	if col < 0 || col >= int32(w.cols) {
		return fmt.Errorf("列号 %d 超出表格范围", col)
	}
	w.beginCellEdit(row, col)
	return nil
}

func (w *Widget) setTableTextColor(color uint32) {
	sendMessage(w.hwnd, lvmSetTextColor, uintptr(color), 0)
}

func (w *Widget) setTableBgColor(color uint32) {
	sendMessage(w.hwnd, lvmSetBkColor, uintptr(color), 0)
	sendMessage(w.hwnd, lvmSetTextBkColor, uintptr(color), 0)
}

func (w *Widget) tableNotify(code uint32, lparam uintptr) bool {
	if w.kind != "table" {
		return false
	}
	switch code {
	case lvnItemChanged:
		nia := (*nmItemActivate)(unsafe.Pointer(lparam))
		if nia.uNewState&lvniSelected != 0 && nia.uOldState&lvniSelected == 0 {
			w.emit("select")
			return true
		}
	case nmClick:
		w.commitCellEdit(true)
		w.emit("click")
		return true
	case nmDblClk:
		nia := (*nmItemActivate)(unsafe.Pointer(lparam))
		w.emit("dblclick")
		if w.editable && nia.iItem >= 0 && nia.iSubItem >= 0 {
			w.beginCellEdit(nia.iItem, nia.iSubItem)
		}
		return true
	case lvnColumnClick:
		nia := (*nmItemActivate)(unsafe.Pointer(lparam))
		col := int(nia.iSubItem)
		asc := true
		if w.sortCol == col {
			asc = !w.sortAsc
		}
		w.tableSort(col, asc)
		return true
	}
	return false
}
