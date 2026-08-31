package update

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/minio/selfupdate"
)

func Run(out io.Writer, in io.Reader, version, apiBase string, yes bool) error {
	if version == "dev" {
		return errors.New("development build cannot self-update; install a release build")
	}
	client := &http.Client{Timeout: 60 * time.Second}
	rel, err := Latest(client, apiBase)
	if err != nil {
		return err
	}
	if !Newer(rel.Tag, version) {
		fmt.Fprintf(out, "odyssey-cli %s is up to date\n", withV(version))
		return nil
	}
	if !yes {
		fmt.Fprintf(out, "update %s -> %s? [y/N] ", withV(version), withV(rel.Tag))
		answer, _ := bufio.NewReader(in).ReadString('\n')
		answer = strings.TrimSpace(answer)
		if answer != "y" && answer != "Y" {
			return errors.New("update aborted")
		}
	}
	name := AssetName(rel.Tag, runtime.GOOS, runtime.GOARCH)
	archiveURL, ok := rel.Assets[name]
	if !ok {
		return fmt.Errorf("release %s has no asset %s", rel.Tag, name)
	}
	sumsURL, ok := rel.Assets["checksums.txt"]
	if !ok {
		return fmt.Errorf("release %s has no checksums.txt", rel.Tag)
	}
	archive, err := fetch(client, archiveURL)
	if err != nil {
		return err
	}
	sums, err := fetch(client, sumsURL)
	if err != nil {
		return err
	}
	if err := Verify(archive, name, string(sums)); err != nil {
		return err
	}
	bin, err := Extract(archive)
	if err != nil {
		return err
	}
	if err := selfupdate.Apply(bytes.NewReader(bin), selfupdate.Options{}); err != nil {
		return fmt.Errorf("replace executable: %w", err)
	}
	fmt.Fprintf(out, "updated to %s\n", withV(rel.Tag))
	return nil
}
