package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// targetInfo is the build metadata carried in the leading comment block of an
// Agui source file. A source file is still valid without one: the default
// target is Windows.
type targetInfo struct {
	OS          string
	Name        string
	Package     string
	Version     string
	Icon        string
	Permissions []string
}

func parseTargetHeader(src string) (targetInfo, error) {
	src = strings.TrimPrefix(src, "\ufeff")
	t := targetInfo{OS: "windows"}
	lines := strings.Split(src, "\n")
	seen := false
	seenTarget := false
	for i, raw := range lines {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" {
			if seen {
				continue
			}
			continue
		}
		if !strings.HasPrefix(line, "#") {
			break
		}
		seen = true
		body := strings.TrimSpace(strings.TrimPrefix(line, "#"))
		key, value := body, ""
		if k, v, ok := strings.Cut(body, " "); ok {
			key, value = k, strings.TrimSpace(v)
		}
		key = strings.ToLower(strings.TrimSpace(key))
		switch key {
		case "target":
			osName := strings.ToLower(strings.TrimSpace(value))
			if osName != "windows" && osName != "android" {
				return t, fmt.Errorf("第 %d 行：#target 只能是 windows 或 android，得到 %q", i+1, value)
			}
			if seenTarget {
				return t, fmt.Errorf("第 %d 行：重复的 #target", i+1)
			}
			seenTarget = true
			t.OS = osName
		case "name":
			t.Name = value
		case "package":
			t.Package = value
		case "version":
			t.Version = value
		case "icon":
			t.Icon = value
		case "permission":
			for _, p := range strings.Fields(value) {
				p = strings.TrimSpace(p)
				if !strings.HasPrefix(p, "android.permission.") || strings.ContainsAny(p, " \t\r\n\"'<>") {
					return t, fmt.Errorf("第 %d 行：#permission 必须是 android.permission.* 权限名，得到 %q", i+1, p)
				}
				dup := false
				for _, old := range t.Permissions {
					if old == p {
						dup = true
						break
					}
				}
				if !dup {
					t.Permissions = append(t.Permissions, p)
				}
			}
		}
	}
	if t.Name == "" {
		t.Name = "Agui 应用"
	}
	if t.Version == "" {
		t.Version = version
	}
	if t.Package == "" {
		t.Package = "com.agui." + packageSlug(t.Name)
	}
	if t.OS != "android" && t.OS != "windows" {
		return t, fmt.Errorf("#target 只能是 windows 或 android")
	}
	if t.OS == "android" {
		if !validPackage(t.Package) {
			return t, fmt.Errorf("#package 不是合法的 Android 包名：%q", t.Package)
		}
	}
	return t, nil
}

var packageSlugRe = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func packageSlug(s string) string {
	s = packageSlugRe.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		s = "app"
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "app_" + s
	}
	return strings.ToLower(s)
}

func validPackage(p string) bool {
	parts := strings.Split(p, ".")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for i, r := range part {
			if i == 0 {
				if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
					return false
				}
			} else if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
				return false
			}
		}
	}
	return true
}

func readTargetHeader(path string) (targetInfo, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return targetInfo{}, "", err
	}
	src := string(data)
	t, err := parseTargetHeader(src)
	if err != nil {
		return t, src, fmt.Errorf("%s：%w", filepath.Base(path), err)
	}
	return t, src, nil
}
