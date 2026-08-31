package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

func AssetName(tag, goos, goarch string) string {
	return fmt.Sprintf("odyssey-cli_%s_%s_%s.tar.gz", strings.TrimPrefix(tag, "v"), goos, goarch)
}

func Verify(data []byte, name, checksums string) error {
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != name {
			continue
		}
		if fields[0] != sum {
			return fmt.Errorf("checksum mismatch for %s", name)
		}
		return nil
	}
	return fmt.Errorf("checksums.txt has no entry for %s", name)
}

func Extract(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("read archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, errors.New("archive has no odyssey-cli binary")
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg && path.Base(hdr.Name) == "odyssey-cli" {
			return io.ReadAll(tr)
		}
	}
}
