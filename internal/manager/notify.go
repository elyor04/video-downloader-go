package manager

import (
	_ "embed"

	"github.com/gen2brain/beeep"
)

// appIconPNG is passed straight to beeep.Notify as raw PNG bytes -- every
// platform backend (Windows toast/balloon, macOS terminal-notifier/
// osascript, Linux dbus/notify-send/kdialog) accepts either a path string
// or []byte PNG data and does its own PNG decode + native icon-format
// conversion (.ico/.icns) internally, cleaning up any temp file it creates.
// Embedding it keeps the icon available regardless of install layout,
// rather than depending on a loose file existing next to the binary.
//
//go:embed assets/app-icon.png
var appIconPNG []byte

func init() {
	// beeep.AppName defaults to the literal string "DefaultAppName" and is
	// used as the OS-level notification sender identity (Windows toast
	// AppID, macOS `osascript -group`, Linux `notify-send -a`) -- left
	// unset, every notification below would show "DefaultAppName" instead
	// of this app.
	beeep.AppName = "Video Downloader"
}

// notificationTitles covers the only two strings the original app ever put
// in a native OS notification (backend/download_manager.py's
// _notify_if_unfocused). There's no other native-notification surface, so a
// tiny table here is simpler than routing this through the frontend's
// i18next just for two short phrases.
var notificationTitles = map[string]map[string]string{
	"en": {"finished": "Download finished", "failed": "Download failed"},
	"ru": {"finished": "Загрузка завершена", "failed": "Загрузка не удалась"},
	"uz": {"finished": "Yuklab olish tugadi", "failed": "Yuklab olish muvaffaqiyatsiz tugadi"},
}

// maybeNotify mirrors _notify_if_unfocused: only fire a native notification
// if the window isn't already focused (SetWindowFocused is kept up to date
// by the frontend's window focus/blur listeners).
func (m *Manager) maybeNotify(success bool, jobTitle string) {
	m.mu.Lock()
	focused := m.windowFocused
	language := m.language
	m.mu.Unlock()
	if focused {
		return
	}

	titles := notificationTitles[language]
	if titles == nil {
		titles = notificationTitles["en"]
	}
	key := "failed"
	if success {
		key = "finished"
	}
	_ = beeep.Notify(titles[key], jobTitle, appIconPNG)
}
