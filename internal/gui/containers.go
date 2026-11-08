package gui

import "fmt"

// This file implements the higher-level layout containers: toolbar, splitter,
// tab pages and property grid. They are ordinary panel windows so child
// controls keep the same message routing as panel/scroll/grid.

func (a *App) createToolbar(x, y, w, h *int32) (*Widget, error) {
	return a.createPanel("toolbar", 1, x, y, w, h)
}

func (a *App) createSplitter(vertical bool, x, y, w, h *int32) (*Widget, error) {
	widget, err := a.createPanel("splitter", 2, x, y, w, h)
	if err != nil {
		return nil, err
	}
	widget.splitVertical = vertical
	widget.splitRatio = 0.5
	// Create both panes eagerly so scripts can parent controls into them.
	for i := 0; i < 2; i++ {
		a.pendingParent = widget
		x0, y0 := int32(0), int32(0)
		w0, h0 := int32(100), int32(100)
		pane, err := a.createPanel("panel", 0, &x0, &y0, &w0, &h0)
		if err != nil {
			a.pendingParent = nil
			return nil, err
		}
		if i == 0 {
			widget.paneA = pane
		} else {
			widget.paneB = pane
		}
	}
	return widget, nil
}

func (a *App) createTabs(x, y, w, h *int32) (*Widget, error) {
	widget, err := a.createPanel("tabs", 0, x, y, w, h)
	if err != nil {
		return nil, err
	}
	widget.activeTab = -1
	return widget, nil
}

// addPage creates a tab header button and a hidden page panel. The first page
// becomes active immediately.
func (w *Widget) addPage(title string) (*Widget, error) {
	if w.kind != "tabs" {
		return nil, fmt.Errorf("add_page 只能用于 tabs")
	}
	a := w.app
	a.pendingParent = w
	btn, err := a.createButton(title, nil, nil, nil, nil)
	a.pendingParent = nil
	if err != nil {
		return nil, err
	}
	btn.tabHeader = true
	btn.tabOwner = w
	btn.tabIndex = len(w.tabPages)
	btn.defW = a.px(112)
	if a.darkTheme {
		_ = btn.SetProp("fg", "#E2E8F0")
		_ = btn.SetProp("bg", "#1E293B")
	}

	a.pendingParent = w
	x0, y0 := int32(0), int32(0)
	w0, h0 := int32(200), int32(120)
	page, err := a.createPanel("tabpage", 0, &x0, &y0, &w0, &h0)
	a.pendingParent = nil
	if err != nil {
		return nil, err
	}
	page.hidden = len(w.tabPages) > 0
	if page.hidden {
		showWindow(page.hwnd, swHide)
	} else {
		showWindow(page.hwnd, swShow)
	}
	w.tabPages = append(w.tabPages, page)
	w.tabButtons = append(w.tabButtons, btn)
	if w.activeTab < 0 {
		w.activateTab(0)
	}
	return page, nil
}

