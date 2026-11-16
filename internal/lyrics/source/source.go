/**
 * [INPUT]: 依赖 context、http、sync、time 与歌词解析器，配置由 config.go 构建后固定到每条链
 * [OUTPUT]: 提供目录查询契约、顺序查询 Chain 及 Clear；按配置控制命中/未命中缓存与容量
 * [POS]: internal/lyrics/source 的对外统一入口与调度聚合层，协调多源检索与歌词获取；只有真正 ErrNotFound 才负缓存，取消优先于缓存
 * [PROTOCOL]: 变更时更新此头部，然后检查 AGENTS.md
 */
package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"mygo-music/internal/lyrics"
)

// Query is what a lookup knows about the track. Duration is zero when the
// response that produced the track did not carry it.
type Query struct {
	Title    string
	Artist   string
	Album    string
	Duration time.Duration
}

// Result is one track's lyrics from one source. LRC carries line timings;
// Plain is the untimed text, used only when LRC is empty.
type Result struct {
	LRC         string
	Translation string
	Plain       string
	Source      string
}

// ErrNotFound reports that no source had the track.
var ErrNotFound = errors.New("lyrics: no lyrics found")

// Provider looks one track up in one catalogue.
type Provider interface {
	// Name is what the listener sees as the lyrics' source.
	Name() string
	// Find returns ErrNotFound when the catalogue has no match.
	Find(ctx context.Context, q Query) (Result, error)
}

// Chain asks its providers in order and keeps what they found, so replaying
// a track does not ask every catalogue again.
type Chain struct {
	Providers  []Provider
	Timeout    time.Duration
	hitTTL     time.Duration
	missTTL    time.Duration
	maxEntries int

	mu    sync.Mutex
	clock func() time.Time
	cache map[string]entry
}

type entry struct {
	res Result
	err error
	exp time.Time
}

const (
	cacheHit  = 6 * time.Hour
	cacheMiss = 30 * time.Minute
	cacheMax  = 512
)

// NewChain returns a chain that asks providers in the order given.
func NewChain(providers ...Provider) *Chain {
	return &Chain{Providers: providers, Timeout: 12 * time.Second, hitTTL: cacheHit, missTTL: cacheMiss, maxEntries: cacheMax, clock: time.Now}
}

// Find returns the first provider's result, from the cache when it was asked
// recently, even for a miss. A cancelled request is answered with its
// cancellation before anything else, and a provider that fails for a reason
// other than ErrNotFound keeps the chain from remembering a miss.
func (c *Chain) Find(ctx context.Context, q Query) (Result, error) {
	if c == nil || len(c.Providers) == 0 {
		return Result{}, ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	key := ""
	if c.maxEntries > 0 && (c.hitTTL > 0 || c.missTTL > 0) {
		key = cacheKey(q)
		if res, err, ok := c.lookup(key); ok {
			return res, err
		}
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 12 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var failure error
	for _, p := range c.Providers {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if p == nil {
			continue
		}
		res, err := p.Find(ctx, q)
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if err == nil && usable(res) {
			c.keep(key, res, nil, c.hitTTL)
			return res, nil
		}
		if err != nil && !errors.Is(err, ErrNotFound) && failure == nil {
			failure = err
		}
	}
	if failure != nil {
		// A catalogue that was merely unreachable has not told us the track is
		// absent, so this answer must not become a remembered miss.
		return Result{}, failure
	}
	c.keep(key, Result{}, ErrNotFound, c.missTTL)
	return Result{}, ErrNotFound
}

// usable reports whether a provider's answer can be shown: timed lyrics the
// parser can turn into lines, or untimed text. Blank and unparseable LRC is
// passed over so it cannot stop the chain.
func usable(res Result) bool {
	if res.LRC != "" && len(lyrics.Build(res.LRC, "")) > 0 {
		return true
	}
	return strings.TrimSpace(res.Plain) != ""
}

// now is the clock the cache reads, for a chain built without NewChain.
func (c *Chain) now() time.Time {
	if c.clock != nil {
		return c.clock()
	}
	return time.Now()
}

func (c *Chain) lookup(key string) (Result, error, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	e, ok := c.cache[key]
	if !ok || !now.Before(e.exp) {
		if ok {
			delete(c.cache, key)
		}
		return Result{}, nil, false
	}
	return e.res, e.err, true
}

func (c *Chain) keep(key string, res Result, err error, ttl time.Duration) {
	if ttl <= 0 || c.maxEntries <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if c.cache == nil {
		c.cache = map[string]entry{}
	}
	if len(c.cache) >= c.maxEntries {
		for k, e := range c.cache {
			if !now.Before(e.exp) {
				delete(c.cache, k)
			}
		}
		if len(c.cache) >= c.maxEntries {
			clear(c.cache)
		}
	}
	c.cache[key] = entry{res: res, err: err, exp: now.Add(ttl)}
}

// Clear forgets catalogue answers without changing the chain's configuration.
func (c *Chain) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.cache)
}

