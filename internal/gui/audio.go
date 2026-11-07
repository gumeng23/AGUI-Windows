package gui

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	sndAsync     = 0x0001
	sndNoDefault = 0x0002
	sndMemory    = 0x0004
	sndLoop      = 0x0008
	sndPurge     = 0x0040
	sndFileName  = 0x00020000
)

var (
	winmm       = syscall.NewLazyDLL("winmm.dll")
	pPlaySoundW = winmm.NewProc("PlaySoundW")
)

// playWAV plays a WAV file asynchronously. loop repeats until stopWAV/another
// sound is played. A path of "" with stop=true purges the current sound.
func playWAV(path string, loop bool) error {
	if path == "" {
		pPlaySoundW.Call(0, 0, sndPurge)
		return nil
	}
	flags := uintptr(sndAsync | sndNoDefault | sndFileName)
	if loop {
		flags |= sndLoop
	}
	r, _, err := pPlaySoundW.Call(uintptr(unsafe.Pointer(u16(path))), 0, flags)
	if r == 0 {
		return fmt.Errorf("播放 WAV 失败: %v", err)
	}
	return nil
}

func stopWAV() {
	pPlaySoundW.Call(0, 0, sndPurge)
}

type audioChannel struct {
	alias   string
	path    string
	playing bool
	loop    bool
	volume  int
}

var (
	pMciSendStringW     = winmm.NewProc("mciSendStringW")
	pMciGetErrorStringW = winmm.NewProc("mciGetErrorStringW")
)

func mciCommand(cmd string) error {
	var ret [512]uint16
	r, _, _ := pMciSendStringW.Call(uintptr(unsafe.Pointer(u16(cmd))),
		uintptr(unsafe.Pointer(&ret[0])), uintptr(len(ret)), 0)
	if r == 0 {
		return nil
	}
	var msg [512]uint16
	pMciGetErrorStringW.Call(uintptr(r), uintptr(unsafe.Pointer(&msg[0])), uintptr(len(msg)))
	return fmt.Errorf("MCI 错误 %d: %s", r, syscall.UTF16ToString(msg[:]))
}

func audioAlias(channel string) string {
	if channel == "" {
		channel = "bgm"
	}
	var b []rune
	for _, r := range channel {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b = append(b, r)
		}
	}
	if len(b) == 0 {
		b = []rune("bgm")
	}
	return "agui_" + string(b)
}

func (a *App) closeAudio(channel string) {
	if ch, ok := a.audio[channel]; ok {
		mciCommand("close " + ch.alias)
		delete(a.audio, channel)
	}
}

func (a *App) playAudio(path, channel string, loop bool, volume int) error {
	resolved, err := a.interp.AssetPath(path)
	if err != nil {
		return err
	}
	a.closeAudio(channel)
	alias := audioAlias(channel)
	open := fmt.Sprintf("open \"%s\" type mpegvideo alias %s", resolved, alias)
	if err := mciCommand(open); err != nil {
		open = fmt.Sprintf("open \"%s\" type waveaudio alias %s", resolved, alias)
		if err2 := mciCommand(open); err2 != nil {
			return err
		}
	}
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	_ = mciCommand(fmt.Sprintf("setaudio %s volume to %d", alias, volume))
	play := "play " + alias
	if loop {
		play += " repeat"
	}
	if err := mciCommand(play); err != nil {
		mciCommand("close " + alias)
		return err
	}
	a.audio[channel] = &audioChannel{alias: alias, path: path, playing: true, loop: loop, volume: volume}
	return nil
}

func (a *App) stopAudio(channel string) {
	if ch, ok := a.audio[channel]; ok {
		mciCommand("stop " + ch.alias)
		ch.playing = false
	}
}

func (a *App) audioPlaying(channel string) bool {
	ch, ok := a.audio[channel]
	if !ok {
		return false
	}
	var ret [64]uint16
	r, _, _ := pMciSendStringW.Call(uintptr(unsafe.Pointer(u16("status "+ch.alias+" mode"))),
		uintptr(unsafe.Pointer(&ret[0])), uintptr(len(ret)), 0)
	if r != 0 {
		return ch.playing
	}
	mode := syscall.UTF16ToString(ret[:])
	return mode == "playing"
}

func (a *App) audioVolume(channel string, volume int) {
	ch, ok := a.audio[channel]
	if !ok {
		return
	}
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	ch.volume = volume
	_ = mciCommand(fmt.Sprintf("setaudio %s volume to %d", ch.alias, volume))
}
