package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		value float64
		want  string
	}{
		{-1, "?"},
		{0, "0.0 B"},
		{512, "512.0 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TB"},
	}
	for _, c := range cases {
		if got := FormatBytes(c.value); got != c.want {
			t.Errorf("FormatBytes(%v) = %q, want %q", c.value, got, c.want)
		}
	}
}

func TestFormatSpeed(t *testing.T) {
	cases := []struct {
		value float64
		want  string
	}{
		{-1, "?"},
		{0, "0.0 B/s"},
		{2048, "2.0 KB/s"},
	}
	for _, c := range cases {
		if got := FormatSpeed(c.value); got != c.want {
			t.Errorf("FormatSpeed(%v) = %q, want %q", c.value, got, c.want)
		}
	}
}

func TestFormatEta(t *testing.T) {
	cases := []struct {
		value float64
		want  string
	}{
		{-1, "?"},
		{0, "0:00"},
		{65, "1:05"},
		{3661, "1:01:01"},
	}
	for _, c := range cases {
		if got := FormatEta(c.value); got != c.want {
			t.Errorf("FormatEta(%v) = %q, want %q", c.value, got, c.want)
		}
	}
}

func TestCheckDownloadDirExistingWritableDir(t *testing.T) {
	if got := CheckDownloadDir(t.TempDir(), false); got != "" {
		t.Errorf("CheckDownloadDir = %q, want empty", got)
	}
}

func TestCheckDownloadDirMissingWithoutCreate(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if got := CheckDownloadDir(missing, false); got != "error.notADirectory" {
		t.Errorf("CheckDownloadDir = %q, want error.notADirectory", got)
	}
}

func TestCheckDownloadDirCreatesMissingDir(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nested", "downloads")
	if got := CheckDownloadDir(target, true); got != "" {
		t.Errorf("CheckDownloadDir = %q, want empty", got)
	}
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		t.Errorf("target %q was not created as a directory", target)
	}
}

func TestCheckDownloadDirPathIsAFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := CheckDownloadDir(f, true); got != "error.notADirectory" {
		t.Errorf("CheckDownloadDir = %q, want error.notADirectory", got)
	}
}
