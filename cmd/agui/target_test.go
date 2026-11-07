package main

import (
	"strings"
	"testing"
)

func TestTargetHeaderDefaults(t *testing.T) {
	ti, err := parseTargetHeader("print(\"hi\")\n")
	if err != nil {
		t.Fatal(err)
	}
	if ti.OS != "windows" {
		t.Fatalf("OS = %q", ti.OS)
	}
	if ti.Name != "Agui 应用" {
		t.Fatalf("Name = %q", ti.Name)
	}
	if !strings.HasPrefix(ti.Package, "com.agui.") {
		t.Fatalf("Package = %q", ti.Package)
	}
}

func TestTargetHeaderAndroid(t *testing.T) {
	src := "#target android\n#name 星野养成\n#package com.agui.star\n#version 0.1.0\n#icon assets/icon.png\nwin = window(\"x\")\n"
	ti, err := parseTargetHeader(src)
	if err != nil {
		t.Fatal(err)
	}
	if ti.OS != "android" || ti.Name != "星野养成" || ti.Package != "com.agui.star" || ti.Version != "0.1.0" || ti.Icon != "assets/icon.png" {
		t.Fatalf("unexpected target: %+v", ti)
	}
}

func TestTargetHeaderPermissions(t *testing.T) {
	src := `#target android
#package com.agui.perm
#permission android.permission.CAMERA android.permission.POST_NOTIFICATIONS
#permission android.permission.CAMERA
win = window("x")
`
	ti, err := parseTargetHeader(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(ti.Permissions) != 2 || ti.Permissions[0] != "android.permission.CAMERA" || ti.Permissions[1] != "android.permission.POST_NOTIFICATIONS" {
		t.Fatalf("permissions = %#v", ti.Permissions)
	}
}

func TestTargetHeaderErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"dup", "#target windows\n#target android\n", "第 2 行"},
		{"bad", "#target ios\n", "#target 只能是 windows 或 android"},
		{"pkg", "#target android\n#package bad package\n", "不是合法的 Android 包名"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := parseTargetHeader(c.src); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want contains %q", err, c.want)
			}
		})
	}
}

func TestTargetHeaderBOM(t *testing.T) {
	bom := string([]byte{0xEF, 0xBB, 0xBF})
	ti, err := parseTargetHeader(bom + "#target android\n#package com.agui.bom\n")
	if err != nil {
		t.Fatal(err)
	}
	if ti.OS != "android" {
		t.Fatalf("OS = %q", ti.OS)
	}
}

func TestTargetHeaderStopsAtCode(t *testing.T) {
	ti, err := parseTargetHeader("print(\"x\")\n#target android\n")
	if err != nil {
		t.Fatal(err)
	}
	if ti.OS != "windows" {
		t.Fatalf("header after code must be ignored, got %q", ti.OS)
	}
}
