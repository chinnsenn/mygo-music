package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The NetEase search and lyric endpoints are read by their JSON shapes, and
// the referer the catalogue expects is sent.
func TestNetEaseFind(t *testing.T) {
	var referer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cloudsearch/pc":
			referer = r.Header.Get("Referer")
			if got := r.URL.Query().Get("s"); !strings.Contains(got, "晴天") {
				t.Errorf("searched %q", got)
			}
			w.Write([]byte(`{"result":{"songs":[
				{"id":1,"name":"晴天","ar":[{"name":"RyaVocal"}],"al":{"name":"翻唱"},"dt":269000},
				{"id":42,"name":"晴天","ar":[{"name":"周杰伦"}],"al":{"name":"叶惠美"},"dt":269000}]}}`))
		case "/api/song/lyric":
			if got := r.URL.Query().Get("id"); got != "42" {
				t.Errorf("lyric id = %q, want the matched song", got)
			}
			w.Write([]byte(`{"lrc":{"lyric":"[00:01.00]故事的小黄花"},"tlyric":{"lyric":"[00:01.00]the little yellow flower"}}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	p := &NetEase{Base: srv.URL, Client: srv.Client()}
	res, err := p.Find(context.Background(), Query{Title: "晴天", Artist: "周杰伦", Duration: 269 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.LRC, "故事的小黄花") || !strings.Contains(res.Translation, "little yellow") {
		t.Fatalf("result = %+v", res)
	}
	if res.Source != "网易云音乐" {
		t.Errorf("source = %q", res.Source)
	}
	if referer != srv.URL+"/" {
		t.Errorf("referer = %q, want %q", referer, srv.URL+"/")
	}
}

// A search with no song of that artist is a miss, not an error.
func TestNetEaseMiss(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"result":{"songs":[]}}`))
	}))
	defer srv.Close()
	p := &NetEase{Base: srv.URL, Client: srv.Client()}
	if _, err := p.Find(context.Background(), Query{Title: "晴天", Artist: "周杰伦"}); err != ErrNotFound {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

// An empty lyric body, a broken body and a server error all stay errors the
// chain can tell apart from a match.
func TestNetEaseBadAnswers(t *testing.T) {
	search := `{"result":{"songs":[{"id":42,"name":"晴天","ar":[{"name":"周杰伦"}],"al":{"name":"叶惠美"},"dt":269000}]}}`
	for name, c := range map[string]struct {
		lyric  string
		status int
		want   error
	}{
		"empty":     {`{"lrc":{"lyric":""}}`, 200, ErrNotFound},
		"malformed": {`<html>changed</html>`, 200, nil},
		"failed":    {``, 500, nil},
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/cloudsearch/pc" {
					w.Write([]byte(search))
					return
				}
				w.WriteHeader(c.status)
				w.Write([]byte(c.lyric))
			}))
			defer srv.Close()
			p := &NetEase{Base: srv.URL, Client: srv.Client()}
			_, err := p.Find(context.Background(), Query{Title: "晴天", Artist: "周杰伦", Duration: 269 * time.Second})
			if c.want != nil && err != c.want {
				t.Fatalf("error = %v, want %v", err, c.want)
			}
			if c.want == nil && (err == nil || err == ErrNotFound) {
				t.Fatalf("error = %v, want a request or decode error", err)
			}
		})
	}
}
