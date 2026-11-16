package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The QQ search gives the song's mid, which the lyric endpoint is then asked
// for with the referer it requires.
func TestQQFind(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Referer"); got != "https://y.qq.com/" {
			t.Errorf("referer = %q", got)
		}
		switch r.URL.Path {
		case "/soso/fcgi-bin/client_search_cp":
			w.Write([]byte(`{"data":{"song":{"list":[
				{"mid":"aaa","name":"晴天","interval":269,"singer":[{"name":"翻唱歌手"}],"album":{"name":"翻唱"}},
				{"mid":"bbb","name":"晴天","interval":269,"singer":[{"name":"周杰伦"}],"album":{"name":"叶惠美"}}]}}}`))
		case "/lyric/fcgi-bin/fcg_query_lyric_new.fcg":
			if got := r.URL.Query().Get("songmid"); got != "bbb" {
				t.Errorf("lyric mid = %q, want the matched song", got)
			}
			w.Write([]byte(`{"retcode":0,"lyric":"[00:01.00]故事的小黄花","trans":""}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	p := &QQ{Base: srv.URL, Client: srv.Client()}
	res, err := p.Find(context.Background(), Query{Title: "晴天", Artist: "周杰伦", Duration: 269 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.LRC, "故事的小黄花") || res.Source != "QQ音乐" {
		t.Fatalf("result = %+v", res)
	}
}

// A lyric answer with a refusal code is a miss.
func TestQQRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "client_search_cp") {
			w.Write([]byte(`{"data":{"song":{"list":[{"mid":"bbb","name":"晴天","interval":269,"singer":[{"name":"周杰伦"}],"album":{"name":"叶惠美"}}]}}}`))
			return
		}
		w.Write([]byte(`{"retcode":-1901,"lyric":""}`))
	}))
	defer srv.Close()
	p := &QQ{Base: srv.URL, Client: srv.Client()}
	if _, err := p.Find(context.Background(), Query{Title: "晴天", Artist: "周杰伦", Duration: 269 * time.Second}); err != ErrNotFound {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

// An HTML page where JSON was expected is an error, so the next source is
// tried rather than trusted.
func TestQQMalformed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>blocked</body></html>`))
	}))
	defer srv.Close()
	p := &QQ{Base: srv.URL, Client: srv.Client()}
	if _, err := p.Find(context.Background(), Query{Title: "晴天", Artist: "周杰伦"}); err == nil || err == ErrNotFound {
		t.Fatalf("error = %v, want a decode error", err)
	}
}

// When the query title or artist is Traditional Chinese (e.g. from YouTube Music,
// such as 牽掛 / 曾沛慈), QQ Music's search responds with simplified metadata (牵挂 / 曾沛慈).
// Find must canonically match the candidate and fetch its full lyric instead of
// failing with ErrNotFound.
func TestQQFindTraditionalQueryMatchesSimplifiedCandidate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/soso/fcgi-bin/client_search_cp":
			// Real probe case: QQ responds with simplified title and album for 曾沛慈
			w.Write([]byte(`{"data":{"song":{"list":[
				{"mid":"003GtCtX1R9Fmx","name":"牵挂","interval":243,"singer":[{"name":"曾沛慈"}],"album":{"name":"牵挂"}}
			]}}}`))
		case "/lyric/fcgi-bin/fcg_query_lyric_new.fcg":
			if got := r.URL.Query().Get("songmid"); got != "003GtCtX1R9Fmx" {
				t.Errorf("songmid = %q, want 003GtCtX1R9Fmx", got)
			}
			w.Write([]byte(`{"retcode":0,"lyric":"[00:00.00]牵挂 - 曾沛慈\n[00:07.82]词Lyricist：葛大为/徐佳莹\n[03:58.05]OP：亚神音乐娱乐AsiaMuse","trans":""}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	p := &QQ{Base: srv.URL, Client: srv.Client()}
	// Query carries Traditional Chinese title from YouTube Music metadata: 牽掛
	res, err := p.Find(context.Background(), Query{Title: "牽掛", Artist: "曾沛慈", Duration: 244 * time.Second})
	if err != nil {
		t.Fatalf("Find failed on Traditional query: %v", err)
	}
	if !strings.Contains(res.LRC, "牵挂 - 曾沛慈") || res.Source != "QQ音乐" {
		t.Fatalf("result = %+v", res)
	}
}
