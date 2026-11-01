package lang

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// AssetStore is the read-only virtual filesystem embedded into a packaged exe.
// Paths use forward slashes and are relative to the program root.
type AssetStore struct {
	mu        sync.Mutex
	files     map[string][]byte
	baseDir   string
	cacheDir  string
	extracted map[string]string
}

func newAssetStore() *AssetStore {
	return &AssetStore{
		files:     map[string][]byte{},
		extracted: map[string]string{},
	}
}

// SetAssets installs embedded files. baseDir is the directory used for
// relative paths that are not embedded (normally the directory of the exe).
func (i *Interp) SetAssets(files map[string][]byte, baseDir string) {
	i.assets.mu.Lock()
	defer i.assets.mu.Unlock()
	i.assets.files = files
	i.assets.baseDir = baseDir
}

// SetBaseDir sets the root for relative file paths.
func (i *Interp) SetBaseDir(dir string) {
	i.assets.mu.Lock()
	i.assets.baseDir = dir
	i.assets.mu.Unlock()
}

// BaseDir returns the root for relative file paths.
func (i *Interp) BaseDir() string {
	i.assets.mu.Lock()
	defer i.assets.mu.Unlock()
	if i.assets.baseDir == "" {
		return "."
	}
	return i.assets.baseDir
}

func normalizeAssetPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = path.Clean(p)
	p = strings.TrimPrefix(p, "./")
	return p
}

// AssetBytes returns an embedded file without touching the disk.
func (i *Interp) AssetBytes(name string) ([]byte, bool) {
	key := normalizeAssetPath(name)
	i.assets.mu.Lock()
	defer i.assets.mu.Unlock()
	data, ok := i.assets.files[key]
	return data, ok
}

// HasAsset reports whether a path is embedded.
func (i *Interp) HasAsset(name string) bool {
	_, ok := i.AssetBytes(name)
	return ok
}

// AssetNames lists embedded paths (sorted), optionally below a directory.
func (i *Interp) AssetNames(dir string) []string {
	prefix := normalizeAssetPath(dir)
	if prefix == "." {
		prefix = ""
	}
	i.assets.mu.Lock()
	defer i.assets.mu.Unlock()
	var out []string
	for name := range i.assets.files {
		if prefix == "" || name == prefix || strings.HasPrefix(name, prefix+"/") {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// ResolvePath maps a relative script path to a real filesystem path.
func (i *Interp) ResolvePath(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(i.BaseDir(), filepath.FromSlash(name))
}

// ReadFile reads an embedded asset first, then a real file. It is safe to call
// from the UI thread; callers that run as a background task should wrap it in
// Interp.Blocking when the file may be large.
func (i *Interp) ReadFile(name string) ([]byte, error) {
	if data, ok := i.AssetBytes(name); ok {
		return data, nil
	}
	return os.ReadFile(i.ResolvePath(name))
}

// AssetPath returns a real path for GDI+/Win32 APIs that require a filename.
// Embedded assets are extracted once into the user cache directory.
func (i *Interp) AssetPath(name string) (string, error) {
	if data, ok := i.AssetBytes(name); ok {
		sum := sha256.Sum256(data)
		key := hex.EncodeToString(sum[:8]) + "-" + filepath.Base(filepath.FromSlash(normalizeAssetPath(name)))
		i.assets.mu.Lock()
		if p, ok := i.assets.extracted[key]; ok {
			i.assets.mu.Unlock()
			return p, nil
		}
		cache := i.assets.cacheDir
		i.assets.mu.Unlock()
		if cache == "" {
			base, err := os.UserCacheDir()
			if err != nil {
				base = os.TempDir()
			}
			cache = filepath.Join(base, "agui", "assets")
		}
		if err := os.MkdirAll(cache, 0o755); err != nil {
			tmp, tmpErr := os.MkdirTemp("", "agui-assets-")
			if tmpErr != nil {
				cache = filepath.Join(i.BaseDir(), ".agui-cache")
				if err2 := os.MkdirAll(cache, 0o755); err2 != nil {
					return "", fmt.Errorf("无法创建资源缓存目录: %v / %v", err, err2)
				}
			} else {
				cache = tmp
			}
		}
		dst := filepath.Join(cache, key)
		if _, err := os.Stat(dst); err != nil {
			tmp := dst + ".tmp"
			if err := os.WriteFile(tmp, data, 0o644); err != nil {
				return "", fmt.Errorf("无法释放内嵌资源: %v", err)
			}
			_ = os.Rename(tmp, dst)
		}
		i.assets.mu.Lock()
		i.assets.extracted[key] = dst
		i.assets.mu.Unlock()
		return dst, nil
	}
	return i.ResolvePath(name), nil
}

// FileExists checks embedded assets and the real filesystem.
func (i *Interp) FileExists(name string) bool {
	if i.HasAsset(name) {
		return true
	}
	_, err := os.Stat(i.ResolvePath(name))
	return err == nil
}
