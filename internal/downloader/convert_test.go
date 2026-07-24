package downloader

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"video-downloader-go/internal/utils"
)

func TestParseDestLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantBase string
		wantExt  string
		wantOK   bool
	}{
		{"normal", `C:\out\Title.mp4`, `C:\out\Title`, "mp4", true},
		{"blank", "  ", "", "", false},
		{"noExtension", `C:\out\Title`, "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ext, ok := parseDestLine(tt.line)
			if ok != tt.wantOK || base != tt.wantBase || ext != tt.wantExt {
				t.Fatalf("parseDestLine(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.line, base, ext, ok, tt.wantBase, tt.wantExt, tt.wantOK)
			}
		})
	}
}

func TestReadAllDestBaseAndExt_MultipleLines(t *testing.T) {
	dir := t.TempDir()
	destFile := filepath.Join(dir, "dest.txt")
	content := "C:\\out\\Item1 \uff5c One.webm\r\nC:\\out\\Item2 \uff5c Two.mp4\r\n"
	if err := os.WriteFile(destFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	refs, ok := readAllDestBaseAndExt(destFile)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := []fileRef{
		{Base: "C:\\out\\Item1 \uff5c One", Ext: "webm"},
		{Base: "C:\\out\\Item2 \uff5c Two", Ext: "mp4"},
	}
	if len(refs) != len(want) || refs[0] != want[0] || refs[1] != want[1] {
		t.Fatalf("refs = %+v, want %+v", refs, want)
	}
}

func TestReadAllDestBaseAndExt_SkipsBlankLines(t *testing.T) {
	dir := t.TempDir()
	destFile := filepath.Join(dir, "dest.txt")
	content := "C:\\out\\A.mp4\r\n\r\nC:\\out\\B.mp4\r\n"
	if err := os.WriteFile(destFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	refs, ok := readAllDestBaseAndExt(destFile)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(refs) != 2 {
		t.Fatalf("refs = %+v, want 2 entries (blank line skipped)", refs)
	}
}

func TestReadAllDestBaseAndExt_EmptyFileReturnsNoRefsButOK(t *testing.T) {
	dir := t.TempDir()
	destFile := filepath.Join(dir, "dest.txt")
	if err := os.WriteFile(destFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	refs, ok := readAllDestBaseAndExt(destFile)
	if !ok {
		t.Fatal("expected ok=true: the file itself was readable, it just has nothing in it yet")
	}
	if len(refs) != 0 {
		t.Fatalf("refs = %+v, want none", refs)
	}
}

func TestReadAllDestBaseAndExt_MissingFileMeansNotOK(t *testing.T) {
	if _, ok := readAllDestBaseAndExt(filepath.Join(t.TempDir(), "does-not-exist.txt")); ok {
		t.Fatal("expected ok=false for a missing file")
	}
}

func TestRemuxArgs_MapsVideoRequiredAudioOptional(t *testing.T) {
	args := remuxArgs(`C:\out\Title.mkv`, `C:\out\Title.mp4`)
	for _, want := range []string{"-map 0:v:0", "-map 0:a:0?", "-c copy"} {
		if !containsAdjacentPair(args, want) {
			t.Errorf("remuxArgs = %v, missing %q", args, want)
		}
	}
	if args[len(args)-1] != `C:\out\Title.mp4` {
		t.Errorf("remuxArgs last element = %q, want the destination path last", args[len(args)-1])
	}
}

func TestEncodeArgs_UsesContainerSpec(t *testing.T) {
	spec := videoContainerSpecs["webm"]
	args := encodeArgs(spec, `C:\out\Title.mkv`, `C:\out\Title.webm`)
	for _, want := range []string{"-map 0:v:0", "-map 0:a:0?", "-c:v libvpx-vp9", "-c:a libopus"} {
		if !containsAdjacentPair(args, want) {
			t.Errorf("encodeArgs = %v, missing %q", args, want)
		}
	}
	if args[len(args)-1] != `C:\out\Title.webm` {
		t.Errorf("encodeArgs last element = %q, want the destination path last", args[len(args)-1])
	}
}

// TestVideoContainerSpecs_CoversEveryVideoConvertOption guards against the
// silent runtime failure convertVideo would hit ("no conversion recipe")
// if utils.VideoConvertOptions ever grows a format this package doesn't
// know how to fall back to encoding.
func TestVideoContainerSpecs_CoversEveryVideoConvertOption(t *testing.T) {
	for _, opt := range utils.VideoConvertOptions {
		if opt == "original" {
			continue
		}
		if _, ok := videoContainerSpecs[opt]; !ok {
			t.Errorf("videoContainerSpecs has no entry for utils.VideoConvertOptions value %q", opt)
		}
	}
}

func TestConvertDownloaded_SkipsAlreadyMatchingExtension(t *testing.T) {
	dir := t.TempDir()
	destFile := filepath.Join(dir, "dest.txt")
	if err := os.WriteFile(destFile, []byte(filepath.Join(dir, "Title")+".mp4\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A bogus ffmpeg path would error immediately if convertVideo actually
	// tried to run it -- reaching return nil proves the same-extension item
	// was skipped without ever attempting to convert it.
	err := convertDownloaded(context.Background(), "does-not-exist-ffmpeg", destFile, "mp4", Callbacks{})
	if err != nil {
		t.Fatalf("convertDownloaded() = %v, want nil (nothing needed converting)", err)
	}
}

func TestConvertDownloaded_SkipsMissingSourceFile(t *testing.T) {
	dir := t.TempDir()
	destFile := filepath.Join(dir, "dest.txt")
	// Source file base+".webm" is never created on disk -- simulates a
	// playlist item whose download failed after before_dl already logged
	// it.
	if err := os.WriteFile(destFile, []byte(filepath.Join(dir, "NeverDownloaded")+".webm\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := convertDownloaded(context.Background(), "does-not-exist-ffmpeg", destFile, "mp4", Callbacks{})
	if err != nil {
		t.Fatalf("convertDownloaded() = %v, want nil (missing source skipped, not errored)", err)
	}
}

func TestConvertDownloaded_ErrorsOnUnreadableDestFile(t *testing.T) {
	err := convertDownloaded(context.Background(), "ffmpeg", filepath.Join(t.TempDir(), "missing.txt"), "mp4", Callbacks{})
	if err == nil {
		t.Fatal("expected an error for a destination-list file that can't be read")
	}
}

func TestConvertDownloaded_CancelledContextStopsBeforeConverting(t *testing.T) {
	dir := t.TempDir()
	destFile := filepath.Join(dir, "dest.txt")
	base := filepath.Join(dir, "Title")
	mustWrite(t, base+".webm", "not a real video, just needs to exist")
	if err := os.WriteFile(destFile, []byte(base+".webm\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// The source file exists and its extension differs from convertTo, so
	// without the ctx check this would actually try to spawn ffmpeg.
	err := convertDownloaded(ctx, "does-not-exist-ffmpeg", destFile, "mp4", Callbacks{})
	if err != ErrCancelled {
		t.Fatalf("convertDownloaded() = %v, want ErrCancelled", err)
	}
}

// containsAdjacentPair reports whether args contains two consecutive
// elements matching "flag value", e.g. "-c copy" for args [..., "-c",
// "copy", ...].
func containsAdjacentPair(args []string, flagAndValue string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i]+" "+args[i+1] == flagAndValue {
			return true
		}
	}
	return false
}
