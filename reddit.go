package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
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

type oauthTokenSource struct {
	ctx                context.Context
	config             *oauth2.Config
	username, password string
}

func (s *oauthTokenSource) Token() (*oauth2.Token, error) {
	return s.config.PasswordCredentialsToken(s.ctx, s.username, s.password)
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
	if resp.StatusCode != 200 {
		return false, fmt.Errorf("got status code %d; error %v", resp.StatusCode, err)
	}

	var redditJSON []RedditPost
	err = json.NewDecoder(resp.Body).Decode(&redditJSON)
	if err != nil {
		return false, err
	}

	d.l.Debug("Reddit URL", "url", url.String())
	for _, post := range redditJSON {
		for _, child := range post.Data.Children {
			if child.Data.Media.RedditVideo.FallbackUrl != "" {
				d.l.Debug("Fallback URL for video", "fallback_url", child.Data.Media.RedditVideo.FallbackUrl)
				ok := d.c.Set(fixURL(url.String()), true, 1)
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

func (d *DiscordBotHandler) getVRedditRedirect(id string) (string, error) {
	d.h.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		d.l.Debug("Redirect", "req", req, "via", via, "status", req.Response.StatusCode, "headers", req.Response.Header, "req headers", req.Header)
		return http.ErrUseLastResponse
	}

	d.getRedditToken()
	req, err := http.NewRequest("HEAD", fmt.Sprintf("https://www.reddit.com/video/%s", id), nil)
	if err != nil {
		return "", err
	}

	resp, err := d.h.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	d.l.Debug("Redirect URL", "url", resp.Header.Get("Location"), "status", resp.StatusCode, "headers", resp, "body", resp.Body)

	d.c.Set(id, resp.Header.Get("Location"), 1)
	return resp.Header.Get("Location"), nil
}

func (d *DiscordBotHandler) getRedditToken() string {
	d.l.Debug("Getting Reddit token", "token_url", d.r.TokenURL.String())
	return ""
}
