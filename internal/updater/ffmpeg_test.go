package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
)

func TestBtbnPlatformFor(t *testing.T) {
	cases := []struct {
		goos, goarch string
		want         string
		wantErr      bool
	}{
		{"windows", "amd64", "win64", false},
		{"windows", "arm64", "winarm64", false},
		{"linux", "amd64", "linux64", false},
		{"linux", "arm64", "linuxarm64", false},
		{"windows", "386", "", true},
		{"linux", "386", "", true},
	}
	for _, c := range cases {
		got, err := btbnPlatformFor(c.goos, c.goarch)
		if c.wantErr {
			if err == nil {
				t.Errorf("btbnPlatformFor(%q, %q) = %q, want error", c.goos, c.goarch, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("btbnPlatformFor(%q, %q) unexpected error: %v", c.goos, c.goarch, err)
			continue
		}
		if got != c.want {
			t.Errorf("btbnPlatformFor(%q, %q) = %q, want %q", c.goos, c.goarch, got, c.want)
		}
	}
}

func TestInstalledFfmpegVersionLine(t *testing.T) {
	t.Setenv("UPDATER_TEST_HELPER", "ffmpeg")
	got, err := InstalledFfmpegVersionLine(context.Background(), os.Args[0])
	if err != nil {
		t.Fatalf("InstalledFfmpegVersionLine() error = %v", err)
	}
	want := "ffmpeg version 8.1-full_build-www.gyan.dev"
	if got != want {
		t.Errorf("InstalledFfmpegVersionLine() = %q, want %q", got, want)
	}
}

// btbnAssetExt mirrors fetchBtbN's own OS->extension choice, for building
// realistic fake asset names in tests.
func btbnAssetExt() string {
	if runtime.GOOS == "linux" {
		return "tar.xz"
	}
	return "zip"
}

func TestLatestFfmpegVersionLinePicksHighestMatchingLine(t *testing.T) {
	platform, err := btbnPlatform()
	if err != nil {
		t.Skipf("btbnPlatform() unsupported on this test platform: %v", err)
	}
	ext := btbnAssetExt()

	assetNames := []string{
		"ffmpeg-n7.1-latest-" + platform + "-gpl-7.1." + ext,
		"ffmpeg-n8.1-latest-" + platform + "-gpl-8.1." + ext,
		"ffmpeg-n8.1-latest-" + platform + "-gpl-shared-8.1." + ext, // shared build: must be ignored
		"ffmpeg-n9.5-latest-" + platform + "-lgpl-9.5." + ext,       // lgpl build: must be ignored
		"ffmpeg-n99.0-latest-someotherplatform-gpl-99.0." + ext,     // wrong platform: must be ignored
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type asset struct {
			Name string `json:"name"`
		}
		assets := make([]asset, len(assetNames))
		for i, n := range assetNames {
			assets[i] = asset{Name: n}
		}
		_ = json.NewEncoder(w).Encode(struct {
			Assets []asset `json:"assets"`
		}{Assets: assets})
	}))
	defer srv.Close()

	orig := btbnLatestReleaseURL
	btbnLatestReleaseURL = srv.URL
	defer func() { btbnLatestReleaseURL = orig }()

	got, err := LatestFfmpegVersionLine(context.Background())
	if err != nil {
		t.Fatalf("LatestFfmpegVersionLine() error = %v", err)
	}
	if got != "8.1" {
		t.Errorf("LatestFfmpegVersionLine() = %q, want %q (shared/lgpl/other-platform assets must be ignored)", got, "8.1")
	}
}

func TestLatestFfmpegVersionLineNoMatchingAssets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Assets []struct {
				Name string `json:"name"`
			} `json:"assets"`
		}{})
	}))
	defer srv.Close()

	orig := btbnLatestReleaseURL
	btbnLatestReleaseURL = srv.URL
	defer func() { btbnLatestReleaseURL = orig }()

	if _, err := LatestFfmpegVersionLine(context.Background()); err == nil {
		t.Error("LatestFfmpegVersionLine() error = nil, want an error when no assets match this platform")
	}
}

func TestFetchEvermeetInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ffmpeg/release" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(evermeetRelease{
			Version: "8.1.2",
			Download: struct {
				Zip struct {
					URL string `json:"url"`
				} `json:"zip"`
			}{Zip: struct {
				URL string `json:"url"`
			}{URL: "https://evermeet.cx/ffmpeg/ffmpeg-8.1.2.zip"}},
		})
	}))
	defer srv.Close()

	orig := evermeetInfoBaseURL
	evermeetInfoBaseURL = srv.URL
	defer func() { evermeetInfoBaseURL = orig }()

	info, err := fetchEvermeetInfo(context.Background(), "ffmpeg")
	if err != nil {
		t.Fatalf("fetchEvermeetInfo() error = %v", err)
	}
	if info.Version != "8.1.2" {
		t.Errorf("info.Version = %q, want %q", info.Version, "8.1.2")
	}
	if info.Download.Zip.URL != "https://evermeet.cx/ffmpeg/ffmpeg-8.1.2.zip" {
		t.Errorf("info.Download.Zip.URL = %q, want %q", info.Download.Zip.URL, "https://evermeet.cx/ffmpeg/ffmpeg-8.1.2.zip")
	}
}

func TestFetchEvermeetInfoMissingURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(evermeetRelease{Version: "8.1.2"})
	}))
	defer srv.Close()

	orig := evermeetInfoBaseURL
	evermeetInfoBaseURL = srv.URL
	defer func() { evermeetInfoBaseURL = orig }()

	if _, err := fetchEvermeetInfo(context.Background(), "ffmpeg"); err == nil {
		t.Error("fetchEvermeetInfo() error = nil, want an error when the response has no zip download URL")
	}
}
