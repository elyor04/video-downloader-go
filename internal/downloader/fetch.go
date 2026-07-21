package downloader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// FetchResult mirrors the payload run_fetch() put on the event queue in
// worker_process.py.
type FetchResult struct {
	Title         string
	Thumbnail     string
	IsPlaylist    bool
	PlaylistCount int // 0 = unknown
	MaxHeight     int // 0 = unknown / not applicable (playlists)
}

// rawInfo covers just the fields of yt-dlp's -J JSON dump that Fetch needs.
type rawInfo struct {
	Type          string        `json:"_type"`
	Title         string        `json:"title"`
	ID            string        `json:"id"`
	Thumbnail     string        `json:"thumbnail"`
	PlaylistCount int           `json:"playlist_count"`
	Entries       []interface{} `json:"entries"`
	Formats       []struct {
		Height int `json:"height"`
	} `json:"formats"`
}

// Fetch mirrors run_fetch(): a flat, non-recursive metadata lookup used both
// for the live URL preview and for addJob's fetch-before-queue step.
func Fetch(ctx context.Context, ytdlpPath, url string) (*FetchResult, error) {
	cmd := exec.CommandContext(ctx, ytdlpPath,
		"--no-warnings",
		"--socket-timeout", "30",
		"--flat-playlist",
		"-J",
		url,
	)
	setProcAttrs(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if cmd.Process == nil {
			// The process never started (e.g. yt-dlp isn't on PATH or the
			// bundled binary is missing) -- stderr is necessarily empty, so
			// fall back to Go's own exec error instead of the misleading
			// "no output" message.
			return nil, err
		}
		return nil, fmt.Errorf("%s", lastErrorLine(stderr.String()))
	}

	var info rawInfo
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		return nil, fmt.Errorf("couldn't parse video info: %w", err)
	}

	isPlaylist := info.Type == "playlist" || info.Entries != nil
	title := info.Title
	if title == "" {
		title = info.ID
	}
	if title == "" {
		title = url
	}

	result := &FetchResult{
		Title:      title,
		Thumbnail:  info.Thumbnail,
		IsPlaylist: isPlaylist,
	}
	if isPlaylist {
		if info.Entries != nil {
			result.PlaylistCount = len(info.Entries)
		} else {
			result.PlaylistCount = info.PlaylistCount
		}
	} else {
		maxHeight := 0
		for _, f := range info.Formats {
			if f.Height > maxHeight {
				maxHeight = f.Height
			}
		}
		result.MaxHeight = maxHeight
	}
	return result, nil
}

// lastErrorLine extracts the most relevant line from yt-dlp's stderr for
// display to the user, mirroring how Python's str(exception) from yt-dlp's
// own DownloadError already comes back as a single clean message.
func lastErrorLine(stderr string) string {
	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, "ERROR: ")
		return line
	}
	return "yt-dlp failed with no output"
}
