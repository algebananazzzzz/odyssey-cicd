package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func TestAssetName(t *testing.T) {
	cases := []struct {
		tag, goos, goarch, want string
	}{
		{"v1.4.2", "linux", "amd64", "odyssey-cli_1.4.2_linux_amd64.tar.gz"},
		{"1.4.2", "linux", "arm64", "odyssey-cli_1.4.2_linux_arm64.tar.gz"},
		{"v1.4.2", "darwin", "amd64", "odyssey-cli_1.4.2_darwin_amd64.tar.gz"},
		{"v2.0.0", "darwin", "arm64", "odyssey-cli_2.0.0_darwin_arm64.tar.gz"},
	}
	for _, c := range cases {
		if got := AssetName(c.tag, c.goos, c.goarch); got != c.want {
			t.Errorf("AssetName(%q, %q, %q) = %q, want %q", c.tag, c.goos, c.goarch, got, c.want)
		}
	}
}

func archive(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestVerify(t *testing.T) {
	data := archive(t, "odyssey-cli", "fake binary")
	name := "odyssey-cli_1.5.0_linux_amd64.tar.gz"
	sums := fmt.Sprintf("%x  other.tar.gz\n%x  %s\n", sha256.Sum256([]byte("other")), sha256.Sum256(data), name)
	if err := Verify(data, name, sums); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyMismatch(t *testing.T) {
	data := archive(t, "odyssey-cli", "fake binary")
	name := "odyssey-cli_1.5.0_linux_amd64.tar.gz"
	sums := fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte("tampered")), name)
	if err := Verify(data, name, sums); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("err = %v, want checksum mismatch", err)
	}
}

func TestVerifyMissingEntry(t *testing.T) {
	data := archive(t, "odyssey-cli", "fake binary")
	if err := Verify(data, "odyssey-cli_1.5.0_linux_amd64.tar.gz", "deadbeef  other.tar.gz\n"); err == nil {
		t.Fatal("missing checksum entry accepted")
	}
}

func TestExtract(t *testing.T) {
	got, err := Extract(archive(t, "odyssey-cli", "fake binary"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "fake binary" {
		t.Fatalf("extracted %q", got)
	}
}

func TestExtractNested(t *testing.T) {
	got, err := Extract(archive(t, "odyssey-cli_1.5.0_linux_amd64/odyssey-cli", "fake binary"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "fake binary" {
		t.Fatalf("extracted %q", got)
	}
}

func TestExtractMissingBinary(t *testing.T) {
	if _, err := Extract(archive(t, "README.md", "docs")); err == nil {
		t.Fatal("archive without odyssey-cli accepted")
	}
}

func TestExtractNotGzip(t *testing.T) {
	if _, err := Extract([]byte("not a gzip stream")); err == nil {
		t.Fatal("garbage archive accepted")
	}
}
