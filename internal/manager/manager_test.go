package manager

import (
	"testing"

	"video-downloader-go/internal/job"
	"video-downloader-go/internal/utils"
)

func newTestManager() *Manager {
	// "yt-dlp" is never actually invoked by these tests -- they exercise
	// the manager's own state transitions and guards, not the subprocess.
	return New("yt-dlp")
}

func addTestJob(m *Manager, state job.State) *job.Job {
	j := job.New("https://example.com/video", job.ModeVideo, 1080, "original", "/tmp", "%(title)s")
	j.State = state
	m.jobs = append(m.jobs, j)
	m.runtimes[j.ID] = &jobRuntime{answerCh: make(chan credentialAnswer, 1)}
	return j
}

func drainAnswer(t *testing.T, m *Manager, jobID string) credentialAnswer {
	t.Helper()
	select {
	case ans := <-m.runtimes[jobID].answerCh:
		return ans
	default:
		t.Fatal("expected an answer on the job's answerCh, got none")
		return credentialAnswer{}
	}
}

func assertNoAnswer(t *testing.T, m *Manager, jobID string) {
	t.Helper()
	select {
	case ans := <-m.runtimes[jobID].answerCh:
		t.Fatalf("expected no answer, got %+v", ans)
	default:
	}
}

// -- submitLogin / submitPassword / skipAuthentication --

func TestSubmitLoginTransitionsJobToDownloading(t *testing.T) {
	m := newTestManager()
	j := addTestJob(m, job.StateAwaitingLogin)

	m.SubmitLogin(j.ID, "user", "pass")

	if j.State != job.StateDownloading {
		t.Fatalf("State = %q, want %q", j.State, job.StateDownloading)
	}
	ans := drainAnswer(t, m, j.ID)
	if ans.username != "user" || ans.password != "pass" || !ans.ok {
		t.Fatalf("unexpected answer: %+v", ans)
	}
}

func TestSubmitPasswordTransitionsJobToDownloading(t *testing.T) {
	m := newTestManager()
	j := addTestJob(m, job.StateAwaitingPassword)

	m.SubmitPassword(j.ID, "secret")

	if j.State != job.StateDownloading {
		t.Fatalf("State = %q, want %q", j.State, job.StateDownloading)
	}
	ans := drainAnswer(t, m, j.ID)
	if ans.password != "secret" || !ans.ok {
		t.Fatalf("unexpected answer: %+v", ans)
	}
}

func TestSkipAuthenticationTransitionsJobToDownloading(t *testing.T) {
	m := newTestManager()
	j := addTestJob(m, job.StateAwaitingLogin)

	m.SkipAuthentication(j.ID)

	if j.State != job.StateDownloading {
		t.Fatalf("State = %q, want %q", j.State, job.StateDownloading)
	}
	ans := drainAnswer(t, m, j.ID)
	if ans.ok {
		t.Fatalf("expected ok=false for a skip, got %+v", ans)
	}
}

// Regression coverage for the double-submit edge case ported from
// test_download_manager.py: a rapid double-click on a dialog button could
// re-enter these methods after the job had already left its "awaiting_*"
// state. Each method guards on current state before acting, so a stale
// second call is a silent no-op rather than an invalid state transition.

func TestDoubleSubmitLoginIsANoOp(t *testing.T) {
	m := newTestManager()
	j := addTestJob(m, job.StateAwaitingLogin)

	m.SubmitLogin(j.ID, "user", "pass")
	drainAnswer(t, m, j.ID)

	m.SubmitLogin(j.ID, "user", "pass") // stale second click
	if j.State != job.StateDownloading {
		t.Fatalf("State = %q, want unchanged %q", j.State, job.StateDownloading)
	}
	assertNoAnswer(t, m, j.ID)
}

func TestDoubleSubmitPasswordIsANoOp(t *testing.T) {
	m := newTestManager()
	j := addTestJob(m, job.StateAwaitingPassword)

	m.SubmitPassword(j.ID, "secret")
	drainAnswer(t, m, j.ID)

	m.SubmitPassword(j.ID, "secret")
	if j.State != job.StateDownloading {
		t.Fatalf("State = %q, want unchanged %q", j.State, job.StateDownloading)
	}
	assertNoAnswer(t, m, j.ID)
}

