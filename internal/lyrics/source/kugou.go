/**
 * [INPUT]: 依赖 context, encoding/base64, http, net/url, strconv 以及同包 Candidate/Query/Result
 * [OUTPUT]: 对外提供 Kugou Provider，实现 Search, Fetch, Find 接口并以 base64 解码下载 LRC
 * [POS]: internal/lyrics/source 的酷狗音乐歌词检索与下载实现，检索结果同时携带 id 与 accesskey
 * [PROTOCOL]: 变更时更新此头部，然后检查 AGENTS.md
 */
package source

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Kugou looks tracks up in the Kugou catalogue: search returns a candidate id
// and the access key its lyrics need, and the download endpoint answers with
// base64 LRC. Only the track's metadata is sent.
type Kugou struct {
	Base     string
	Download string
	Client   *http.Client
}

// NewKugou returns the provider, ready to ask krcs.kugou.com.
func NewKugou() *Kugou {
	return &Kugou{
		Base:     "https://krcs.kugou.com",
		Download: "https://lyrics.kugou.com",
		Client:   &http.Client{Timeout: 8 * time.Second},
	}
}

// Name is what the listener sees as the lyrics' source.
func (k *Kugou) Name() string { return "酷狗音乐" }

// Find searches for the track and downloads the lyrics of the best match.
func (k *Kugou) Find(ctx context.Context, q Query) (Result, error) {
	if k == nil {
		return Result{}, ErrNotFound
	}
	cands, err := k.Search(ctx, q)
	if err != nil {
		return Result{}, err
	}
	c, ok := best(q, cands)
	if !ok {
		return Result{}, ErrNotFound
	}
	return k.Fetch(ctx, c)
}

// Search queries Kugou for candidates matching q.
func (k *Kugou) Search(ctx context.Context, q Query) ([]Candidate, error) {
	if k == nil {
		return nil, ErrNotFound
	}
	term := strings.TrimSpace(q.Title + " " + q.Artist)
	if term == "" {
		return nil, nil
	}
	v := url.Values{
		"ver":     {"1"},
		"man":     {"yes"},
		"client":  {"mobi"},
		"keyword": {term},
		"hash":    {""},
	}
	if q.Duration > 0 {
		v.Set("duration", strconv.FormatInt(q.Duration.Milliseconds(), 10))
	}
	var out struct {
		Status     int `json:"status"`
		Candidates []struct {
			ID        string `json:"id"`
			AccessKey string `json:"accesskey"`
			Song      string `json:"song"`
			Singer    string `json:"singer"`
			Duration  int64  `json:"duration"`
		} `json:"candidates"`
	}
	if err := getJSON(ctx, k.Client, k.base()+"/search?"+v.Encode(), nil, &out); err != nil {
		return nil, err
	}
	cands := make([]Candidate, 0, len(out.Candidates))
	for _, c := range out.Candidates {
		if c.ID == "" || c.AccessKey == "" {
			continue
		}
		cands = append(cands, Candidate{
			Provider: k.Name(),
			Title:    c.Song,
			Artist:   c.Singer,
			Duration: time.Duration(c.Duration) * time.Millisecond,
			// The key is per candidate, so the ref carries both.
			Ref: c.ID + "\x00" + c.AccessKey,
		})
	}
	return cands, nil
}

// Fetch retrieves lyrics for candidate c.
func (k *Kugou) Fetch(ctx context.Context, c Candidate) (Result, error) {
	if k == nil {
		return Result{}, ErrNotFound
	}
	return k.download(ctx, c.Ref)
}
func (k *Kugou) download(ctx context.Context, ref string) (Result, error) {
	id, key, ok := strings.Cut(ref, "\x00")
	if !ok {
		return Result{}, ErrNotFound
	}
	v := url.Values{
		"ver":       {"1"},
		"client":    {"pc"},
		"id":        {id},
		"accesskey": {key},
		"fmt":       {"lrc"},
		"charset":   {"utf8"},
	}
	var out struct {
		Status  int    `json:"status"`
		Content string `json:"content"`
		Fmt     string `json:"fmt"`
	}
	if err := getJSON(ctx, k.Client, k.downloadBase()+"/download?"+v.Encode(), nil, &out); err != nil {
		return Result{}, err
	}
	if out.Status != 200 || out.Content == "" {
		return Result{}, ErrNotFound
	}
	raw, err := base64.StdEncoding.DecodeString(out.Content)
	if err != nil {
		return Result{}, ErrNotFound
	}
	text := string(raw)
	if text == "" {
		return Result{}, ErrNotFound
	}
	return Result{LRC: text, Source: k.Name()}, nil
}

func (k *Kugou) downloadBase() string {
	if k == nil || k.Download == "" {
		return "https://lyrics.kugou.com"
	}
	return k.Download
}

func (k *Kugou) base() string {
	if k == nil || k.Base == "" {
		return "https://krcs.kugou.com"
	}
	return k.Base
}
