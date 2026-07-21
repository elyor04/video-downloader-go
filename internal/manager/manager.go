// Package manager ports download_manager.py's DownloadManager: it owns the
// job list, the add-job options, the live URL preview, the concurrency
// scheduler, and the serialized playlist/login/password prompt queue, and
// emits events for the frontend to render (replacing Qt Signals).
package manager

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"video-downloader-go/internal/job"
	"video-downloader-go/internal/settings"
	"video-downloader-go/internal/utils"
)

type jobRuntime struct {
	cancel   context.CancelFunc
	answerCh chan credentialAnswer // buffered 1; used by login/password/skip submission
}

type credentialAnswer struct {
	username string
	password string
	ok       bool // false = explicit skip (or cancelled while waiting)
}

type promptRequest struct {
	jobID string
	kind  string // "playlist" | "login" | "password"
}

type Manager struct {
	mu sync.Mutex

	ytdlpPath   string
	emit        func(event string, data ...interface{})
	browseDirFn func() (string, error)

	settingsData settings.Settings

	jobs     []*job.Job
	runtimes map[string]*jobRuntime
	active   map[string]struct{}
	fetching map[string]struct{}

	pendingPrompts     []promptRequest
	currentPromptJobID string

	mode       string
	resolution int
	convertTo  string
	fileName   string
	outputDir  string
	language   string

	previewURL           string
	previewState         string // idle | fetching | ready | error
	previewTitle         string
	previewThumbnail     string
	previewIsPlaylist    bool
	previewPlaylistCount int
	previewMaxHeight     int
	previewError         string
	previewCancel        context.CancelFunc

	windowFocused bool
}

func New(ytdlpPath string) *Manager {
	s := settings.Load()
	outputDir := s.OutputDir
	if outputDir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			outputDir = filepath.Join(home, "Downloads")
		}
	}
	language := s.Language
	if language == "" {
		language = "en"
	}

	return &Manager{
		ytdlpPath:     ytdlpPath,
		emit:          func(string, ...interface{}) {},
		settingsData:  s,
		runtimes:      make(map[string]*jobRuntime),
		active:        make(map[string]struct{}),
		fetching:      make(map[string]struct{}),
		mode:          string(job.ModeVideo),
		resolution:    utils.MaxResolution,
		convertTo:     "original",
		outputDir:     outputDir,
		language:      language,
		previewState:  "idle",
		windowFocused: true,
	}
}

func (m *Manager) SetEmitter(fn func(event string, data ...interface{})) { m.emit = fn }
func (m *Manager) SetBrowseDirFunc(fn func() (string, error))            { m.browseDirFn = fn }

// findJob assumes m.mu is held by the caller.
func (m *Manager) findJob(id string) *job.Job {
	for _, j := range m.jobs {
		if j.ID == id {
			return j
		}
	}
	return nil
}

// -- Add-job options --

type optionsState struct {
	Mode           string   `json:"mode"`
	Resolution     int      `json:"resolution"`
	ConvertTo      string   `json:"convertTo"`
	ConvertOptions []string `json:"convertOptions"`
}

func (m *Manager) convertOptionsLocked() []string {
	if m.mode == string(job.ModeAudio) {
		return utils.AudioConvertOptions
	}
	return utils.VideoConvertOptions
}

func (m *Manager) optionsSnapshotLocked() optionsState {
	return optionsState{
		Mode:           m.mode,
		Resolution:     m.resolution,
		ConvertTo:      m.convertTo,
		ConvertOptions: m.convertOptionsLocked(),
	}
}

func (m *Manager) SetMode(mode string) {
	m.mu.Lock()
	if mode == m.mode {
		m.mu.Unlock()
		return
	}
	m.mode = mode
	state := m.optionsSnapshotLocked()
	m.mu.Unlock()
	m.emit("options:changed", state)
}

func (m *Manager) SetResolution(value int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, opt := range utils.ResolutionLadder {
		if opt.Value == value {
			m.resolution = value
			return
		}
	}
}

