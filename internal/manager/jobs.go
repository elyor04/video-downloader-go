package manager

import (
	"context"
	"errors"
	"strings"
	"time"

	"video-downloader-go/internal/downloader"
	"video-downloader-go/internal/job"
	"video-downloader-go/internal/utils"
)

// -- URL preview (mirrors requestPreview/_drain_preview/_clear_preview) --
// Debouncing while the user types happens on the frontend (matching QML's
// own Timer{interval:600}), so RequestPreview here is called already
// debounced.

type previewState struct {
	URL           string `json:"url"`
	State         string `json:"state"`
	Title         string `json:"title"`
	Thumbnail     string `json:"thumbnail"`
	IsPlaylist    bool   `json:"isPlaylist"`
	PlaylistCount int    `json:"playlistCount"`
	Error         string `json:"error"`
}

func (m *Manager) previewSnapshotLocked() previewState {
	return previewState{
		URL:           m.previewURL,
		State:         m.previewState,
		Title:         m.previewTitle,
		Thumbnail:     m.previewThumbnail,
		IsPlaylist:    m.previewIsPlaylist,
		PlaylistCount: m.previewPlaylistCount,
		Error:         m.previewError,
	}
}

func (m *Manager) resolutionOptionsLocked() []utils.ResolutionOption {
	if m.previewMaxHeight == 0 {
		return utils.ResolutionLadder
	}
	filtered := make([]utils.ResolutionOption, 0, len(utils.ResolutionLadder))
	for _, opt := range utils.ResolutionLadder {
		if opt.Value == utils.MaxResolution || opt.Value <= m.previewMaxHeight {
			filtered = append(filtered, opt)
		}
	}
	if len(filtered) == 0 {
		return utils.ResolutionLadder
	}
	return filtered
}

func (m *Manager) emitResolutionOptions() {
	m.mu.Lock()
	opts := m.resolutionOptionsLocked()
	m.mu.Unlock()
	m.emit("resolution-options:changed", opts)
}

func (m *Manager) RequestPreview(url string) {
	url = strings.TrimSpace(url)

	m.mu.Lock()
	if url == m.previewURL && (m.previewState == "fetching" || m.previewState == "ready") {
		m.mu.Unlock()
		return
	}
	if m.previewCancel != nil {
		m.previewCancel()
	}
	m.previewURL = url
	hadMaxHeight := m.previewMaxHeight != 0
	m.previewMaxHeight = 0

	if url == "" {
		m.previewState = "idle"
		m.previewCancel = nil
		state := m.previewSnapshotLocked()
		m.mu.Unlock()
		m.emit("preview:changed", state)
		if hadMaxHeight {
			m.emitResolutionOptions()
		}
		return
	}

	m.previewState = "fetching"
	m.previewTitle = ""
	m.previewThumbnail = ""
	m.previewIsPlaylist = false
	m.previewPlaylistCount = 0
	m.previewError = ""
	state := m.previewSnapshotLocked()

	ctx, cancel := context.WithCancel(context.Background())
	m.previewCancel = cancel
	ytdlpPath := m.ytdlpPath
	m.mu.Unlock()

	m.emit("preview:changed", state)
	if hadMaxHeight {
		m.emitResolutionOptions()
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.logPanic("preview fetch", r)
			}
		}()
		result, err := downloader.Fetch(ctx, ytdlpPath, url)
		m.finishPreview(url, result, err)
	}()
}

func (m *Manager) finishPreview(forURL string, result *downloader.FetchResult, err error) {
	m.mu.Lock()
	if m.previewURL != forURL {
		// A newer preview request (or an explicit clear) has already
		// superseded this one; the stale result is discarded.
		m.mu.Unlock()
		return
	}
	if err != nil {
		m.previewCancel = nil
		if errors.Is(err, context.Canceled) {
			m.mu.Unlock()
			return
		}
		m.previewError = err.Error()
		m.previewState = "error"
		state := m.previewSnapshotLocked()
		m.mu.Unlock()
		m.emit("preview:changed", state)
		m.emit("error:occurred", err.Error())
		return
	}

	m.previewTitle = result.Title
	m.previewThumbnail = result.Thumbnail
	m.previewIsPlaylist = result.IsPlaylist
	m.previewPlaylistCount = result.PlaylistCount
	m.previewMaxHeight = result.MaxHeight
	m.previewState = "ready"
	m.previewCancel = nil
	state := m.previewSnapshotLocked()
	m.mu.Unlock()

	m.emit("preview:changed", state)
	m.emitResolutionOptions()
}

