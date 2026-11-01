package gui

// ShowError displays a modal error dialog that works even when the program runs
// without a console (used for bundled .exe files).
func ShowError(title, text string) {
	messageBox(0, text, title, mbOK|mbIconError)
}

// ShowInfo displays a modal information dialog.
func ShowInfo(title, text string) {
	messageBox(0, text, title, mbOK|mbIconInformation)
}

// After schedules a one-shot callback on the GUI thread and returns its timer id.
// It is mainly useful for hosts that need to act once the window is painted.
func (a *App) After(ms int, fn func() error) uintptr {
	return a.addTimer(ms, false, fn)
}
