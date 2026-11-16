package source

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"
)

// Configured source priority determines which catalogue is searched first;
// empty slots never create a request.
func TestConfiguredSourceOrder(t *testing.T) {
	var paths []string
	var pathsMu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathsMu.Lock()
		paths = append(paths, r.URL.Path)
		pathsMu.Unlock()
		switch r.URL.Path {
		case "/soso/fcgi-bin/client_search_cp":
			fmt.Fprint(w, `{"data":{"song":{"list":[]}}}`)
		case "/api/cloudsearch/pc":
			fmt.Fprint(w, `{"result":{"songs":[{"id":42,"name":"Song","ar":[{"name":"Artist"}],"dt":180000}]}}`)
		case "/api/song/lyric":
			fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]found"}}`)
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	config := DefaultConfig()
	config.Sources = [3]string{"qq", "netease", ""}
	chain := NewConfiguredChain(config)
	chain.Providers[0].(*QQ).Base = srv.URL
	chain.Providers[1].(*NetEase).Base = srv.URL
	res, err := chain.Find(context.Background(), Query{Title: "Song", Artist: "Artist", Duration: 3 * time.Minute})
	if err != nil || res.LRC != "[00:01.00]found" {
		t.Fatalf("Find = %+v, %v", res, err)
	}
	pathsMu.Lock()
	defer pathsMu.Unlock()
	want := []string{"/soso/fcgi-bin/client_search_cp", "/api/cloudsearch/pc", "/api/song/lyric"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("requested %v, want %v", paths, want)
	}
}

// A slow first catalogue times out without spending the next one's chance to
// provide lyrics inside the overall lookup deadline.
func TestConfiguredRequestTimeoutFallsThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cloudsearch/pc":
			<-r.Context().Done()
		case "/soso/fcgi-bin/client_search_cp":
			fmt.Fprint(w, `{"data":{"song":{"list":[{"mid":"found","name":"Song","interval":180,"singer":[{"name":"Artist"}]}]}}}`)
		case "/lyric/fcgi-bin/fcg_query_lyric_new.fcg":
			fmt.Fprint(w, `{"retcode":0,"lyric":"[00:01.00]next catalogue"}`)
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	config := DefaultConfig()
	config.Sources = [3]string{"netease", "qq", ""}
	config.RequestTimeoutSeconds = 1
	config.TimeoutSeconds = 5
	chain := NewConfiguredChain(config)
	chain.Providers[0].(*NetEase).Base = srv.URL
	chain.Providers[1].(*QQ).Base = srv.URL
	res, err := chain.Find(context.Background(), Query{Title: "Song", Artist: "Artist", Duration: 3 * time.Minute})
	if err != nil || res.LRC != "[00:01.00]next catalogue" {
		t.Fatalf("Find = %+v, %v", res, err)
	}
}

// The overall limit ends a stalled lookup before a longer per-request limit.
func TestConfiguredLookupDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	config := DefaultConfig()
	config.Sources = [3]string{"netease", "", ""}
	config.TimeoutSeconds, config.RequestTimeoutSeconds = 1, 3
	chain := NewConfiguredChain(config)
	chain.Providers[0].(*NetEase).Base = srv.URL
	_, err := chain.Find(context.Background(), Query{Title: "Song", Artist: "Artist"})
	if err != context.DeadlineExceeded {
		t.Fatalf("lookup returned %v, want the overall deadline", err)
	}
}

// Cache lifetimes from settings expire hits and misses at the selected limit.
func TestConfiguredCacheLifetimes(t *testing.T) {
	for _, tc := range []struct {
		name string
		miss bool
		age  time.Duration
	}{
		{"hit", false, time.Hour},
		{"miss", true, 5 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultConfig()
			config.CacheHitHours, config.CacheMissMinutes = 1, 5
			chain := NewConfiguredChain(config)
			now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			chain.clock = func() time.Time { return now }
			p := &fakeProvider{name: "catalogue", fn: func(context.Context, Query) (Result, error) {
				if tc.miss {
					return Result{}, ErrNotFound
				}
				return Result{LRC: "[00:01.00]cached"}, nil
			}}
			chain.Providers = []Provider{p}
			q := Query{Title: "Song", Artist: "Artist"}
			for range 2 {
				_, err := chain.Find(context.Background(), q)
				if tc.miss != errors.Is(err, ErrNotFound) || err != nil && !tc.miss {
					t.Fatalf("unexpected lookup error: %v", err)
				}
			}
			if p.calls != 1 {
				t.Fatalf("cache did not keep the answer: %d calls", p.calls)
			}
			now = now.Add(tc.age)
			chain.Find(context.Background(), q)
			if p.calls != 2 {
				t.Fatalf("expired answer still cached: %d calls", p.calls)
			}
		})
	}
}

// Turning a cache lifetime or the capacity off really queries again, and the
// chosen capacity bounds how many distinct answers can be retained.
func TestConfiguredCacheDisableAndCapacity(t *testing.T) {
	for _, name := range []string{"hit off", "miss off", "capacity off", "capacity one"} {
		t.Run(name, func(t *testing.T) {
			config := DefaultConfig()
			switch name {
			case "hit off":
				config.CacheHitHours = 0
			case "miss off":
				config.CacheMissMinutes = 0
			case "capacity off":
				config.CacheEntries = 0
			case "capacity one":
				config.CacheEntries = 1
			}
			chain := NewConfiguredChain(config)
			p := &fakeProvider{name: "catalogue", fn: func(context.Context, Query) (Result, error) {
				if name == "miss off" {
					return Result{}, ErrNotFound
				}
				return Result{LRC: "[00:01.00]found"}, nil
			}}
			chain.Providers = []Provider{p}
			first := Query{Title: "One", Artist: "Artist"}
			chain.Find(context.Background(), first)
			want := 2
			if name == "capacity one" {
				chain.Find(context.Background(), Query{Title: "Two", Artist: "Artist"})
				want = 3
			}
			chain.Find(context.Background(), first)
			if p.calls != want {
				t.Fatalf("provider called %d times, want %d", p.calls, want)
			}
		})
	}
}

// Clearing remembered hits and misses lets the next lookup ask the catalogue.
func TestClearConfiguredCache(t *testing.T) {
	for _, miss := range []bool{false, true} {
		chain := NewConfiguredChain(DefaultConfig())
		p := &fakeProvider{name: "catalogue", fn: func(context.Context, Query) (Result, error) {
			if miss {
				return Result{}, ErrNotFound
			}
			return Result{LRC: "[00:01.00]found"}, nil
		}}
		chain.Providers = []Provider{p}
		q := Query{Title: "Song", Artist: "Artist"}
		chain.Find(context.Background(), q)
		chain.Find(context.Background(), q)
		chain.Clear()
		chain.Find(context.Background(), q)
		if p.calls != 2 {
			t.Fatalf("miss=%v: provider called %d times, want 2", miss, p.calls)
		}
	}
}