func (m *Manager) clearPreview() {
	m.mu.Lock()
	if m.previewCancel != nil {
		m.previewCancel()
		m.previewCancel = nil
	}
	hadMaxHeight := m.previewMaxHeight != 0
	m.previewURL = ""
	m.previewState = "idle"
	m.previewTitle = ""
	m.previewThumbnail = ""
	m.previewIsPlaylist = false
	m.previewPlaylistCount = 0
	m.previewMaxHeight = 0
	m.previewError = ""
	state := m.previewSnapshotLocked()
	m.mu.Unlock()

	m.emit("preview:changed", state)
	if hadMaxHeight {
		m.emitResolutionOptions()
	}
}

// -- Adding jobs (mirrors addJob/_start_fetch) --

func (m *Manager) AddJob(url string) {
	url = strings.TrimSpace(url)
	if url == "" {
		// "error." prefix marks this as an i18n key rather than final
		// display text -- see ErrorDialog.tsx, which is the single place
		// that distinguishes the two and runs keys through t().
		m.emit("error:occurred", "error.emptyUrl")
		return
	}

	m.mu.Lock()
	outputDir := m.outputDir
	m.mu.Unlock()
	if key := utils.CheckDownloadDir(outputDir, true); key != "" {
		m.emit("error:occurred", key)
		return
	}

	m.mu.Lock()
	fname := m.fileName
	if fname == "" {
		fname = "%(title)s"
	}
	// A custom filename is consumed by exactly one job -- otherwise it
	// would silently reapply to every subsequent job added afterward,
	// overwriting/colliding with this one's output.
	m.fileName = ""
	j := job.New(url, job.Mode(m.mode), m.resolution, m.convertTo, outputDir, fname)

	reuse := url == m.previewURL && m.previewState == "ready"
	var previewTitle, previewThumb string
	var previewIsPlaylist bool
	var previewCount int
	if reuse {
		previewTitle = m.previewTitle
		previewThumb = m.previewThumbnail
		previewIsPlaylist = m.previewIsPlaylist
		previewCount = m.previewPlaylistCount
	}
	m.jobs = append(m.jobs, j)

	if reuse {
		j.Title = previewTitle
		j.Thumbnail = previewThumb
		j.IsPlaylist = previewIsPlaylist
		j.PlaylistCount = previewCount
		if j.IsPlaylist {
			j.SetState(job.StateAwaitingPlaylistConfirm)
		} else {
			j.SetState(job.StateQueued)
		}
	}
	dto := j.ToDTO()
	m.mu.Unlock()

	m.emit("job:added", dto)

	if reuse {
		if j.IsPlaylist {
			m.requestPrompt(j.ID, "playlist")
		} else {
			m.scheduleNext()
		}
	} else {
		m.startFetch(j)
	}

	m.clearPreview()
}

func (m *Manager) startFetch(j *job.Job) {
	// CancelCauseFunc for consistency with startDownload's jobRuntime (same
	// field, same type) -- Fetch itself never inspects the cause, since its
	// exec.CommandContext-driven cancellation is already an immediate kill
	// of a single short-lived metadata lookup, graceful or not.
	ctx, cancel := context.WithCancelCause(context.Background())
	m.mu.Lock()
	if _, already := m.fetching[j.ID]; already {
		// No current caller re-enters startFetch for a job already being
		// fetched (AddJob only ever calls it once, right after creating a
		// fresh job ID) -- this guards against a second yt-dlp -J process
		// racing the first one and finishFetch firing twice for one job,
		// should a future caller ever do so.
		m.mu.Unlock()
		cancel(nil)
		return
	}
	m.runtimes[j.ID] = &jobRuntime{cancel: cancel}
	m.fetching[j.ID] = struct{}{}
	ytdlpPath := m.ytdlpPath
	m.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.logPanic("fetch "+j.ID, r)
			}
		}()
		result, err := downloader.Fetch(ctx, ytdlpPath, j.URL)
		m.finishFetch(j.ID, result, err)
	}()
}

