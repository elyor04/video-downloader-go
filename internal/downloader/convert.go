package downloader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"video-downloader-go/internal/procutil"
)

// videoContainerSpec is the software fallback ffmpeg is told to use when a
// video source's codec isn't already compatible with the target container.
// Verified against the bundled ffmpeg (n8.1.2): h264/aac, vp9/opus and
// av1/opus sources all remux cleanly (a plain stream copy, no re-encode)
// into mp4/mkv, and vp9/av1 sources remux cleanly into webm too -- so in
// practice this only ever runs for the one combination that doesn't: an
// h264-sourced video converted to webm (webm requires VP8/VP9/AV1 video and
// Vorbis/Opus audio). "veryfast"/"-cpu-used 4" are chosen deliberately over
// each encoder's slower, quality-favoring defaults -- libvpx-vp9 in
// particular is well known to be very slow at its default speed (confirmed
// with a timed comparison: -cpu-used 0 took ~2x as long as -cpu-used 4 on a
// small test clip), and speed is the entire point of this fallback.
type videoContainerSpec struct {
	videoCodec string
	videoArgs  []string
	audioCodec string
	audioArgs  []string
}

var videoContainerSpecs = map[string]videoContainerSpec{
	"mp4": {
		videoCodec: "libx264",
		videoArgs:  []string{"-preset", "veryfast", "-crf", "23"},
		audioCodec: "aac",
		audioArgs:  []string{"-b:a", "192k"},
	},
	"mkv": {
		videoCodec: "libx264",
		videoArgs:  []string{"-preset", "veryfast", "-crf", "23"},
		audioCodec: "aac",
		audioArgs:  []string{"-b:a", "192k"},
	},
	"webm": {
		videoCodec: "libvpx-vp9",
		videoArgs:  []string{"-crf", "30", "-b:v", "0", "-cpu-used", "4", "-row-mt", "1"},
		audioCodec: "libopus",
		audioArgs:  []string{"-b:a", "160k"},
	},
}

// remuxArgs/encodeArgs build the two ffmpeg invocations convertVideo tries
// in order. Both map video/audio explicitly (0:v:0 required, 0:a:0?
// optional) rather than a blanket "-map 0", so an unexpected extra stream
// (e.g. an attached-picture cover-art stream some extractors embed) can't
// silently ride along into either the copy or the encode -- verified
// against both a video+audio and a video-only synthetic source that the
// "?" correctly makes the missing-audio case a non-error.
func remuxArgs(src, dest string) []string {
	return []string{
		"-hide_banner", "-y", "-i", src,
		"-map", "0:v:0", "-map", "0:a:0?",
		"-c", "copy",
		dest,
	}
}

func encodeArgs(spec videoContainerSpec, src, dest string) []string {
	args := []string{
		"-hide_banner", "-y", "-i", src,
		"-map", "0:v:0", "-map", "0:a:0?",
		"-c:v", spec.videoCodec,
	}
	args = append(args, spec.videoArgs...)
	args = append(args, "-c:a", spec.audioCodec)
	args = append(args, spec.audioArgs...)
	return append(args, dest)
}

// runFFmpeg runs one ffmpeg invocation to completion, honoring ctx the same
// way runAttempt does for yt-dlp: a graceful cancelTree first, escalating
// to killTree if it doesn't exit within cancelGracePeriod (see
// process_windows.go/process_unix.go, both already generic over any
// *exec.Cmd/pid, not yt-dlp-specific). Mirrors runAttempt's own handling of
// a process that finishes successfully right as the cancel signal lands --
// trusting that success rather than discarding a completed conversion just
// because of unlucky timing. Returns ErrCancelled on a real cancellation,
// or an error built from ffmpeg's own last stderr line on a genuine
// failure.
func runFFmpeg(ctx context.Context, ffmpegPath string, args []string) error {
	cmd := exec.Command(ffmpegPath, args...)
	procutil.SetProcAttrs(cmd)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	waitErr := make(chan error, 1)
	go func() { waitErr <- cmd.Wait() }()

	select {
	case <-ctx.Done():
		cancelTree(cmd, waitErr, cancelGracePeriod)
		if cmd.ProcessState != nil && cmd.ProcessState.Success() {
			return nil
		}
		return ErrCancelled
	case err := <-waitErr:
		if err != nil {
			return fmt.Errorf("%s", lastErrorLine(stderr.String()))
		}
		return nil
	}
}

// convertVideo runs the remux-then-encode cascade for one downloaded item:
// a fast lossless stream copy first, falling back to a software re-encode
// (per videoContainerSpecs) only when the copy fails. Deletes srcPath on
// success, mirroring yt-dlp's own default --no-keep-video behavior;
// deletions are best-effort (removeWithRetry, same as cleanup.go) since a
// leftover raw file next to a successfully converted one is clutter, not a
// job failure. Callers are expected to have already confirmed srcPath
// exists and destPath's extension actually differs from it (see
// convertDownloaded).
func convertVideo(ctx context.Context, ffmpegPath, srcPath, destPath, convertTo string) error {
	err := runFFmpeg(ctx, ffmpegPath, remuxArgs(srcPath, destPath))
	switch {
	case err == nil:
		removeWithRetry(srcPath)
		return nil
	case errors.Is(err, ErrCancelled):
		removeWithRetry(destPath)
		return ErrCancelled
	}
	removeWithRetry(destPath) // discard the failed remux's partial/empty output, if any

	spec, ok := videoContainerSpecs[convertTo]
	if !ok {
		return fmt.Errorf("no conversion recipe for %q", convertTo)
	}
	if err := runFFmpeg(ctx, ffmpegPath, encodeArgs(spec, srcPath, destPath)); err != nil {
		removeWithRetry(destPath)
		if errors.Is(err, ErrCancelled) {
			return ErrCancelled
		}
		return fmt.Errorf("converting to %s: %w", convertTo, err)
	}
	removeWithRetry(srcPath)
	return nil
}

// convertDownloaded runs convertVideo for every item runAttempt's yt-dlp
// invocation reported to destFilePath (one line per playlist item, or a
// single line for a non-playlist job -- see readAllDestBaseAndExt), and is
// itself what fires the "converting" stage now that yt-dlp is no longer
// asked to recode. Two cases are skipped rather than treated as errors:
// destPath already matching srcPath's extension (nothing to convert), and a
// source file that doesn't actually exist -- "before_dl" fires before each
// item's download attempt regardless of whether that attempt then
// succeeds, and yt-dlp's default --no-abort-on-error means one playlist
// item failing doesn't stop the rest, so destFilePath can contain a line
// for an item that was never actually written to disk.
func convertDownloaded(ctx context.Context, ffmpegPath, destFilePath, convertTo string, cb Callbacks) error {
	refs, ok := readAllDestBaseAndExt(destFilePath)
	if !ok {
		return fmt.Errorf("could not read destination list %s", destFilePath)
	}

	if cb.OnStage != nil {
		cb.OnStage("converting")
	}

	for _, ref := range refs {
		if err := ctx.Err(); err != nil {
			return ErrCancelled
		}
		if ref.Ext == convertTo {
			continue
		}
		srcPath := ref.Base + "." + ref.Ext
		if _, err := os.Stat(srcPath); err != nil {
			continue // item never finished downloading; nothing to convert
		}
		destPath := ref.Base + "." + convertTo
		if err := convertVideo(ctx, ffmpegPath, srcPath, destPath, convertTo); err != nil {
			return err
		}
	}
	return nil
}
