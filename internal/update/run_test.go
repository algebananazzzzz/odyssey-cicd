package update

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func releaseServer(t *testing.T, tag, checksums string, archive []byte, downloads *int) *httptest.Server {
	t.Helper()
	name := AssetName(tag, runtime.GOOS, runtime.GOARCH)
	mux := http.NewServeMux()
	var s *httptest.Server
	mux.HandleFunc("/repos/algebananazzzzz/odyssey-cicd/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name": %q, "assets": [
			{"name": %q, "browser_download_url": %q},
			{"name": "checksums.txt", "browser_download_url": %q}
		]}`, tag, name, s.URL+"/archive", s.URL+"/checksums")
	})
	mux.HandleFunc("/archive", func(w http.ResponseWriter, r *http.Request) {
		*downloads++
		w.Write(archive)
	})
	mux.HandleFunc("/checksums", func(w http.ResponseWriter, r *http.Request) {
		*downloads++
		io.WriteString(w, checksums)
	})
	s = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

func TestRunDevBuild(t *testing.T) {
	err := Run(io.Discard, strings.NewReader(""), "dev", "http://127.0.0.1:0", false)
	if err == nil || !strings.Contains(err.Error(), "development build") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunUpToDate(t *testing.T) {
	downloads := 0
	s := releaseServer(t, "v1.4.2", "", nil, &downloads)
	var out bytes.Buffer
	if err := Run(&out, strings.NewReader(""), "1.4.2", s.URL, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "up to date") {
		t.Fatalf("out = %q", out.String())
	}
	if downloads != 0 {
		t.Fatalf("downloads = %d, want 0", downloads)
	}
}

func TestRunAbort(t *testing.T) {
	downloads := 0
	s := releaseServer(t, "v2.0.0", "", nil, &downloads)
	var out bytes.Buffer
	err := Run(&out, strings.NewReader("n\n"), "v1.4.2", s.URL, false)
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out.String(), "update v1.4.2 -> v2.0.0? [y/N]") {
		t.Fatalf("out = %q", out.String())
	}
	if downloads != 0 {
		t.Fatalf("downloads = %d, want 0", downloads)
	}
}

func TestRunChecksumMismatch(t *testing.T) {
	data := archive(t, "odyssey-cli", "fake binary")
	name := AssetName("v2.0.0", runtime.GOOS, runtime.GOARCH)
	sums := fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte("tampered")), name)
	downloads := 0
	s := releaseServer(t, "v2.0.0", sums, data, &downloads)
	err := Run(io.Discard, strings.NewReader(""), "v1.4.2", s.URL, true)
	if err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("err = %v", err)
	}
	if downloads != 2 {
		t.Fatalf("downloads = %d, want 2", downloads)
	}
}