func TestDoubleSkipAuthenticationIsANoOp(t *testing.T) {
	m := newTestManager()
	j := addTestJob(m, job.StateAwaitingLogin)

	m.SkipAuthentication(j.ID)
	drainAnswer(t, m, j.ID)

	m.SkipAuthentication(j.ID)
	if j.State != job.StateDownloading {
		t.Fatalf("State = %q, want unchanged %q", j.State, job.StateDownloading)
	}
	assertNoAnswer(t, m, j.ID)
}

func TestSubmitLoginAfterJobAlreadyCancelledIsIgnored(t *testing.T) {
	m := newTestManager()
	j := addTestJob(m, job.StateAwaitingLogin)
	j.State = job.StateCancelled

	m.SubmitLogin(j.ID, "user", "pass") // must not panic

	if j.State != job.StateCancelled {
		t.Fatalf("State = %q, want unchanged %q", j.State, job.StateCancelled)
	}
	assertNoAnswer(t, m, j.ID)
}

// -- resolution selection --

func TestResolutionOptionsReflectsFullLadderByDefault(t *testing.T) {
	m := newTestManager()
	m.mu.Lock()
	opts := m.resolutionOptionsLocked()
	m.mu.Unlock()
	if len(opts) != len(utils.ResolutionLadder) {
		t.Fatalf("got %d options, want %d", len(opts), len(utils.ResolutionLadder))
	}
}

func TestResolutionOptionsNarrowsToPreviewMaxHeight(t *testing.T) {
	m := newTestManager()
	m.mu.Lock()
	m.previewMaxHeight = 480
	opts := m.resolutionOptionsLocked()
	m.mu.Unlock()

	hasBest := false
	for _, o := range opts {
		if o.Value == utils.MaxResolution {
			hasBest = true
			continue
		}
		if o.Value > 480 {
			t.Errorf("resolution %d should have been filtered out when narrowed to 480", o.Value)
		}
	}
	if !hasBest {
		t.Error("\"Best\" should always remain available")
	}
}

func TestSetResolutionAcceptsAValidValue(t *testing.T) {
	m := newTestManager()
	m.SetResolution(720)
	if m.resolution != 720 {
		t.Fatalf("resolution = %d, want 720", m.resolution)
	}
}

func TestSetResolutionRejectsAValueNotOnTheLadder(t *testing.T) {
	m := newTestManager()
	m.resolution = 720
	m.SetResolution(999999)
	if m.resolution != 720 {
		t.Fatalf("resolution = %d, want unchanged 720", m.resolution)
	}
}

func TestResetResolutionToBest(t *testing.T) {
	m := newTestManager()
	m.resolution = 720
	m.ResetResolutionToBest()
	if m.resolution != utils.MaxResolution {
		t.Fatalf("resolution = %d, want %d", m.resolution, utils.MaxResolution)
	}
}

// -- convert-format selection --

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestConvertOptionsReflectsVideoOptionsByDefault(t *testing.T) {
	m := newTestManager()
	m.mu.Lock()
	opts := m.convertOptionsLocked()
	m.mu.Unlock()
	if !equalStrings(opts, utils.VideoConvertOptions) {
		t.Fatalf("got %v, want %v", opts, utils.VideoConvertOptions)
	}
}

func TestConvertOptionsReflectsAudioOptionsInAudioMode(t *testing.T) {
	m := newTestManager()
	m.SetMode("audio")
	m.mu.Lock()
	opts := m.convertOptionsLocked()
	m.mu.Unlock()
	if !equalStrings(opts, utils.AudioConvertOptions) {
		t.Fatalf("got %v, want %v", opts, utils.AudioConvertOptions)
	}
}

func TestSetConvertToAcceptsAValidValueForCurrentMode(t *testing.T) {
	m := newTestManager()
	m.SetConvertTo("mp4")
	if m.convertTo != "mp4" {
		t.Fatalf("convertTo = %q, want mp4", m.convertTo)
	}
}

func TestSetConvertToRejectsValueFromTheOtherMode(t *testing.T) {
	m := newTestManager()
	m.convertTo = "original"
	m.SetConvertTo("mp3") // manager starts in "video" mode; mp3 is audio-only
	if m.convertTo != "original" {
		t.Fatalf("convertTo = %q, want unchanged original", m.convertTo)
	}
}

func TestResetConvertToOriginal(t *testing.T) {
	m := newTestManager()
	m.SetConvertTo("mkv")
	m.ResetConvertToOriginal()
	if m.convertTo != "original" {
		t.Fatalf("convertTo = %q, want original", m.convertTo)
	}
}
