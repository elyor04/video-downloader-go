package manager

import "github.com/gen2brain/beeep"

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
	_ = beeep.Notify(titles[key], jobTitle, "")
}
