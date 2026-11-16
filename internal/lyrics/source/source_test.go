package source

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// YouTube titles carry decorations the catalogues do not, and the match must
// survive them.
func TestNormalizeTitle(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"Wonderwall (Remastered 2011)", "wonderwall"},
		{"YOASOBI「夜に駆ける」Official Music Video", "yoasobi夜に駆ける"},
		{"Lemon - Topic", "lemon"},
		{"告白气球 (Official Video)", "告白气球"},
		{"晴天 feat. 杨瑞代", "晴天"},
		{"夜に駆ける【MV】", "夜に駆ける"},
	} {
		if got := normalizeTitle(c.in); got != c.want {
			t.Errorf("normalizeTitle(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The artist a title was credited to must not make its own title miss.
func TestMatchDropsTitleCredit(t *testing.T) {
	got, ok := best(Query{Title: "YOASOBI「夜に駆ける」Official Music Video", Artist: "YOASOBI", Duration: 261 * time.Second}, []Candidate{
		{Title: "夜に駆ける", Artist: "YOASOBI", Duration: 261 * time.Second},
	})
	if !ok || got.Title != "夜に駆ける" {
		t.Fatalf("best = %+v, %v", got, ok)
	}
}

// A same-titled song by another artist is never the match, whatever its
// duration.
func TestMatchNeedsTheArtist(t *testing.T) {
	if _, ok := best(Query{Title: "晴天", Artist: "周杰伦", Duration: 269 * time.Second}, []Candidate{
		{Title: "晴天", Artist: "RyaVocal", Duration: 269 * time.Second},
	}); ok {
		t.Fatal("matched another artist")
	}
}

// The duration picks the studio version over a live one of the same title.
func TestMatchUsesDuration(t *testing.T) {
	got, ok := best(Query{Title: "晴天", Artist: "周杰伦", Duration: 269 * time.Second}, []Candidate{
		{Title: "晴天", Artist: "周杰伦", Duration: 330 * time.Second},
		{Title: "晴天", Artist: "周杰伦", Duration: 269 * time.Second},
	})
	if !ok || got.Duration != 269*time.Second {
		t.Fatalf("best = %+v, %v", got, ok)
	}
}

// Without a duration, two different versions are ambiguous and no match is
// made, rather than one at random.
func TestMatchRefusesAmbiguousVersions(t *testing.T) {
	if _, ok := best(Query{Title: "晴天", Artist: "周杰伦"}, []Candidate{
		{Title: "晴天", Artist: "周杰伦", Duration: 269 * time.Second},
		{Title: "晴天", Artist: "周杰伦", Duration: 330 * time.Second},
	}); ok {
		t.Fatal("picked one of two versions")
	}
}

type fakeProvider struct {
	name  string
	fn    func(context.Context, Query) (Result, error)
	calls int
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) Find(ctx context.Context, q Query) (Result, error) {
	f.calls++
	return f.fn(ctx, q)
}

// The chain asks its providers in order and stops at the first hit.
func TestChainOrder(t *testing.T) {
	var asked []string
	a := &fakeProvider{name: "a", fn: func(context.Context, Query) (Result, error) {
		asked = append(asked, "a")
		return Result{}, ErrNotFound
	}}
	b := &fakeProvider{name: "b", fn: func(context.Context, Query) (Result, error) {
		asked = append(asked, "b")
		return Result{LRC: "[00:01.00]one", Source: "b"}, nil
	}}
	c := &fakeProvider{name: "c", fn: func(context.Context, Query) (Result, error) {
		asked = append(asked, "c")
		return Result{LRC: "[00:01.00]other", Source: "c"}, nil
	}}
	chain := NewChain(a, b, c)
	q := Query{Title: "晴天", Artist: "周杰伦", Duration: 269 * time.Second}
	res, err := chain.Find(context.Background(), q)
	if err != nil || res.Source != "b" {
		t.Fatalf("Find = %+v, %v", res, err)
	}
	if len(asked) != 2 || asked[0] != "a" || asked[1] != "b" {
		t.Fatalf("asked %v", asked)
	}
}

// A provider that fails is passed over for the next one.
func TestChainSkipsFailure(t *testing.T) {
	bad := &fakeProvider{name: "bad", fn: func(context.Context, Query) (Result, error) {
		return Result{}, errors.New("down")
	}}
	good := &fakeProvider{name: "good", fn: func(context.Context, Query) (Result, error) {
		return Result{LRC: "[00:01.00]one", Source: "good"}, nil
	}}
	if res, err := NewChain(bad, good).Find(context.Background(), Query{Title: "x", Artist: "y"}); err != nil || res.Source != "good" {
		t.Fatalf("Find = %+v, %v", res, err)
	}
}

// A found song is not asked for again, and a miss is remembered too.
func TestChainCachesBothAnswers(t *testing.T) {
	hit := &fakeProvider{name: "hit", fn: func(context.Context, Query) (Result, error) {
		return Result{LRC: "[00:01.00]one", Source: "hit"}, nil
	}}
	chain := NewChain(hit)
	q := Query{Title: "晴天", Artist: "周杰伦", Duration: 269 * time.Second}
	for range 2 {
		if _, err := chain.Find(context.Background(), q); err != nil {
			t.Fatal(err)
		}
	}
	if hit.calls != 1 {
		t.Errorf("asked %d times for a hit, want 1", hit.calls)
	}

	miss := &fakeProvider{name: "miss", fn: func(context.Context, Query) (Result, error) {
		return Result{}, ErrNotFound
	}}
	chain = NewChain(miss)
	q = Query{Title: "没有这首歌", Artist: "谁"}
	for range 2 {
		if _, err := chain.Find(context.Background(), q); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Find error = %v", err)
		}
	}
	if miss.calls != 1 {
		t.Errorf("asked %d times for a miss, want 1", miss.calls)
	}
}

// When a track's metadata has Traditional Chinese (from YouTube Music) and the first
// provider misses, but QQ Music has the simplified candidate, Chain.Find must succeed
// and return QQ's full lyric.
func TestChainFindTraditionalQueryMatchesSimplifiedCandidate(t *testing.T) {
	// Provider A (e.g. NetEase) returns no match for the song
	pA := &fakeProvider{name: "网易云音乐", fn: func(context.Context, Query) (Result, error) {
		return Result{}, ErrNotFound
	}}

	// Real probe HTTP fixture for QQ: Traditional search query -> simplified candidate -> full LRC
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/soso/fcgi-bin/client_search_cp":
			w.Write([]byte(`{"data":{"song":{"list":[
				{"mid":"003GtCtX1R9Fmx","name":"牵挂","interval":243,"singer":[{"name":"曾沛慈"}],"album":{"name":"牵挂"}}
			]}}}`))
		case "/lyric/fcgi-bin/fcg_query_lyric_new.fcg":
			w.Write([]byte(`{"retcode":0,"lyric":"[00:00.00]牵挂 - 曾沛慈\n[03:58.05]OP：亚神音乐娱乐AsiaMuse","trans":""}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	pB := &QQ{Base: srv.URL, Client: srv.Client()}

	chain := NewChain(pA, pB)

	// Case 1: Traditional query with non-zero duration (244s)
	q1 := Query{Title: "牽掛", Artist: "曾沛慈", Duration: 244 * time.Second}
	res1, err := chain.Find(context.Background(), q1)
	if err != nil {
		t.Fatalf("Chain.Find failed on Traditional query (244s): %v", err)
	}
	if res1.Source != "QQ音乐" || !strings.Contains(res1.LRC, "牵挂 - 曾沛慈") {
		t.Fatalf("res1 = %+v, want QQ音乐 result", res1)
	}

	// Case 2: Traditional query with duration 0 (session track.dt == 0 fallback)
	q2 := Query{Title: "牽掛", Artist: "曾沛慈", Duration: 0}
	res2, err := chain.Find(context.Background(), q2)
	if err != nil {
		t.Fatalf("Chain.Find failed on Traditional query (dur 0): %v", err)
	}
	if res2.Source != "QQ音乐" || !strings.Contains(res2.LRC, "牵挂 - 曾沛慈") {
		t.Fatalf("res2 = %+v, want QQ音乐 result", res2)
	}
}

// Reverse case: Simplified query matching a Traditional candidate (and vice-versa for artist and album).
func TestChainFindSimplifiedQueryMatchesTraditionalCandidate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/soso/fcgi-bin/client_search_cp":
			// Candidate metadata has Traditional Chinese
			w.Write([]byte(`{"data":{"song":{"list":[
				{"mid":"trad-1","name":"牽掛","interval":243,"singer":[{"name":"曾沛慈"}],"album":{"name":"牽掛"}}
			]}}}`))
		case "/lyric/fcgi-bin/fcg_query_lyric_new.fcg":
			w.Write([]byte(`{"retcode":0,"lyric":"[00:00.00]牽掛 - 曾沛慈\n[03:58.05]OP","trans":""}`))
		}
	}))
	defer srv.Close()

	p := &QQ{Base: srv.URL, Client: srv.Client()}
	chain := NewChain(p)

	// Simplified query
	q := Query{Title: "牵挂", Artist: "曾沛慈", Album: "牵挂", Duration: 244 * time.Second}
	res, err := chain.Find(context.Background(), q)
	if err != nil {
		t.Fatalf("Chain.Find failed on simplified query: %v", err)
	}
	if res.Source != "QQ音乐" || !strings.Contains(res.LRC, "牽掛 - 曾沛慈") {
		t.Fatalf("res = %+v", res)
	}
}

// Negative verification: short clips (e.g. 27s ringtone fragment) or mismatched artists
// must continue to be rejected by best() duration/artist gating.
func TestChainFindRejectsFragmentAndWrongArtist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/soso/fcgi-bin/client_search_cp":
			w.Write([]byte(`{"data":{"song":{"list":[
				{"mid":"clip","name":"牵挂 (27秒试听版片段)","interval":27,"singer":[{"name":"曾沛慈"}]},
				{"mid":"diff-ar","name":"牵挂","interval":244,"singer":[{"name":"伍佰"}]}
			]}}}`))
		case "/lyric/fcgi-bin/fcg_query_lyric_new.fcg":
			t.Errorf("Fetch should not be called for rejected candidate, but received request for %s", r.URL.RawQuery)
			w.Write([]byte(`{"retcode":0,"lyric":"[00:00.00]invalid lyric that should never be fetched","trans":""}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	p := &QQ{Base: srv.URL, Client: srv.Client()}
	chain := NewChain(p)
	q := Query{Title: "牽掛", Artist: "曾沛慈", Duration: 244 * time.Second}
	_, err := chain.Find(context.Background(), q)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for clip & wrong artist, got %v", err)
	}
}

// A cancelled request is failed, never answered from the cache.
func TestChainCancelledIsNotServedFromCache(t *testing.T) {
	hit := &fakeProvider{name: "hit", fn: func(context.Context, Query) (Result, error) {
		return Result{LRC: "[00:01.00]one", Source: "hit"}, nil
	}}
	chain := NewChain(hit)
	q := Query{Title: "晴天", Artist: "周杰伦"}
	if _, err := chain.Find(context.Background(), q); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := chain.Find(ctx, q); !errors.Is(err, context.Canceled) {
		t.Fatalf("Find error = %v, want context.Canceled", err)
	}
	if hit.calls != 1 {
		t.Errorf("asked %d times, want 1", hit.calls)
	}
}

// A catalogue that was merely down is asked again next time, not remembered
// as a miss.
func TestChainDoesNotCacheTemporaryFailure(t *testing.T) {
	down := errors.New("down")
	p := &fakeProvider{name: "p"}
	p.fn = func(context.Context, Query) (Result, error) {
		if p.calls == 1 {
			return Result{}, down
		}
		return Result{LRC: "[00:01.00]one", Source: "p"}, nil
	}
	chain := NewChain(p)
	q := Query{Title: "晴天", Artist: "周杰伦"}
	if _, err := chain.Find(context.Background(), q); !errors.Is(err, down) {
		t.Fatalf("Find error = %v, want the provider failure", err)
	}
	if res, err := chain.Find(context.Background(), q); err != nil || res.Source != "p" {
		t.Fatalf("Find = %+v, %v", res, err)
	}
	if p.calls != 2 {
		t.Errorf("asked %d times, want 2", p.calls)
	}
}

// Lyric bodies with no readable lines do not stop the chain from asking the
// next source.
func TestChainSkipsUnusableLyrics(t *testing.T) {
	empty := &fakeProvider{name: "empty", fn: func(context.Context, Query) (Result, error) {
		return Result{LRC: "[00:01.00]", Source: "empty"}, nil
	}}
	garbage := &fakeProvider{name: "garbage", fn: func(context.Context, Query) (Result, error) {
		return Result{LRC: "not timed lyrics", Source: "garbage"}, nil
	}}
	good := &fakeProvider{name: "good", fn: func(context.Context, Query) (Result, error) {
		return Result{LRC: "[00:01.00]one", Source: "good"}, nil
	}}
	res, err := NewChain(empty, garbage, good).Find(context.Background(), Query{Title: "x", Artist: "y"})
	if err != nil || res.Source != "good" {
		t.Fatalf("Find = %+v, %v", res, err)
	}
}