func (m *Manager) finishFetch(jobID string, result *downloader.FetchResult, err error) {
	m.mu.Lock()
	j := m.findJob(jobID)
	delete(m.fetching, jobID)
	delete(m.runtimes, jobID)
	if j == nil {
		m.mu.Unlock()
		return
	}

	if err != nil {
		if errors.Is(err, context.Canceled) {
			j.SetState(job.StateCancelled)
		} else {
			j.ErrorMessage = err.Error()
			j.SetState(job.StateError)
		}
		dto := j.ToDTO()
		m.mu.Unlock()
		m.emit("job:updated", dto)
		return
	}

	j.Title = result.Title
	j.Thumbnail = result.Thumbnail
	j.IsPlaylist = result.IsPlaylist
	j.PlaylistCount = result.PlaylistCount
	if j.IsPlaylist {
		j.SetState(job.StateAwaitingPlaylistConfirm)
	} else {
		j.SetState(job.StateQueued)
	}
	dto := j.ToDTO()
	m.mu.Unlock()

	m.emit("job:updated", dto)
	if j.IsPlaylist {
		m.requestPrompt(jobID, "playlist")
	} else {
		m.scheduleNext()
	}
}

// -- Concurrency scheduler (mirrors _schedule_next) --

func (m *Manager) scheduleNext() {
	for {
		m.mu.Lock()
		if len(m.active) >= utils.MaxConcurrentDownloads {
			m.mu.Unlock()
			return
		}
		var next *job.Job
		for _, j := range m.jobs {
			if j.State == job.StateQueued {
				next = j
				break
			}
		}
		m.mu.Unlock()
		if next == nil {
			return
		}
		m.startDownload(next)
	}
}

func applyProgress(j *job.Job, ev downloader.ProgressEvent) {
	j.DownloadedBytes = ev.DownloadedBytes
	j.TotalBytes = ev.TotalBytes
	j.Speed = ev.Speed
	j.ETA = ev.ETA
	if ev.PlaylistIndex > 0 {
		j.PlaylistIndex = ev.PlaylistIndex
	}
	if ev.NEntries > 0 {
		j.PlaylistCount = ev.NEntries
	}
	if ev.Status == "finished" || j.TotalBytes <= 0 || j.DownloadedBytes < 0 {
		j.Progress = -1
	} else {
		j.Progress = float64(j.DownloadedBytes) / float64(j.TotalBytes)
	}
	j.Stage = "downloading"
}

// progressEmitInterval throttles job:updated events during a download to
// roughly the same rate the Python UI updated at (its 80ms QTimer poll only
// ever applied the *last* progress event queued since the previous tick).
const progressEmitInterval = 80 * time.Millisecond

func (m *Manager) startDownload(j *job.Job) {
	// CancelCauseFunc: CancelJob/CancelAll cancel with a nil cause (a plain
	// user cancel, downloader.runAttempt's graceful cancelTree path), while
	// Manager.Shutdown cancels with downloader.ErrShutdown (skip straight
	// to killTree) -- see the comment on Shutdown.
	ctx, cancel := context.WithCancelCause(context.Background())
	m.mu.Lock()
	m.runtimes[j.ID] = &jobRuntime{cancel: cancel, answerCh: make(chan credentialAnswer, 1)}
	m.active[j.ID] = struct{}{}
	j.SetState(job.StateDownloading)
	dto := j.ToDTO()
	ytdlpPath := m.ytdlpPath
	m.mu.Unlock()
	m.emit("job:updated", dto)

	params := downloader.Params{
		URL:              j.URL,
		Mode:             string(j.Mode),
		Resolution:       j.Resolution,
		ConvertTo:        j.ConvertTo,
		OutputDir:        j.OutputDir,
		FileName:         j.FileName,
		IsPlaylist:       j.IsPlaylist,
		DownloadPlaylist: j.DownloadPlaylist,
	}

	// onProgress runs synchronously on the downloader's single stdout-
	// scanning goroutine (one line at a time), so lastEmit needs no lock of
	// its own.
	var lastEmit time.Time
	onProgress := func(ev downloader.ProgressEvent) {
		now := time.Now()
		throttled := ev.Status != "finished" && now.Sub(lastEmit) < progressEmitInterval

		m.mu.Lock()
		jj := m.findJob(j.ID)
		if jj == nil || jj.State != job.StateDownloading {
			m.mu.Unlock()
			return
		}
		applyProgress(jj, ev)
		dto := jj.ToDTO()
		m.mu.Unlock()

		if !throttled {
			lastEmit = now
			m.emit("job:updated", dto)
		}
	}

	onStage := func(stage string) {
		m.mu.Lock()
		jj := m.findJob(j.ID)
		if jj == nil {
			m.mu.Unlock()
			return
		}
		jj.Stage = stage
		dto := jj.ToDTO()
		m.mu.Unlock()
		m.emit("job:updated", dto)
	}

	cb := downloader.Callbacks{
		OnProgress:      onProgress,
		OnStage:         onStage,
		RequestLogin:    m.requestLoginCallback(j.ID),
		RequestPassword: m.requestPasswordCallback(j.ID),
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.logPanic("download "+j.ID, r)
			}
		}()
		finishedDir, err := downloader.Download(ctx, ytdlpPath, params, cb)
		m.finishDownload(j.ID, finishedDir, err)
	}()
}

