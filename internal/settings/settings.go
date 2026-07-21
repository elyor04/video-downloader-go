// Package settings persists the handful of user preferences that used to
// live in QSettings("elyor04", "VideoDownloader") — the output directory and
// UI language — as a small JSON file under the user's config directory.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	OutputDir string `json:"outputDir"`
	Language  string `json:"language"`
}

func filePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "VideoDownloader")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "settings.json"), nil
}

// Load returns saved settings, or zero-value Settings if none exist yet or
// the file can't be read/parsed.
func Load() Settings {
	path, err := filePath()
	if err != nil {
		return Settings{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Settings{}
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}
	}
	return s
}

func (s Settings) Save() error {
	path, err := filePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
