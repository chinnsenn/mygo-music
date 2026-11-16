package source

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Kugou search returns a candidate id and its access key, and the download
// endpoint answers with base64 LRC that is decoded into a timed result.
func TestKugouFind(t *testing.T) {
	lrc := "[00:01.00]故事的小黄花\n[00:05.00]从出生那年就飘着\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			if got := r.URL.Query().Get("duration"); got != "269000" {
				t.Errorf("duration = %q, want 269000", got)
			}
			w.Write([]byte(`{"status":200,"candidates":[
				{"id":"7","accesskey":"key-7","song":"晴天","singer":"周杰伦","duration":269000}]}`))
		case "/download":
			if got := r.URL.Query().Get("accesskey"); got != "key-7" {
				t.Errorf("accesskey = %q", got)
			}
			w.Write([]byte(`{"status":200,"fmt":"lrc","content":"` + base64.StdEncoding.EncodeToString([]byte(lrc)) + `"}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	p := &Kugou{Base: srv.URL, Download: srv.URL, Client: srv.Client()}
	res, err := p.Find(context.Background(), Query{Title: "晴天", Artist: "周杰伦", Duration: 269 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.LRC, "[00:01.00]故事的小黄花") || res.Source != "酷狗音乐" {
		t.Fatalf("result = %+v", res)
	}
}

// No candidate for the track is a miss.
func TestKugouNoCandidates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":200,"candidates":[]}`))
	}))
	defer srv.Close()
	p := &Kugou{Base: srv.URL, Download: srv.URL, Client: srv.Client()}
	if _, err := p.Find(context.Background(), Query{Title: "晴天", Artist: "周杰伦"}); err != ErrNotFound {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

// A download that is refused, or is not base64, is a miss rather than text
// shown as lyrics.
func TestKugouBadDownload(t *testing.T) {
	for name, body := range map[string]string{
		"refused": `{"status":403,"content":""}`,
		"garbage": `{"status":200,"fmt":"lrc","content":"not base64!!"}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/search" {
					w.Write([]byte(`{"status":200,"candidates":[{"id":"7","accesskey":"k","song":"晴天","singer":"周杰伦","duration":269000}]}`))
					return
				}
				w.Write([]byte(body))
			}))
			defer srv.Close()
			p := &Kugou{Base: srv.URL, Download: srv.URL, Client: srv.Client()}
			if _, err := p.Find(context.Background(), Query{Title: "晴天", Artist: "周杰伦", Duration: 269 * time.Second}); err != ErrNotFound {
				t.Fatalf("error = %v, want ErrNotFound", err)
			}
		})
	}
}