func (m *Manager) finishDownload(jobID, finishedDir string, err error) {
	m.mu.Lock()
	j := m.findJob(jobID)
	delete(m.active, jobID)
	delete(m.runtimes, jobID)
	if j == nil {
		m.mu.Unlock()
		return
	}

	shouldNotify := false
	switch {
	case err == nil:
		j.FinishedDir = finishedDir
		j.Progress = 1.0
		j.SetState(job.StateSuccess)
		shouldNotify = true
	case errors.Is(err, downloader.ErrCancelled):
		j.SetState(job.StateCancelled)
	default:
		j.ErrorMessage = err.Error()
		j.SetState(job.StateError)
		shouldNotify = true
	}
	dto := j.ToDTO()
	title := dto.Title
	success := j.State == job.StateSuccess
	m.mu.Unlock()

	m.emit("job:updated", dto)
	if shouldNotify {
		m.maybeNotify(success, title)
	}
	m.scheduleNext()
}

// -- Cancel / remove (mirrors cancelJob/removeJob/cancelAll/clearCompleted) --

func (m *Manager) CancelJob(jobID string) {
	m.mu.Lock()
	j := m.findJob(jobID)
	if j == nil || job.TerminalStates[j.State] {
		m.mu.Unlock()
		return
	}
	rt := m.runtimes[jobID]
	m.mu.Unlock()

	m.dismissPromptsFor(jobID)

	if rt != nil {
		// A process is running (fetch or download); cancelling ctx (with a
		// nil cause, meaning "ordinary user cancel" as opposed to
		// Manager.Shutdown's downloader.ErrShutdown) makes
		// downloader.Fetch/Download react inside downloader.runAttempt,
		// which reports back through finishFetch/finishDownload. Fetch's
		// own exec.CommandContext cancellation is always an immediate kill
		// (a short-lived metadata lookup, nothing to gracefully wind down);
		// Download's runAttempt asks yt-dlp/ffmpeg to stop nicely first
		// (cancelTree) and only escalates to a hard process-tree kill
		// (killTree) if they don't exit within a few seconds.
		rt.cancel(nil)
		return
	}

	// No runtime handle yet: job is still queued or awaiting playlist
	// confirmation, so there's nothing to kill — resolve immediately.
	m.mu.Lock()
	j = m.findJob(jobID)
	if j == nil || job.TerminalStates[j.State] {
		m.mu.Unlock()
		return
	}
	j.SetState(job.StateCancelled)
	dto := j.ToDTO()
	m.mu.Unlock()

	m.emit("job:updated", dto)
	m.scheduleNext()
}

func (m *Manager) removeJobLocked(jobID string) {
	for i, j := range m.jobs {
		if j.ID == jobID {
			m.jobs = append(m.jobs[:i], m.jobs[i+1:]...)
			return
		}
	}
}

func (m *Manager) RemoveJob(jobID string) {
	m.mu.Lock()
	j := m.findJob(jobID)
	if j == nil || !job.TerminalStates[j.State] {
		m.mu.Unlock()
		return
	}
	m.removeJobLocked(jobID)
	m.mu.Unlock()
	m.emit("job:removed", jobID)
}

func (m *Manager) CancelAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.jobs))
	for _, j := range m.jobs {
		if job.ActiveStates[j.State] {
			ids = append(ids, j.ID)
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.CancelJob(id)
	}
}

func (m *Manager) ClearCompleted() {
	m.mu.Lock()
	var removedIDs []string
	kept := m.jobs[:0:0]
	for _, j := range m.jobs {
		if job.TerminalStates[j.State] {
			removedIDs = append(removedIDs, j.ID)
		} else {
			kept = append(kept, j)
		}
	}
	m.jobs = kept
	m.mu.Unlock()
	for _, id := range removedIDs {
		m.emit("job:removed", id)
	}
}

func (m *Manager) OpenOutputFolder(jobID string) {
	m.mu.Lock()
	j := m.findJob(jobID)
	var dir string
	ok := j != nil && j.State == job.StateSuccess
	if ok {
		dir = j.FinishedDir
		if dir == "" {
			dir = j.OutputDir
		}
	}
	m.mu.Unlock()
	if ok {
		_ = utils.OpenInFileManager(dir)
	}
}
