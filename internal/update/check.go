package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const GitHubAPI = "https://api.github.com"

type cache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

func CachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "odyssey", "update-check.json")
}

func Refresh(path, apiBase string) {
	if path == "" {
		return
	}
	if c, ok := readCache(path); ok && time.Since(c.CheckedAt) < 24*time.Hour {
		return
	}
	client := &http.Client{Timeout: 2 * time.Second}
	rel, err := Latest(client, apiBase)
	if err != nil {
		return
	}
	writeCache(path, cache{CheckedAt: time.Now(), Latest: rel.Tag})
}

func Nudge(w io.Writer, path, version string) {
	if version == "dev" {
		return
	}
	c, ok := readCache(path)
	if !ok || !Newer(c.Latest, version) {
		return
	}
	fmt.Fprintf(w, "odyssey-cli %s is available (you have %s). Run: odyssey-cli update\n", withV(c.Latest), withV(version))
}

func readCache(path string) (cache, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cache{}, false
	}
	var c cache
	if err := json.Unmarshal(data, &c); err != nil || c.CheckedAt.IsZero() || c.Latest == "" {
		return cache{}, false
	}
	return c, true
}

func writeCache(path string, c cache) {
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	os.Rename(tmp, path)
}
