package downloader

import (
	"os"
	"regexp"
	"strings"
	"time"
)

// cleanupTracker records what a runAttempt needs to remove a cancelled
// download's leftovers precisely -- never by globbing the output
// directory, only by paths yt-dlp itself is confirmed to have used.
//
// yt-dlp has a real inconsistency worth documenting: plain-text lines it
// writes to stdout (e.g. "[download] Destination: X") can silently mangle
// the title if it contains characters illegal in a Windows filename (e.g.
// "|"), because those get dropped when yt-dlp's own console-encoding path
// renders them -- verified by running the bundled yt-dlp against a real
// title containing "|": the stdout line reported it as two plain spaces,
// while the actual file on disk (and even yt-dlp's own `--print-to-file`,
// which writes bytes directly to a file instead of through that
// console-encoding path) used the correct fullwidth "｜" substitute. So the
// *base* filename this package works with always comes from
// `--print-to-file "before_dl:%(filename)s"` (byte-accurate, one line per
// video/playlist item; see readLastDestBaseAndExt), never from stdout --
// stdout is only trusted for the *suffix* (format id + extension, always
// plain ASCII, never affected by the mangling) of the raw per-format
// downloads, since before_dl only reports the eventual merged/single-
// stream name, not each interim format's temp filename. The merge and
// recode/extract targets don't need stdout parsing at all: the merge
// target is exactly what before_dl already reports, and the recode/
// extract target is base+"."+Params.ConvertTo, which this app chooses
// itself rather than yt-dlp deciding it.
type cleanupTracker struct {
	suffixes map[string]struct{}
}

// destinationSuffixRe pulls the trailing "(.f<formatid>)?.<ext>" off a
// yt-dlp-reported destination, e.g. "...Title.f396.mp4" -> ".f396.mp4",
// "...Title.mp4" -> ".mp4". Matched from the end, so it's unaffected by
// whatever mangling happened earlier in the string.
var destinationSuffixRe = regexp.MustCompile(`(\.f[A-Za-z0-9]+)?\.[A-Za-z0-9]+$`)

func (c *cleanupTracker) addDestination(mangledPath string) {
	suffix := destinationSuffixRe.FindString(mangledPath)
	if suffix == "" {
		return
	}
	if c.suffixes == nil {
		c.suffixes = make(map[string]struct{})
	}
	c.suffixes[suffix] = struct{}{}
}

func (c *cleanupTracker) removeDestination(mangledPath string) {
	suffix := destinationSuffixRe.FindString(mangledPath)
	delete(c.suffixes, suffix)
}

// trackCleanupCandidate scans one line of yt-dlp's stdout for the raw
// per-format download announcement, or for yt-dlp confirming it already
// deleted one itself after a successful merge/recode/extract (in which
// case there's nothing left for us to do about that particular file).
// Verified against the bundled yt-dlp by actually running it:
//
//	[download] Destination: <path>
//	Deleting original file <path> (pass -k to keep)
//
// Purely additive to handleStdoutLine's progress/stage parsing -- if a
// future yt-dlp version rewords these, the switch below just matches
// nothing and cleanup does less than it could, never something wrong.
func trackCleanupCandidate(tracker *cleanupTracker, line string) {
	line = strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(line, "[download] Destination: "):
		tracker.addDestination(strings.TrimPrefix(line, "[download] Destination: "))
	case strings.HasPrefix(line, "Deleting original file "):
		path := strings.TrimPrefix(line, "Deleting original file ")
		tracker.removeDestination(strings.TrimSuffix(path, " (pass -k to keep)"))
	}
}

// fileRef is one "before_dl" line's destination, split into its
// extension-stripped base and bare extension -- see readAllDestBaseAndExt.
type fileRef struct {
	Base, Ext string
}

// parseDestLine splits one `--print-to-file "before_dl:%(filename)s"` line
// into its extension-stripped base and bare extension (e.g.
// base=`C:\out\Title`, ext=`webm`). ok=false for a blank line or one with no
// extension separator.
func parseDestLine(line string) (base, ext string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", "", false
	}
	dot := strings.LastIndexByte(line, '.')
	if dot < 0 {
		return "", "", false
	}
	return line[:dot], line[dot+1:], true
}

// readLastDestBaseAndExt reads the file `--print-to-file
// "before_dl:%(filename)s"` was told to append to and returns the last
// line's path split via parseDestLine. "before_dl" fires once per video --
// once per playlist item, in strict sequence -- so the *last* line is
// always the item that was still in flight when the job was cancelled;
// every earlier item must have already finished (including its own
// merge/recode) before yt-dlp could move on and log the next one. Returns
// ok=false if the file is empty (cancelled before yt-dlp got far enough to
// write anything, meaning there's nothing on disk to clean up yet either).
func readLastDestBaseAndExt(path string) (base, ext string, ok bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", false
	}
	lines := strings.Split(strings.TrimRight(string(data), "\r\n"), "\n")
	return parseDestLine(lines[len(lines)-1])
}

// readAllDestBaseAndExt is readLastDestBaseAndExt's multi-item counterpart,
// used by convert.go: a playlist download appends one "before_dl" line per
// item, in order, and every downloaded item needs converting -- not just
// the last/in-flight one cleanup cares about. Blank or malformed lines are
// skipped rather than aborting the whole read; ok=false only when the file
// itself can't be read.
func readAllDestBaseAndExt(path string) (refs []fileRef, ok bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	lines := strings.Split(strings.TrimRight(string(data), "\r\n"), "\n")
	for _, line := range lines {
		if base, ext, lineOk := parseDestLine(line); lineOk {
			refs = append(refs, fileRef{Base: base, Ext: ext})
		}
	}
	return refs, true
}

// cleanup removes every remaining raw per-format candidate (base+suffix,
// both bare and with yt-dlp's ".part" temp-file suffix), the merge-or-
// single-stream target (base+"."+ext, from before_dl), and -- if a
// convert/recode was requested -- its target (base+"."+convertTo, derived
// directly from Params rather than parsed, since we choose that extension
// ourselves and yt-dlp doesn't rename it partway through). Best-effort and
// silent: cancellation must never fail or block on this. Missing files are
// expected (most of these candidates are guesses that didn't pan out) and
// ignored.
func (c *cleanupTracker) cleanup(destFile, convertTo string) {
	base, ext, ok := readLastDestBaseAndExt(destFile)
	if !ok {
		return
	}

	for suffix := range c.suffixes {
		removeWithRetry(base + suffix)
		removeWithRetry(base + suffix + ".part")
	}

	removeWithRetry(base + "." + ext)
	if convertTo != "" && convertTo != "original" && convertTo != ext {
		removeWithRetry(base + "." + convertTo)
	}
}

// removeWithRetry gives a file a few short chances to become deletable
// before giving up silently. Right after killTree/cancelTree, the OS is
// expected to have already released yt-dlp/ffmpeg's file handles by the
// time cmd.Wait() returns -- but a brief retry is cheap insurance against
// a transient sharing violation (e.g. antivirus/indexing) without risking
// a real hang: worst case this adds ~150ms, nowhere near cancelGracePeriod.
func removeWithRetry(path string) {
	for attempt := 0; attempt < 3; attempt++ {
		err := os.Remove(path)
		if err == nil || os.IsNotExist(err) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}
