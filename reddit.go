package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type RedditPost struct {
	Kind string         `json:"kind"`
	Data RedditPostData `json:"data"`
}

type RedditPostData struct {
	Children []RedditPostChild `json:"children"`
}

type RedditPostChild struct {
	Kind string              `json:"kind"`
	Data RedditPostChildData `json:"data"`
}

type RedditPostChildData struct {
	Title               string                   `json:"title"`
	Media               RedditPostChildDataMedia `json:"media"`
	Permalink           string                   `json:"permalink"`
	CrosspostParentList []RedditPostChildData    `json:"crosspost_parent_list"`
}

type RedditPostChildDataMedia struct {
	RedditVideo RedditPostChildDataMediaRedditVideo `json:"reddit_video"`
}

type RedditPostChildDataMediaRedditVideo struct {
	FallbackUrl string `json:"fallback_url"`
}

func (d *DiscordBotHandler) isRedditVideo(url *url.URL) (bool, error) {
	req, err := http.NewRequest("GET", fixURL(url.String()), nil)
	if err != nil {
		return false, err
	}

	resp, err := d.h.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return false, fmt.Errorf("got status code %d; error %v; body %v", resp.StatusCode, err, string(respBody[:]))
	}
	var redditJSON []RedditPost
	err = json.Unmarshal(respBody, &redditJSON)
	if err != nil {
		d.l.Error("Error decoding Reddit JSON", "error", err)
		return false, err
	}

	d.l.Debug("Reddit URL", "url", url.String())
	for _, post := range redditJSON {
		d.l.Debug("Reddit Post", "post", post)
		for _, child := range post.Data.Children {
			if child.Data.Media.RedditVideo.FallbackUrl != "" {
				d.l.Debug("Fallback URL for video", "fallback_url", child.Data.Media.RedditVideo.FallbackUrl)
				ok := d.c.Set(url.String(), true, 1)
				if !ok {
					d.l.Error("Failed to set cache", "url", url)
					return false, errors.New("failed to set cache")
				}
				return true, nil
			}
		}
	}
	return false, nil
}

func fixURL(url string) string {
	if strings.HasPrefix(url, "/") {
		url = url[:len(url)-1]
	}
	if strings.HasSuffix(url, "||") {
		return url[:len(url)-2]
	}
	return fmt.Sprintf("%s.json", url)
}

func (d *DiscordBotHandler) getVRedditRedirect(videoURL string) (string, error) {
	requestURL, err := url.Parse("https://www.reddit.com/search/.json")
	if err != nil {
		return "", err
	}

	params := requestURL.Query()

	params.Add("q", videoURL)
	params.Add("limit", "1")
	requestURL.RawQuery = params.Encode()

	req, err := http.NewRequest("GET", requestURL.String(), nil)
	if err != nil {
		return "", err
	}

	resp, err := d.h.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	d.l.Debug("Reddit URL", "url", videoURL, "resp", resp.Body)

	var redditJSON RedditPost
	err = json.NewDecoder(resp.Body).Decode(&redditJSON)
	if err != nil {
		return "", err
	}

	if redditJSON.Kind == "" {
		d.l.Debug("Reddit JSON Kind is empty", "json", redditJSON)
		return "", nil
	}

	for _, child := range redditJSON.Data.Children {
		if len(child.Data.CrosspostParentList) > 0 {
			if child.Data.CrosspostParentList[0].Permalink != "" {
				fixedURL := fmt.Sprintf("https://www.rxddit.com%s", child.Data.CrosspostParentList[0].Permalink)
				d.l.Debug("Fallback URL for video", "permalink", fixedURL)
				ok := d.c.Set(videoURL, fixedURL, 1)
				if !ok {
					d.l.Error("Failed to set cache", "url", videoURL)
					return "", errors.New("failed to set cache")
				}
				return fixedURL, nil
			}
		}
		if child.Data.Permalink != "" {
			fixedURL := fmt.Sprintf("https://www.rxddit.com%s", child.Data.Permalink)
			d.l.Debug("Fallback URL for video", "permalink", fixedURL)
			ok := d.c.Set(videoURL, fixedURL, 1)
			if !ok {
				d.l.Error("Failed to set cache", "url", videoURL)
				return "", errors.New("failed to set cache")
			}
			return fixedURL, nil
		}
	}

	return "", nil
}
