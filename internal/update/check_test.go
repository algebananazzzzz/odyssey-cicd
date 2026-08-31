package update

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func server(t *testing.T, tag string, hits *int) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/algebananazzzzz/odyssey-cicd/releases/latest" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		*hits++
		fmt.Fprintf(w, `{"tag_name": %q}`, tag)
	}))
	t.Cleanup(s.Close)
	return s
}

func cacheFile(t *testing.T, checkedAt time.Time, latest string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "update-check.json")
	data, err := json.Marshal(cache{CheckedAt: checkedAt, Latest: latest})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRefreshFreshCacheSkipsHTTP(t *testing.T) {
	hits := 0
	s := server(t, "v9.9.9", &hits)
	path := cacheFile(t, time.Now().Add(-time.Hour), "v1.0.0")
	Refresh(path, s.URL)
	if hits != 0 {
		t.Fatalf("hits = %d, want 0", hits)
	}
}

func TestRefreshStaleCache(t *testing.T) {
	hits := 0
	s := server(t, "v2.0.0", &hits)
	path := cacheFile(t, time.Now().Add(-25*time.Hour), "v1.0.0")
	Refresh(path, s.URL)
	if hits != 1 {
		t.Fatalf("hits = %d, want 1", hits)
	}
	c, ok := readCache(path)
	if !ok {
		t.Fatal("cache unreadable after refresh")
	}
	if c.Latest != "v2.0.0" || time.Since(c.CheckedAt) > time.Minute {
		t.Fatalf("cache = %+v", c)
	}
}

func TestRefreshCorruptCache(t *testing.T) {
	hits := 0
	s := server(t, "v2.0.0", &hits)
	path := filepath.Join(t.TempDir(), "update-check.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	Refresh(path, s.URL)
	if hits != 1 {
		t.Fatalf("hits = %d, want 1", hits)
	}
	if c, ok := readCache(path); !ok || c.Latest != "v2.0.0" {
		t.Fatalf("cache = %+v, ok = %v", c, ok)
	}
}

func TestRefreshMissingCache(t *testing.T) {
	hits := 0
	s := server(t, "v2.0.0", &hits)
	path := filepath.Join(t.TempDir(), "odyssey", "update-check.json")
	Refresh(path, s.URL)
	if hits != 1 {
		t.Fatalf("hits = %d, want 1", hits)
	}
	if c, ok := readCache(path); !ok || c.Latest != "v2.0.0" {
		t.Fatalf("cache = %+v, ok = %v", c, ok)
	}
}

func TestRefreshServerErrorKeepsCache(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(s.Close)
	stale := time.Now().Add(-25 * time.Hour)
	path := cacheFile(t, stale, "v1.0.0")
	Refresh(path, s.URL)
	c, ok := readCache(path)
	if !ok || c.Latest != "v1.0.0" || !c.CheckedAt.Equal(stale) {
		t.Fatalf("cache = %+v, ok = %v", c, ok)
	}
}

func TestNudge(t *testing.T) {
	path := cacheFile(t, time.Now(), "v1.5.0")
	var b bytes.Buffer
	Nudge(&b, path, "v1.4.2")
	want := "odyssey-cli v1.5.0 is available (you have v1.4.2). Run: odyssey-cli update\n"
	if b.String() != want {
		t.Fatalf("nudge = %q, want %q", b.String(), want)
	}
}

func TestNudgeSilent(t *testing.T) {
	cases := []struct {
		name, latest, version string
	}{
		{"dev build", "v1.5.0", "dev"},
		{"current", "v1.4.2", "v1.4.2"},
		{"older latest", "v1.4.0", "v1.4.2"},
		{"garbage latest", "garbage", "v1.4.2"},
	}
	for _, c := range cases {
		path := cacheFile(t, time.Now(), c.latest)
		var b bytes.Buffer
		Nudge(&b, path, c.version)
		if b.Len() != 0 {
			t.Errorf("%s: nudge = %q, want none", c.name, b.String())
		}
	}
	var b bytes.Buffer
	Nudge(&b, filepath.Join(t.TempDir(), "missing.json"), "v1.4.2")
	if b.Len() != 0 {
		t.Errorf("missing cache: nudge = %q, want none", b.String())
	}
}