func (w *Widget) activateTab(idx int) {
	if w.kind != "tabs" || len(w.tabPages) == 0 {
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(w.tabPages) {
		idx = len(w.tabPages) - 1
	}
	w.activeTab = idx
	for i, page := range w.tabPages {
		page.hidden = i != idx
		if page.hidden {
			showWindow(page.hwnd, swHide)
		} else {
			showWindow(page.hwnd, swShow)
		}
	}
	for i, btn := range w.tabButtons {
		state := uintptr(0)
		if i == idx {
			state = 1
		}
		sendMessage(btn.hwnd, bmSetState, state, 0)
	}
	w.app.layoutTabs(w, w.w, w.h)
}

func (a *App) layoutToolbar(c *Widget, width, height int32) {
	margin := a.px(6)
	x := margin
	for _, w := range c.children {
		if w == nil || w.hwnd == 0 || w.hidden {
			continue
		}
		ww := w.defW
		if ww <= 0 {
			ww = w.w
		}
		hh := w.ctrlH
		if hh <= 0 {
			hh = height - 2*margin
		}
		if hh > height-2*margin {
			hh = height - 2*margin
		}
		moveWindow(w.hwnd, x, margin, ww, hh, true)
		w.x, w.y, w.w, w.h = x, margin, ww, hh
		x += ww + a.px(6)
	}
}

func (a *App) layoutSplitter(c *Widget, width, height int32) {
	margin := a.px(4)
	divider := a.px(6)
	if c.splitRatio <= 0 {
		c.splitRatio = 0.5
	}
	if c.splitRatio > 0.9 {
		c.splitRatio = 0.9
	}
	if c.splitRatio < 0.1 {
		c.splitRatio = 0.1
	}
	if c.paneA == nil || c.paneB == nil {
		for _, child := range c.children {
			if child != nil && child.kind == "panel" {
				if c.paneA == nil {
					c.paneA = child
				} else if c.paneB == nil {
					c.paneB = child
				}
			}
		}
	}
	if c.paneA == nil || c.paneB == nil {
		return
	}
	if c.splitVertical {
		avail := width - 2*margin - divider
		first := int32(float64(avail) * c.splitRatio)
		moveWindow(c.paneA.hwnd, margin, margin, first, height-2*margin, true)
		moveWindow(c.paneB.hwnd, margin+first+divider, margin, avail-first, height-2*margin, true)
		c.paneA.x, c.paneA.y, c.paneA.w, c.paneA.h = margin, margin, first, height-2*margin
		c.paneB.x, c.paneB.y, c.paneB.w, c.paneB.h = margin+first+divider, margin, avail-first, height-2*margin
		a.layoutContainer(c.paneA, first, height-2*margin)
		a.layoutContainer(c.paneB, avail-first, height-2*margin)
	} else {
		avail := height - 2*margin - divider
		first := int32(float64(avail) * c.splitRatio)
		moveWindow(c.paneA.hwnd, margin, margin, width-2*margin, first, true)
		moveWindow(c.paneB.hwnd, margin, margin+first+divider, width-2*margin, avail-first, true)
		c.paneA.x, c.paneA.y, c.paneA.w, c.paneA.h = margin, margin, width-2*margin, first
		c.paneB.x, c.paneB.y, c.paneB.w, c.paneB.h = margin, margin+first+divider, width-2*margin, avail-first
		a.layoutContainer(c.paneA, width-2*margin, first)
		a.layoutContainer(c.paneB, width-2*margin, avail-first)
	}
}

func (a *App) layoutTabs(c *Widget, width, height int32) {
	if len(c.tabPages) == 0 {
		return
	}
	margin := a.px(6)
	headerH := a.px(34)
	x := margin
	for _, btn := range c.tabButtons {
		if btn == nil || btn.hwnd == 0 {
			continue
		}
		ww := btn.defW
		if ww <= 0 {
			ww = a.px(112)
		}
		moveWindow(btn.hwnd, x, margin, ww, headerH-a.px(4), true)
		btn.x, btn.y, btn.w, btn.h = x, margin, ww, headerH-a.px(4)
		x += ww + a.px(4)
	}
	pageY := margin + headerH
	for i, page := range c.tabPages {
		if page == nil || page.hwnd == 0 {
			continue
		}
		if page.hidden || i != c.activeTab {
			continue
		}
		moveWindow(page.hwnd, margin, pageY, width-2*margin, height-pageY-margin, true)
		page.x, page.y, page.w, page.h = margin, pageY, width-2*margin, height-pageY-margin
		a.layoutContainer(page, width-2*margin, height-pageY-margin)
	}
}

// splitterHit reports whether the cursor is on the splitter divider.
func (w *Widget) splitterHit(x, y int32) bool {
	if w.splitVertical {
		divider := int32(float64(w.w) * w.splitRatio)
		return x >= divider-w.app.px(6) && x <= divider+w.app.px(6)
	}
	divider := int32(float64(w.h) * w.splitRatio)
	return y >= divider-w.app.px(6) && y <= divider+w.app.px(6)
}

func (w *Widget) startSplitDrag(x, y int32) bool {
	if !w.splitterHit(x, y) {
		return false
	}
	w.splitDrag = true
	pSetCapture.Call(uintptr(w.hwnd))
	return true
}

func (w *Widget) updateSplitDrag(x, y int32) {
	if !w.splitDrag || w.w <= 0 || w.h <= 0 {
		return
	}
	if w.splitVertical {
		r := float64(x) / float64(w.w)
		if r < 0.1 {
			r = 0.1
		}
		if r > 0.9 {
			r = 0.9
		}
		w.splitRatio = r
	} else {
		r := float64(y) / float64(w.h)
		if r < 0.1 {
			r = 0.1
		}
		if r > 0.9 {
			r = 0.9
		}
		w.splitRatio = r
	}
	w.app.layoutSplitter(w, w.w, w.h)
}

func (w *Widget) endSplitDrag() {
	if !w.splitDrag {
		return
	}
	w.splitDrag = false
	pReleaseCapture.Call()
}

// splitterPane returns one of the two splitter panes.
func (w *Widget) splitterPane(idx int) *Widget {
	if idx == 0 {
		return w.paneA
	}
	return w.paneB
}

// createPropertyGrid is a two-column table specialized for key/value pairs.
func (a *App) createPropertyGrid(rows [][]string, x, y, w, h *int32) (*Widget, error) {
	widget, err := a.createTable([]string{"属性", "值"}, rows, x, y, w, h, false)
	if err != nil {
		return nil, err
	}
	widget.editable = true
	return widget, nil
}

func (w *Widget) propertyRow(key string) int {
	if w.kind != "table" {
		return -1
	}
	n := int(sendMessage(w.hwnd, lvmGetItemCount, 0, 0))
	for i := 0; i < n; i++ {
		if w.tableCell(i, 0) == key {
			return i
		}
	}
	return -1
}

func (w *Widget) setProperty(key, val string) {
	if w.kind != "table" {
		return
	}
	row := w.propertyRow(key)
	if row < 0 {
		w.addRow([]string{key, val})
		return
	}
	w.setTableCell(int32(row), 1, val)
}

func (w *Widget) getProperty(key string) string {
	row := w.propertyRow(key)
	if row < 0 {
		return ""
	}
	return w.tableCell(row, 1)
}
