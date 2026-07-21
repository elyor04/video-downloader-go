package manager

import (
	"context"

	"video-downloader-go/internal/job"
)

// -- Modal prompt serialization (mirrors _request_prompt/_maybe_show_next_prompt) --
// Only one of the playlist-confirm/login/password dialogs is ever shown at
// a time; the rest queue up and are shown in order as each is resolved.

func (m *Manager) requestPrompt(jobID, kind string) {
	m.mu.Lock()
	m.pendingPrompts = append(m.pendingPrompts, promptRequest{jobID: jobID, kind: kind})
	m.mu.Unlock()
	m.maybeShowNextPrompt()
}

func (m *Manager) maybeShowNextPrompt() {
	m.mu.Lock()
	if m.currentPromptJobID != "" || len(m.pendingPrompts) == 0 {
		m.mu.Unlock()
		return
	}
	req := m.pendingPrompts[0]
	m.pendingPrompts = m.pendingPrompts[1:]
	j := m.findJob(req.jobID)
	if j == nil {
		m.mu.Unlock()
		m.maybeShowNextPrompt()
		return
	}
	m.currentPromptJobID = req.jobID
	kind := req.kind
	jobID := j.ID
	url := j.URL
	playlistCount := j.PlaylistCount
	m.mu.Unlock()

	switch kind {
	case "playlist":
		m.emit("playlist:detected", map[string]interface{}{"jobId": jobID, "count": playlistCount})
	case "login":
		m.emit("login:requested", map[string]interface{}{"jobId": jobID, "url": url})
	case "password":
		m.emit("password:requested", map[string]interface{}{"jobId": jobID, "url": url})
	}
}

func (m *Manager) dismissPromptsFor(jobID string) {
	m.mu.Lock()
	filtered := m.pendingPrompts[:0:0]
	for _, p := range m.pendingPrompts {
		if p.jobID != jobID {
			filtered = append(filtered, p)
		}
	}
	m.pendingPrompts = filtered
	wasCurrent := m.currentPromptJobID == jobID
	if wasCurrent {
		m.currentPromptJobID = ""
	}
	m.mu.Unlock()

	if wasCurrent {
		m.emit("prompt:cancelled", jobID)
		m.maybeShowNextPrompt()
	}
}

func (m *Manager) clearCurrentPrompt(jobID string) {
	m.mu.Lock()
	if m.currentPromptJobID == jobID {
		m.currentPromptJobID = ""
	}
	m.mu.Unlock()
	m.maybeShowNextPrompt()
}

// -- Playlist confirm --

func (m *Manager) ConfirmPlaylist(jobID string, downloadAll bool) {
	m.mu.Lock()
	j := m.findJob(jobID)
	changed := false
	var dto job.DTO
	if j != nil && j.State == job.StateAwaitingPlaylistConfirm {
		j.DownloadPlaylist = downloadAll
		j.SetState(job.StateQueued)
		dto = j.ToDTO()
		changed = true
	}
	m.mu.Unlock()

	if changed {
		m.emit("job:updated", dto)
	}
	m.clearCurrentPrompt(jobID)
	if changed {
		m.scheduleNext()
	}
}

// -- Login / video-password (mirrors submitLogin/submitPassword/skipAuthentication) --
// These feed the credential answer to the goroutine blocked inside
// downloader.Download's RequestLogin/RequestPassword callback for this job.

func (m *Manager) SubmitLogin(jobID, username, password string) {
	m.mu.Lock()
	j := m.findJob(jobID)
	rt := m.runtimes[jobID]
	send := j != nil && j.State == job.StateAwaitingLogin && rt != nil
	var dto job.DTO
	if send {
		j.SetState(job.StateDownloading)
		dto = j.ToDTO()
	}
	m.mu.Unlock()

	if send {
		m.emit("job:updated", dto)
		select {
		case rt.answerCh <- credentialAnswer{username: username, password: password, ok: true}:
		default:
		}
	}
	m.clearCurrentPrompt(jobID)
}

func (m *Manager) SubmitPassword(jobID, password string) {
	m.mu.Lock()
	j := m.findJob(jobID)
	rt := m.runtimes[jobID]
	send := j != nil && j.State == job.StateAwaitingPassword && rt != nil
	var dto job.DTO
	if send {
		j.SetState(job.StateDownloading)
		dto = j.ToDTO()
	}
	m.mu.Unlock()

	if send {
		m.emit("job:updated", dto)
		select {
		case rt.answerCh <- credentialAnswer{password: password, ok: true}:
		default:
		}
	}
	m.clearCurrentPrompt(jobID)
}

func (m *Manager) SkipAuthentication(jobID string) {
	m.mu.Lock()
	j := m.findJob(jobID)
	rt := m.runtimes[jobID]
	send := j != nil && rt != nil && (j.State == job.StateAwaitingLogin || j.State == job.StateAwaitingPassword)
	var dto job.DTO
	if send {
		j.SetState(job.StateDownloading)
		dto = j.ToDTO()
	}
	m.mu.Unlock()

	if send {
		m.emit("job:updated", dto)
		select {
		case rt.answerCh <- credentialAnswer{ok: false}:
		default:
		}
	}
	m.clearCurrentPrompt(jobID)
}

// requestLoginCallback/requestPasswordCallback are handed to
// downloader.Download as Callbacks.RequestLogin/RequestPassword: called from
// the download's stderr-watching goroutine as soon as yt-dlp reports it
// needs credentials. They transition the job, enqueue the prompt, then block
// until SubmitLogin/SubmitPassword/SkipAuthentication answers — or ctx is
// cancelled, e.g. because the user cancelled the job while the prompt was
// still open.

func (m *Manager) requestLoginCallback(jobID string) func(ctx context.Context) (string, string, bool) {
	return func(ctx context.Context) (string, string, bool) {
		m.mu.Lock()
		j := m.findJob(jobID)
		rt := m.runtimes[jobID]
		if j == nil || rt == nil {
			m.mu.Unlock()
			return "", "", false
		}
		j.SetState(job.StateAwaitingLogin)
		dto := j.ToDTO()
		m.mu.Unlock()

		m.emit("job:updated", dto)
		m.requestPrompt(jobID, "login")

		select {
		case ans := <-rt.answerCh:
			return ans.username, ans.password, ans.ok
		case <-ctx.Done():
			return "", "", false
		}
	}
}

func (m *Manager) requestPasswordCallback(jobID string) func(ctx context.Context) (string, bool) {
	return func(ctx context.Context) (string, bool) {
		m.mu.Lock()
		j := m.findJob(jobID)
		rt := m.runtimes[jobID]
		if j == nil || rt == nil {
			m.mu.Unlock()
			return "", false
		}
		j.SetState(job.StateAwaitingPassword)
		dto := j.ToDTO()
		m.mu.Unlock()

		m.emit("job:updated", dto)
		m.requestPrompt(jobID, "password")

		select {
		case ans := <-rt.answerCh:
			return ans.password, ans.ok
		case <-ctx.Done():
			return "", false
		}
	}
}
