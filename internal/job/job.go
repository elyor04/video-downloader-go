// Package job defines the download-job state machine and its data shape.
// It is a direct port of the Python DownloadJob dataclass and its state
// machine (backend/job.py), plus the row-formatting logic that used to live
// in the QAbstractListModel (backend/queue_model.py) — merged here into a
// single ToDTO method since there is no separate Qt model layer in Go.
package job

import (
	"fmt"

	"video-downloader-go/internal/utils"

	"github.com/google/uuid"
)

type State string

const (
	StateFetching                State = "fetching"
	StateAwaitingPlaylistConfirm State = "awaiting_playlist_confirm"
	StateQueued                  State = "queued"
	StateDownloading             State = "downloading"
	StateAwaitingLogin           State = "awaiting_login"
	StateAwaitingPassword        State = "awaiting_password"
	StateSuccess                 State = "success"
	StateError                   State = "error"
	StateCancelled               State = "cancelled"
)

type Mode string

const (
	ModeVideo Mode = "video"
	ModeAudio Mode = "audio"
)

// validTransitions mirrors job.py's VALID_TRANSITIONS exactly.
var validTransitions = map[State]map[State]bool{
	StateFetching: {
		StateQueued:                  true,
		StateAwaitingPlaylistConfirm: true,
		StateError:                   true,
		StateCancelled:               true,
	},
	StateAwaitingPlaylistConfirm: {
		StateQueued:    true,
		StateCancelled: true,
	},
	StateQueued: {
		StateDownloading: true,
		StateCancelled:   true,
	},
	StateDownloading: {
		StateAwaitingLogin:    true,
		StateAwaitingPassword: true,
		StateSuccess:          true,
		StateError:            true,
		StateCancelled:        true,
	},
	StateAwaitingLogin: {
		StateDownloading: true,
		StateCancelled:   true,
	},
	StateAwaitingPassword: {
		StateDownloading: true,
		StateCancelled:   true,
	},
	StateSuccess:   {},
	StateError:     {},
	StateCancelled: {},
}

var ActiveStates = map[State]bool{
	StateFetching:                true,
	StateAwaitingPlaylistConfirm: true,
	StateQueued:                  true,
	StateAwaitingLogin:           true,
	StateAwaitingPassword:        true,
	StateDownloading:             true,
}

var TerminalStates = map[State]bool{
	StateSuccess:   true,
	StateError:     true,
	StateCancelled: true,
}

// Job holds a download's full state. It is only ever mutated while the
// owning manager's mutex is held (see internal/manager) — mirroring how the
// Python DownloadJob was only ever touched from Qt's single-threaded main
// loop. Job itself carries no OS process handles; the manager keeps those
// separately so this package stays free of concurrency/process concerns.
type Job struct {
	ID         string
	URL        string
	Mode       Mode
	Resolution int
	ConvertTo  string
	OutputDir  string
	FileName   string

	State        State
	Title        string
	Thumbnail    string
	ErrorMessage string

	IsPlaylist       bool
	PlaylistCount    int // 0 means unknown, mirrors Python's Optional[int] via 0-as-absent
	DownloadPlaylist bool

	Progress        float64 // 0..1, negative = indeterminate/unknown
	DownloadedBytes int64
	TotalBytes      int64
	Speed           float64
	ETA             float64
	PlaylistIndex   int // 0 means "not part of a playlist download"
	Stage           string

	FinishedDir string
}

// New mirrors DownloadJob's constructor defaults from job.py.
func New(url string, mode Mode, resolution int, convertTo, outputDir, fileName string) *Job {
	return &Job{
		ID:               uuid.NewString(),
		URL:              url,
		Mode:             mode,
		Resolution:       resolution,
		ConvertTo:        convertTo,
		OutputDir:        outputDir,
		FileName:         fileName,
		State:            StateFetching,
		DownloadPlaylist: true,
		Progress:         -1,
		DownloadedBytes:  -1,
		TotalBytes:       -1,
		Speed:            -1,
		ETA:              -1,
		Stage:            "downloading",
	}
}

// SetState mirrors job.py's set_state: panics on an invalid transition,
// exactly like the Python assert. Callers (the manager) run each job's work
// under a recover() guard so one bad transition can't take the app down —
// see the manager package for the "why" (a stray unhandled panic in a
// background goroutine would otherwise crash the whole process).
func (j *Job) SetState(newState State) {
	allowed := validTransitions[j.State]
	if !allowed[newState] {
		panic(fmt.Sprintf("invalid job transition %q -> %q", j.State, newState))
	}
	j.State = newState
}

// DTO is the JSON shape sent to the frontend, replacing queue_model.py's
// role-based QAbstractListModel::data().
type DTO struct {
	ID             string  `json:"jobId"`
	URL            string  `json:"url"`
	Title          string  `json:"title"`
	Thumbnail      string  `json:"thumbnail"`
	State          string  `json:"jobState"`
	Stage          string  `json:"stage"`
	Mode           string  `json:"mode"`
	Progress       float64 `json:"progress"`
	PlaylistIndex  int     `json:"playlistIndex"`
	PlaylistCount  int     `json:"playlistCount"`
	DownloadedText string  `json:"downloadedText"`
	TotalText      string  `json:"totalText"`
	SpeedText      string  `json:"speedText"`
	EtaText        string  `json:"etaText"`
	ErrorMessage   string  `json:"errorMessage"`
	CanCancel      bool    `json:"canCancel"`
	CanRemove      bool    `json:"canRemove"`
	CanOpenFolder  bool    `json:"canOpenFolder"`
}

func (j *Job) ToDTO() DTO {
	title := j.Title
	if title == "" {
		title = j.URL
	}
	return DTO{
		ID:             j.ID,
		URL:            j.URL,
		Title:          title,
		Thumbnail:      j.Thumbnail,
		State:          string(j.State),
		Stage:          j.Stage,
		Mode:           string(j.Mode),
		Progress:       j.Progress,
		PlaylistIndex:  j.PlaylistIndex,
		PlaylistCount:  j.PlaylistCount,
		DownloadedText: utils.FormatBytes(float64(j.DownloadedBytes)),
		TotalText:      utils.FormatBytes(float64(j.TotalBytes)),
		SpeedText:      utils.FormatSpeed(j.Speed),
		EtaText:        utils.FormatEta(j.ETA),
		ErrorMessage:   j.ErrorMessage,
		CanCancel:      ActiveStates[j.State],
		CanRemove:      TerminalStates[j.State],
		CanOpenFolder:  j.State == StateSuccess,
	}
}
