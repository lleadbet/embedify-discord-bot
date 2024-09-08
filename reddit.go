package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	Title string                   `json:"title"`
	Media RedditPostChildDataMedia `json:"media"`
}

type RedditPostChildDataMedia struct {
	RedditVideo RedditPostChildDataMediaRedditVideo `json:"reddit_video"`
}

type RedditPostChildDataMediaRedditVideo struct {
	FallbackUrl string `json:"fallback_url"`
}

func (d *DiscordBotHandler) isRedditVideo(url string) (bool, error) {
	client := http.DefaultClient
	req, err := http.NewRequest("GET", fixURL(url), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/116.0")
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false, fmt.Errorf("got status code %d", resp.StatusCode)
	}

	var redditJSON []RedditPost
	err = json.NewDecoder(resp.Body).Decode(&redditJSON)
	if err != nil {
		return false, err
	}

	d.l.Debug("Reddit URL", "url", url)
	for _, post := range redditJSON {
		for _, child := range post.Data.Children {
			if child.Data.Media.RedditVideo.FallbackUrl != "" {
				d.l.Debug("Fallback URL for video", "fallback_url", child.Data.Media.RedditVideo.FallbackUrl)
				ok := d.c.Set(fixURL(url), true, 1)
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