func (m *Manager) ResetResolutionToBest() {
	m.mu.Lock()
	m.resolution = utils.MaxResolution
	m.mu.Unlock()
}

func (m *Manager) SetConvertTo(value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, opt := range m.convertOptionsLocked() {
		if opt == value {
			m.convertTo = value
			return
		}
	}
}

func (m *Manager) ResetConvertToOriginal() {
	m.mu.Lock()
	m.convertTo = "original"
	m.mu.Unlock()
}

func (m *Manager) SetOutputDir(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	m.mu.Lock()
	if path == m.outputDir {
		m.mu.Unlock()
		return
	}
	m.outputDir = path
	m.settingsData.OutputDir = path
	s := m.settingsData
	m.mu.Unlock()

	_ = s.Save()
	m.emit("output-dir:changed", path)
}

// BrowseOutputDir opens the native folder picker (replaces QML's
// Qt.labs.platform FolderDialog) and, if the user picked one, persists and
// broadcasts it exactly like SetOutputDir.
func (m *Manager) BrowseOutputDir() string {
	if m.browseDirFn == nil {
		return ""
	}
	dir, err := m.browseDirFn()
	if err != nil || dir == "" {
		return ""
	}
	m.SetOutputDir(dir)
	return dir
}

func (m *Manager) SetFileName(value string) {
	m.mu.Lock()
	m.fileName = strings.TrimSpace(value)
	m.mu.Unlock()
}

var supportedLanguages = map[string]bool{"en": true, "ru": true, "uz": true}

func (m *Manager) SetLanguage(code string) {
	if !supportedLanguages[code] {
		return
	}
	m.mu.Lock()
	if code == m.language {
		m.mu.Unlock()
		return
	}
	m.language = code
	m.settingsData.Language = code
	s := m.settingsData
	m.mu.Unlock()

	_ = s.Save()
	m.emit("language:changed", code)
}

// -- Startup snapshot --

type InitialState struct {
	Mode              string                   `json:"mode"`
	Resolution        int                      `json:"resolution"`
	ConvertTo         string                   `json:"convertTo"`
	OutputDir         string                   `json:"outputDir"`
	FileName          string                   `json:"fileName"`
	Language          string                   `json:"language"`
	ResolutionOptions []utils.ResolutionOption `json:"resolutionOptions"`
	ConvertOptions    []string                 `json:"convertOptions"`
	Jobs              []job.DTO                `json:"jobs"`
}

func (m *Manager) GetInitialState() InitialState {
	m.mu.Lock()
	defer m.mu.Unlock()
	jobs := make([]job.DTO, len(m.jobs))
	for i, j := range m.jobs {
		jobs[i] = j.ToDTO()
	}
	return InitialState{
		Mode:              m.mode,
		Resolution:        m.resolution,
		ConvertTo:         m.convertTo,
		OutputDir:         m.outputDir,
		FileName:          m.fileName,
		Language:          m.language,
		ResolutionOptions: utils.ResolutionLadder,
		ConvertOptions:    m.convertOptionsLocked(),
		Jobs:              jobs,
	}
}

// SetWindowFocused lets the frontend report focus/blur so notifications only
// fire while the user isn't already looking at the app (mirrors
// _notify_if_unfocused's applicationState() check).
func (m *Manager) SetWindowFocused(focused bool) {
	m.mu.Lock()
	m.windowFocused = focused
	m.mu.Unlock()
}

// Shutdown mirrors requestShutdown: cancel every in-flight job so its
// process tree is killed immediately, then give the OS a brief moment to
// finish tearing them down before Wails exits the app.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(m.runtimes))
	for _, rt := range m.runtimes {
		if rt.cancel != nil {
			cancels = append(cancels, rt.cancel)
		}
	}
	m.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

func (m *Manager) logPanic(context string, r interface{}) {
	log.Printf("recovered panic in %s: %v", context, r)
}
