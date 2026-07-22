# Video Downloader

Download videos and audio from YouTube, Instagram, TikTok, and other sites — with a queue for multiple downloads at once, resolution and format selection, and a Material-styled desktop interface. A Go/[Wails](https://wails.io) rewrite of [the original PyQt/QML version](https://github.com/elyor04/video-downloader).

## Features

- Paste a URL and it's automatically previewed (title and thumbnail) before you add it — no separate "Fetch" step; the Download button enables once the preview succeeds, and any failure (bad link, network issue, site blocking the request) shows up as a clear error dialog
- Add several URLs and download up to 2 at a time; the rest wait in a queue and start automatically
- Video or audio mode, resolution selection (narrowed to what's actually available once a video is previewed), optional format conversion (mp4/mkv/webm/mp3/m4a/wav)
- Playlist detection with a confirmation prompt before downloading an entire playlist
- Sign-in / video-password support for gated content
- Each download runs in its own isolated process, so a failure in one never affects the app or other downloads, and cancelling is immediate
- Native OS notification when a download finishes while the window isn't focused; output folder and language are remembered across restarts
- Interface available in English, Russian, and Uzbek (switchable at any time, top-right)

## Installation

- [Go](https://go.dev) 1.23+
- [Node.js](https://nodejs.org) 18+
- The [Wails CLI](https://wails.io/docs/gettingstarted/installation):
  ```
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```

### yt-dlp & FFmpeg

Unlike the original, these aren't installed manually — `wails dev`/`wails build` fetch the latest yt-dlp and FFmpeg binaries into `bin/` automatically (see `tools/fetchytdlp`, `tools/fetchffmpeg`). The running app also checks for newer versions on every startup and offers to update them in place.

## Usage

```
wails dev
```

To build a redistributable instead of running in development mode:

```
wails build
```

To build the Windows NSIS installer, pass `-installscope user`:

```
wails build -nsis -installscope user
```

This installs per-user (no admin/UAC) into `%LOCALAPPDATA%` instead of the default `Program Files`, which the running app can't write to without elevation — required so the app's own auto-updater can overwrite `bin/yt-dlp.exe`/`ffmpeg.exe`/`ffprobe.exe` in place (see `build/windows/installer/project.nsi`).
