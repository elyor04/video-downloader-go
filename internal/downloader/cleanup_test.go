package downloader

import (
	"os"
	"path/filepath"
	"testing"
)

// Lines below are verbatim from real bin/yt-dlp.exe runs (captured while
// investigating what files get left behind on cancel), not guessed.

func TestTrackCleanupCandidate_RawDownloadDestinationSuffix(t *testing.T) {
	var tr cleanupTracker
	trackCleanupCandidate(&tr, `[download] Destination: C:\Users\Elyor\Downloads\Skyfall.f396.mp4`)
	if _, ok := tr.suffixes[".f396.mp4"]; !ok {
		t.Fatalf("suffixes = %v, want .f396.mp4 present", tr.suffixes)
	}
}

func TestTrackCleanupCandidate_SingleFormatNoIDSuffix(t *testing.T) {
	var tr cleanupTracker
	trackCleanupCandidate(&tr, `[download] Destination: C:\Users\Elyor\Downloads\Tutorial.mp4`)
	if _, ok := tr.suffixes[".mp4"]; !ok {
		t.Fatalf("suffixes = %v, want .mp4 present", tr.suffixes)
	}
}

func TestTrackCleanupCandidate_DeletingOriginalFileRemovesSuffix(t *testing.T) {
	var tr cleanupTracker
	trackCleanupCandidate(&tr, `[download] Destination: C:\Users\Elyor\Downloads\Skyfall.f396.mp4`)
	trackCleanupCandidate(&tr, `[download] Destination: C:\Users\Elyor\Downloads\Skyfall.f251.webm`)
	trackCleanupCandidate(&tr, `Deleting original file C:\Users\Elyor\Downloads\Skyfall.f396.mp4 (pass -k to keep)`)
	if _, ok := tr.suffixes[".f396.mp4"]; ok {
		t.Fatalf("suffixes = %v, .f396.mp4 should have been removed", tr.suffixes)
	}
	if _, ok := tr.suffixes[".f251.webm"]; !ok {
		t.Fatalf("suffixes = %v, .f251.webm should still be pending", tr.suffixes)
	}
}

func TestTrackCleanupCandidate_UnrecognizedLinesAreIgnored(t *testing.T) {
	var tr cleanupTracker
	trackCleanupCandidate(&tr, `[youtube] Extracting URL: https://example.com`)
	trackCleanupCandidate(&tr, `{"progress":{"status":"downloading"}}`)
	trackCleanupCandidate(&tr, `CONVERTING`)
	trackCleanupCandidate(&tr, `[Merger] Merging formats into "C:\out\Title.webm"`)
	if len(tr.suffixes) != 0 {
		t.Fatalf("expected no tracked suffixes from unrelated/unhandled lines, got %v", tr.suffixes)
	}
}

func TestTrackCleanupCandidate_DuplicateDestinationDeduped(t *testing.T) {
	var tr cleanupTracker
	trackCleanupCandidate(&tr, `[download] Destination: C:\out\Title.mp4`)
	trackCleanupCandidate(&tr, `[download] Destination: C:\out\Title.mp4`)
	if len(tr.suffixes) != 1 {
		t.Fatalf("expected exactly one entry, got %v", tr.suffixes)
	}
}

func TestReadLastDestBaseAndExt(t *testing.T) {
	dir := t.TempDir()
	destFile := filepath.Join(dir, "dest.txt")

	// Two lines mimics a playlist: the earlier item must have fully
	// finished (yt-dlp only logs the next one once the previous is done),
	// so the LAST line is always the one that could still be in flight.
	content := "C:\\out\\Item1 ｜ One.webm\r\nC:\\out\\Item2 ｜ Two.mp4\r\n"
	if err := os.WriteFile(destFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	base, ext, ok := readLastDestBaseAndExt(destFile)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if base != "C:\\out\\Item2 ｜ Two" {
		t.Fatalf("base = %q, want %q", base, "C:\\out\\Item2 ｜ Two")
	}
	if ext != "mp4" {
		t.Fatalf("ext = %q, want mp4", ext)
	}
}

func TestReadLastDestBaseAndExt_EmptyFileMeansNotOK(t *testing.T) {
	dir := t.TempDir()
	destFile := filepath.Join(dir, "dest.txt")
	if err := os.WriteFile(destFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := readLastDestBaseAndExt(destFile); ok {
		t.Fatal("expected ok=false for an empty file (cancelled before before_dl ever fired)")
	}
}

func TestReadLastDestBaseAndExt_MissingFileMeansNotOK(t *testing.T) {
	if _, _, ok := readLastDestBaseAndExt(filepath.Join(t.TempDir(), "does-not-exist.txt")); ok {
		t.Fatal("expected ok=false for a missing file")
	}
}

func TestCleanupTracker_Cleanup_RemovesRawMergeAndRecodeTargets(t *testing.T) {
	dir := t.TempDir()
	destFile := filepath.Join(dir, "dest.txt")
	base := filepath.Join(dir, "Title")
	if err := os.WriteFile(destFile, []byte(base+".webm\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Raw per-format source still mid-download (.part only).
	mustWrite(t, base+".f396.mp4.part", "partial")
	// Merge target, incomplete.
	mustWrite(t, base+".webm", "incomplete-merge")
	// Recode target, corrupt/truncated.
	mustWrite(t, base+".mp4", "corrupt")

	tr := cleanupTracker{suffixes: map[string]struct{}{".f396.mp4": {}}}
	tr.cleanup(destFile, "mp4")

	for _, p := range []string{base + ".f396.mp4.part", base + ".webm", base + ".mp4"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed, stat err = %v", p, err)
		}
	}
}

func TestCleanupTracker_Cleanup_LeavesUnrelatedFilesAlone(t *testing.T) {
	dir := t.TempDir()
	destFile := filepath.Join(dir, "dest.txt")
	base := filepath.Join(dir, "Title")
	if err := os.WriteFile(destFile, []byte(base+".webm\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	unrelated := filepath.Join(dir, "SomeoneElsesFile.mp4")
	mustWrite(t, unrelated, "do not touch")

	tr := cleanupTracker{}
	tr.cleanup(destFile, "original")

	if _, err := os.Stat(unrelated); err != nil {
		t.Fatalf("unrelated file should have survived cleanup, stat err = %v", err)
	}
}

func TestCleanupTracker_Cleanup_NoOpWhenDestFileEmpty(t *testing.T) {
	dir := t.TempDir()
	destFile := filepath.Join(dir, "dest.txt")
	if err := os.WriteFile(destFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	tr := cleanupTracker{suffixes: map[string]struct{}{".mp4": {}}}
	tr.cleanup(destFile, "mp4") // must not panic
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveWithRetry_ToleratesMissingFile(t *testing.T) {
	removeWithRetry(filepath.Join(t.TempDir(), "never-existed.mp4")) // must not panic or block
}
