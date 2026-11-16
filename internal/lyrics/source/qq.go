package source

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// QQ looks tracks up in the QQ Music catalogue. Search returns the song's
// mid, which is what the lyric endpoint needs; only the track's metadata is
// sent.
type QQ struct {
	Base   string
	Client *http.Client
}

// NewQQ returns the provider, ready to ask c.y.qq.com.
func NewQQ() *QQ {
	return &QQ{Base: "https://c.y.qq.com", Client: &http.Client{Timeout: 8 * time.Second}}
}

// Name is what the listener sees as the lyrics' source.
func (q *QQ) Name() string { return "QQ音乐" }

// Find searches for the track and fetches the lyrics of the best match.
func (q *QQ) Find(ctx context.Context, query Query) (Result, error) {
	if q == nil {
		return Result{}, ErrNotFound
	}
	cands, err := q.Search(ctx, query)
	if err != nil {
		return Result{}, err
	}
	c, ok := best(query, cands)
	if !ok {
		return Result{}, ErrNotFound
	}
	return q.Fetch(ctx, c)
}

// Search queries QQ Music for candidates matching query.
func (q *QQ) Search(ctx context.Context, query Query) ([]Candidate, error) {
	if q == nil {
		return nil, ErrNotFound
	}
	term := strings.TrimSpace(query.Title + " " + query.Artist)
	if term == "" {
		return nil, nil
	}
	v := url.Values{
		"p":        {"1"},
		"n":        {"10"},
		"w":        {term},
		"format":   {"json"},
		"cr":       {"1"},
		"new_json": {"1"},
	}
	var out struct {
		Data struct {
			Song struct {
				List []struct {
					Mid      string `json:"mid"`
					Name     string `json:"name"`
					Interval int    `json:"interval"`
					Singer   []struct {
						Name  string `json:"name"`
						Title string `json:"title"`
					} `json:"singer"`
					Album struct {
						Name string `json:"name"`
					} `json:"album"`
				} `json:"list"`
			} `json:"song"`
		} `json:"data"`
	}
	if err := getJSON(ctx, q.Client, q.base()+"/soso/fcgi-bin/client_search_cp?"+v.Encode(), map[string]string{"Referer": "https://y.qq.com/"}, &out); err != nil {
		return nil, err
	}
	list := out.Data.Song.List
	cands := make([]Candidate, 0, len(list))
	for _, s := range list {
		artist := ""
		for i, a := range s.Singer {
			if i > 0 {
				artist += "/"
			}
			artist += a.Name
		}
		var aliases []string
		if len(s.Singer) > 0 {
			primary := s.Singer[0]
			// QQ returns format e.g. "陈奕迅 (Eason Chan)" in Singer.Title
			if primary.Title != "" && primary.Name != "" {
				prefix := primary.Name + " ("
				if strings.HasPrefix(primary.Title, prefix) && strings.HasSuffix(primary.Title, ")") {
					alias := strings.TrimSuffix(strings.TrimPrefix(primary.Title, prefix), ")")
					alias = strings.TrimSpace(alias)
					if alias != "" {
						aliases = append(aliases, alias)
					}
				}
			}
		}
		cands = append(cands, Candidate{
			Provider:      q.Name(),
			Title:         s.Name,
			Artist:        artist,
			ArtistAliases: aliases,
			Album:         s.Album.Name,
			Duration:      time.Duration(s.Interval) * time.Second,
			Ref:           s.Mid,
		})
	}
	return cands, nil
}

// Fetch retrieves lyrics for candidate c.
func (q *QQ) Fetch(ctx context.Context, c Candidate) (Result, error) {
	if q == nil {
		return Result{}, ErrNotFound
	}
	return q.lyric(ctx, c.Ref)
}

func (q *QQ) lyric(ctx context.Context, mid string) (Result, error) {
	v := url.Values{"songmid": {mid}, "format": {"json"}, "nobase64": {"1"}, "g_tk": {"5381"}}
	var out struct {
		Retcode int    `json:"retcode"`
		Lyric   string `json:"lyric"`
		Trans   string `json:"trans"`
	}
	if err := getJSON(ctx, q.Client, q.base()+"/lyric/fcgi-bin/fcg_query_lyric_new.fcg?"+v.Encode(), map[string]string{"Referer": "https://y.qq.com/"}, &out); err != nil {
		return Result{}, err
	}
	if out.Retcode != 0 || out.Lyric == "" {
		return Result{}, ErrNotFound
	}
	return Result{LRC: out.Lyric, Translation: out.Trans, Source: q.Name()}, nil
}

func (q *QQ) base() string {
	if q == nil || q.Base == "" {
		return "https://c.y.qq.com"
	}
	return q.Base
}
