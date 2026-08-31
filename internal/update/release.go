package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type Release struct {
	Tag    string
	Assets map[string]string
}

func Latest(client *http.Client, apiBase string) (Release, error) {
	body, err := fetch(client, apiBase+"/repos/algebananazzzzz/odyssey-cicd/releases/latest")
	if err != nil {
		return Release{}, err
	}
	var raw struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Release{}, fmt.Errorf("decode release: %w", err)
	}
	if raw.TagName == "" {
		return Release{}, errors.New("release has no tag_name")
	}
	rel := Release{Tag: raw.TagName, Assets: map[string]string{}}
	for _, a := range raw.Assets {
		rel.Assets[a.Name] = a.URL
	}
	return rel, nil
}

func fetch(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}