func cacheKey(q Query) string {
	return strings.Join([]string{
		normalizeTitle(q.Title),
		normalizeArtist(q.Artist),
		normalizeText(q.Album),
		fmt.Sprint(q.Duration / time.Second),
	}, "\x00")
}

// Candidate is one search result of a provider, before its lyrics are fetched.
type Candidate struct {
	Provider      string
	Title         string
	Artist        string
	ArtistAliases []string
	Album         string
	Duration      time.Duration
	Ref           string
}

// Searcher extends Provider for sources that support searching candidates
// directly and fetching lyrics for a chosen candidate.
type Searcher interface {
	Provider
	Search(ctx context.Context, q Query) ([]Candidate, error)
	Fetch(ctx context.Context, c Candidate) (Result, error)
}

// best picks the candidate that matches q best, or reports none when nothing
// is close enough. A candidate must share the artist, so a same-titled song by
// someone else is never used.
func best(q Query, cands []Candidate) (Candidate, bool) {
	top, topTier, topScore, tied := Candidate{}, 0, 0, false
	var topMeta trackMeta

	qM := parseQueryMeta(q)
	qArtist := normalizeArtist(q.Artist)
	if qM.cleanTitle == "" || qArtist == "" {
		return Candidate{}, false
	}
	for _, c := range cands {
		if !artistMatches(q.Artist, c.Artist, c.ArtistAliases) {
			continue
		}
		cM := parseCandidateMeta(c, q.Artist)
		tier := candidateTier(qM, cM)
		if tier == 0 {
			continue
		}

		s := titleScore(qM.cleanTitle, cM.cleanTitle)
		if s == 0 {
			continue
		}
		s += 2
		// Album bonus: if core albums match and are non-empty
		if len(qM.albumCores) > 0 && len(cM.albumCores) > 0 && coresIntersect(qM.albumCores, cM.albumCores) {
			s++
		}
		if q.Duration > 0 && c.Duration > 0 {
			switch d := abs(q.Duration - c.Duration); {
			case d > time.Minute:
				continue
			case d <= 2*time.Second:
				s += 3
			case d <= 5*time.Second:
				s++
			}
		}
		if s < 4 {
			continue
		}
		switch {
		case tier > topTier:
			top, topMeta, topTier, topScore, tied = c, cM, tier, s, false
		case tier == topTier:
			switch {
			case s > topScore:
				top, topMeta, topTier, topScore, tied = c, cM, tier, s, false
			case s == topScore:
				if q.Duration == 0 && c.Duration != top.Duration {
					tied = true
				} else if qM.isLive && hasDistinctConcertIdentity(topMeta, cM) {
					// Equal tier and score between two distinct concert identities is ambiguous
					tied = true
				}
			}
		}
	}
	if topScore < 4 || tied {
		return Candidate{}, false
	}
	return top, true
}

// trimArtist drops a credit that a catalogue put in front of the title, as
// in "YOASOBI「夜に駆ける」" or "Oasis - Wonderwall".
func trimArtist(title, artist string) string {
	a := normalizeArtist(artist)
	if a != "" && strings.HasPrefix(title, a) {
		return strings.TrimPrefix(title, a)
	}
	return title
}

// titleScore weighs an exact title over one that only contains the other.
func titleScore(q, c string) int {
	switch {
	case c == "":
		return 0
	case c == q:
		return 3
	case strings.Contains(c, q), strings.Contains(q, c):
		return 1
	}
	return 0
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// getJSON sends one GET and decodes its JSON body, refusing anything that is
// not a success: catalogues answer with HTML pages when they change.
func getJSON(ctx context.Context, client *http.Client, url string, header map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("lyrics: %s: %s", url, res.Status)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("lyrics: %s: %w", url, err)
	}
	return nil
}

// UserAgent identifies the app to the catalogues it asks.
const UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"
