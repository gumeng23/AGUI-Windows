package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// actCtxW mirrors ACTCTXW. The activation context is what makes common
// controls use the current visual style (comctl32 v6) without embedding a
// manifest resource into the executable.
type actCtxW struct {
	cbSize                 uint32
	dwFlags                uint32
	lpSource               *uint16
	wProcessorArchitecture uint16
	wLangId                uint16
	lpAssemblyDirectory    *uint16
	lpResourceName         *uint16
	lpApplicationName      *uint16
	hModule                syscall.Handle
}

var (
	visualCtx      syscall.Handle
	visualCookie   uintptr
	visualManifest string
)

const visualManifestXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity type="win32" name="Agui.Application" version="1.0.0.0" processorArchitecture="*"/>
  <description>Agui application</description>
  <dependency>
    <dependentAssembly>
      <assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0" processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*"/>
    </dependentAssembly>
  </dependency>
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application>
      <supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>
    </application>
  </compatibility>
</assembly>
`

// enableVisualStyles activates a side-by-side manifest for the current thread.
// Keep the returned context alive for the whole process; deactivation happens
// only when the process exits.
func enableVisualStyles() error {
	if visualCtx != 0 {
		return nil
	}
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	dir = filepath.Join(dir, "agui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		dir = os.TempDir()
	}
	f, err := os.CreateTemp(dir, "agui-*.manifest")
	if err != nil {
		return err
	}
	path := f.Name()
	if _, err := f.WriteString(visualManifestXML); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return err
	}
	ctx := actCtxW{
		cbSize:   uint32(unsafe.Sizeof(actCtxW{})),
		lpSource: u16(path),
	}
	h, _, callErr := pCreateActCtxW.Call(uintptr(unsafe.Pointer(&ctx)))
	if h == 0 || h == ^uintptr(0) {
		os.Remove(path)
		if callErr != nil && callErr.Error() != "The operation completed successfully." {
			return fmt.Errorf("创建视觉样式清单失败: %v", callErr)
		}
		return fmt.Errorf("创建视觉样式清单失败")
	}
	var cookie uintptr
	ok, _, actErr := pActivateActCtx.Call(h, uintptr(unsafe.Pointer(&cookie)))
	if ok == 0 {
		pReleaseActCtx.Call(h)
		os.Remove(path)
		return fmt.Errorf("激活视觉样式失败: %v", actErr)
	}
	visualCtx = syscall.Handle(h)
	visualCookie = cookie
	visualManifest = path
	return nil
}

func disableVisualStyles() {
	if visualCtx == 0 {
		return
	}
	pDeactivateActCtx.Call(0, visualCookie)
	pReleaseActCtx.Call(uintptr(visualCtx))
	visualCtx = 0
	visualCookie = 0
	if visualManifest != "" {
		os.Remove(visualManifest)
		visualManifest = ""
	}
}
