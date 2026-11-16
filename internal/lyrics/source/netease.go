package source

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// NetEase looks tracks up in the NetEase Cloud Music catalogue. Its web
// endpoints answer without an account: a lookup sends the track's title and
// artist, and never a cookie or a token.
type NetEase struct {
	Base   string
	Client *http.Client
}

// NewNetEase returns the provider, ready to ask music.163.com.
func NewNetEase() *NetEase {
	return &NetEase{Base: "https://music.163.com", Client: &http.Client{Timeout: 8 * time.Second}}
}

// Name is what the listener sees as the lyrics' source.
func (n *NetEase) Name() string { return "网易云音乐" }

// Find searches for the track and fetches the lyrics of the best match.
func (n *NetEase) Find(ctx context.Context, q Query) (Result, error) {
	if n == nil {
		return Result{}, ErrNotFound
	}
	cands, err := n.Search(ctx, q)
	if err != nil {
		return Result{}, err
	}
	c, ok := best(q, cands)
	if !ok {
		return Result{}, ErrNotFound
	}
	return n.Fetch(ctx, c)
}

// Search queries NetEase Cloud Music for candidates matching q.
func (n *NetEase) Search(ctx context.Context, q Query) ([]Candidate, error) {
	if n == nil {
		return nil, ErrNotFound
	}
	term := strings.TrimSpace(q.Title + " " + q.Artist)
	if term == "" {
		return nil, nil
	}
	v := url.Values{
		"s":     {term},
		"type":  {"1"},
		"limit": {"20"},
		"total": {"true"},
	}
	var out struct {
		Result struct {
			Songs []struct {
				ID   json.Number `json:"id"`
				Name string      `json:"name"`
				Ar   []struct {
					Name  string   `json:"name"`
					Alias []string `json:"alias"`
					Alia  []string `json:"alia"`
					Tns   []string `json:"tns"`
				} `json:"ar"`
				Al struct {
					Name string `json:"name"`
				} `json:"al"`
				DT int64 `json:"dt"`
			} `json:"songs"`
		} `json:"result"`
	}
	if err := getJSON(ctx, n.Client, n.base()+"/api/cloudsearch/pc?"+v.Encode(), map[string]string{"Referer": n.base() + "/"}, &out); err != nil {
		return nil, err
	}
	cands := make([]Candidate, 0, len(out.Result.Songs))
	for _, s := range out.Result.Songs {
		artist := ""
		for i, a := range s.Ar {
			if i > 0 {
				artist += "/"
			}
			artist += a.Name
		}
		var aliases []string
		if len(s.Ar) > 0 {
			primary := s.Ar[0]
			seen := make(map[string]bool)
			for _, arr := range [][]string{primary.Alias, primary.Alia, primary.Tns} {
				for _, alias := range arr {
					alias = strings.TrimSpace(alias)
					if alias != "" && !seen[alias] {
						seen[alias] = true
						aliases = append(aliases, alias)
					}
				}
			}
		}
		cands = append(cands, Candidate{
			Provider:      n.Name(),
			Title:         s.Name,
			Artist:        artist,
			ArtistAliases: aliases,
			Album:         s.Al.Name,
			Duration:      time.Duration(s.DT) * time.Millisecond,
			Ref:           s.ID.String(),
		})
	}
	return cands, nil
}

// Fetch retrieves lyrics for candidate c.
func (n *NetEase) Fetch(ctx context.Context, c Candidate) (Result, error) {
	if n == nil {
		return Result{}, ErrNotFound
	}
	return n.lyric(ctx, c.Ref)
}

func (n *NetEase) lyric(ctx context.Context, id string) (Result, error) {
	v := url.Values{"id": {id}, "lv": {"1"}, "tv": {"-1"}}
	var out struct {
		LRC struct {
			Lyric string `json:"lyric"`
		} `json:"lrc"`
		Tlyric struct {
			Lyric string `json:"lyric"`
		} `json:"tlyric"`
	}
	if err := getJSON(ctx, n.Client, n.base()+"/api/song/lyric?"+v.Encode(), map[string]string{"Referer": n.base() + "/"}, &out); err != nil {
		return Result{}, err
	}
	if out.LRC.Lyric == "" {
		return Result{}, ErrNotFound
	}
	return Result{LRC: out.LRC.Lyric, Translation: out.Tlyric.Lyric, Source: n.Name()}, nil
}

func (n *NetEase) base() string {
	if n == nil || n.Base == "" {
		return "https://music.163.com"
	}
	return n.Base
}
