package main

import (
	"archive/zip"
	"bytes"
	"testing"
)

func zipWithFiles(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractUpdateExePicksLargest(t *testing.T) {
	raw := zipWithFiles(t, map[string]string{
		"readme.txt": "x",
		"small.exe":  "a",
		"large.exe":  "larger payload",
	})
	got, err := extractUpdateExe(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "larger payload" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractUpdateExeRejectsBadZip(t *testing.T) {
	if _, err := extractUpdateExe([]byte("not a zip")); err == nil {
		t.Fatal("expected error")
	}
}
