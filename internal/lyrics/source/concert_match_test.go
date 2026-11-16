package source

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestSoulPowerConcertMatchingRegression validates the 4 tracks from Tao Zhe's
// "Soul Power (Live Concert)" album that are currently mismatched to the studio
// album "黑色柳丁" (Black Tangerine).
//
// When listening to the live concert album, automatic matching must select the
// matching Soul Power Live candidate and fetch its live LRC, rather than the studio cut.
func TestSoulPowerConcertMatchingRegression(t *testing.T) {
	type candidateRaw struct {
		id   int64
		name string
		ar   string
		al   string
		dt   int64 // ms
	}

	tests := []struct {
		name         string
		query        Query
		candidates   []candidateRaw
		wantedSongID int64
		wantedLRC    string
		studioSongID int64
		studioLRC    string
	}{
		{
			name: "Track 05: 二十二",
			query: Query{
				Title:    "二十二",
				Artist:   "陶喆",
				Album:    "Soul Power (Live Concert)",
				Duration: 264 * time.Second, // 4m24s
			},
			candidates: []candidateRaw{
				{id: 150565, name: "二十二", ar: "陶喆", al: "黑色柳丁", dt: 261906},
				{id: 518863192, name: "二十二 (Live)", ar: "陶喆", al: "Live Again: 陶喆 小人物狂想曲 香港站", dt: 161245},
				{id: 150454, name: "二十二 (Live)", ar: "陶喆", al: "Soul Power Live 陶喆现场原音专辑", dt: 263586},
			},
			wantedSongID: 150454,
			wantedLRC:    "[00:00.000]二十二 (Soul Power Live)",
			studioSongID: 150565,
			studioLRC:    "[00:00.000]二十二 (黑色柳丁 录音室版)",
		},
		{
			name: "Track 06: 討厭紅樓夢",
			query: Query{
				Title:    "討厭紅樓夢",
				Artist:   "陶喆",
				Album:    "Soul Power (Live Concert)",
				Duration: 242 * time.Second, // 4m02s
			},
			candidates: []candidateRaw{
				{id: 150560, name: "讨厌红楼梦", ar: "陶喆", al: "黑色柳丁", dt: 242373},
				{id: 150456, name: "讨厌红楼梦 (Live)", ar: "陶喆", al: "Soul Power Live 陶喆现场原音专辑", dt: 241120},
				{id: 518863187, name: "讨厌红楼梦 (Live)", ar: "陶喆", al: "Live Again: 陶喆 小人物狂想曲 香港站", dt: 209665},
			},
			wantedSongID: 150456,
			wantedLRC:    "[00:00.000]讨厌红楼梦 (Soul Power Live)",
			studioSongID: 150560,
			studioLRC:    "[00:00.000]讨厌红楼梦 (黑色柳丁 录音室版)",
		},
		{
			name: "Track 12: Angel",
			query: Query{
				Title:    "Angel",
				Artist:   "陶喆",
				Album:    "Soul Power (Live Concert)",
				Duration: 235 * time.Second, // 3m55s
			},
			candidates: []candidateRaw{
				{id: 150558, name: "Angel", ar: "陶喆", al: "黑色柳丁", dt: 232400},
				{id: 150468, name: "Angel (Live)", ar: "陶喆", al: "Soul Power Live 陶喆现场原音专辑", dt: 234186},
				{id: 518863207, name: "Angel (Live)", ar: "陶喆", al: "Live Again: 陶喆 小人物狂想曲 香港站", dt: 166751},
			},
			wantedSongID: 150468,
			wantedLRC:    "[00:00.000]Angel (Soul Power Live)",
			studioSongID: 150558,
			studioLRC:    "[00:00.000]Angel (黑色柳丁 录音室版)",
		},
		{
			name: "Track 18: 月亮代表誰的心",
			query: Query{
				Title:    "月亮代表誰的心",
				Artist:   "陶喆",
				Album:    "Soul Power (Live Concert)",
				Duration: 207 * time.Second, // 3m27s
			},
			candidates: []candidateRaw{
				{id: 150564, name: "月亮代表谁的心", ar: "陶喆", al: "黑色柳丁", dt: 204986},
				{id: 150480, name: "月亮代表谁的心 (Live)", ar: "陶喆", al: "Soul Power Live 陶喆现场原音专辑", dt: 206173},
				{id: 518863198, name: "月亮代表谁的心 (Live)", ar: "陶喆", al: "Live Again: 陶喆 小人物狂想曲 香港站", dt: 145770},
			},
			wantedSongID: 150480,
			wantedLRC:    "[00:00.000]月亮代表谁的心 (Soul Power Live)",
			studioSongID: 150564,
			studioLRC:    "[00:00.000]月亮代表谁的心 (黑色柳丁 录音室版)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var requestedLyricID string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/cloudsearch/pc":
					var songsJSON []string
					for _, c := range tc.candidates {
						songsJSON = append(songsJSON, fmt.Sprintf(
							`{"id":%d,"name":%q,"ar":[{"name":%q}],"al":{"name":%q},"dt":%d}`,
							c.id, c.name, c.ar, c.al, c.dt,
						))
					}
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprintf(w, `{"result":{"songs":[%s]}}`, strings.Join(songsJSON, ","))
				case "/api/song/lyric":
					id := r.URL.Query().Get("id")
					requestedLyricID = id
					switch id {
					case fmt.Sprint(tc.wantedSongID):
						fmt.Fprintf(w, `{"lrc":{"lyric":%q}}`, tc.wantedLRC)
					case fmt.Sprint(tc.studioSongID):
						fmt.Fprintf(w, `{"lrc":{"lyric":%q}}`, tc.studioLRC)
					default:
						http.NotFound(w, r)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			provider := &NetEase{Base: srv.URL, Client: srv.Client()}
			res, err := provider.Find(context.Background(), tc.query)
			if err != nil {
				t.Fatalf("Find failed: %v", err)
			}

			// Regression check: the chosen candidate must be the Soul Power Live track,
			// not the Black Tangerine studio cut.
			if requestedLyricID != fmt.Sprint(tc.wantedSongID) {
				t.Errorf("fetched song ID = %s, want %d (mismatched to studio id %d)",
					requestedLyricID, tc.wantedSongID, tc.studioSongID)
			}
			if !strings.Contains(res.LRC, "(Soul Power Live)") {
				t.Errorf("got LRC %q, want Soul Power Live version", res.LRC)
			}
		})
	}
}

// TestConcertMatchingEdgeCases tests consumer-level edge cases:
// 1. Studio query with studio vs live candidate with identical duration: prefers studio.
// 2. Explicit live query rejects studio-only candidate.
// 3. Different concert years/venues do not match.
// 4. "Live" as an ordinary word in song title is preserved ("Live and Let Die").
// 5. Short duration mismatch (27s fragment) rejected.
// 6. Cross-language live album packaging normalization (English vs Chinese packaging).
func TestConcertMatchingEdgeCases(t *testing.T) {
	t.Run("StudioQueryPrefersStudioOverLive", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":101,"name":"二十二 (Live)","ar":[{"name":"陶喆"}],"al":{"name":"Soul Power Live"},"dt":264000},
					{"id":102,"name":"二十二","ar":[{"name":"陶喆"}],"al":{"name":"黑色柳丁"},"dt":264000}
				]}}`)
			case "/api/song/lyric":
				id := r.URL.Query().Get("id")
				if id == "102" {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]二十二 录音室版"}}`)
				} else {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]二十二 Live版"}}`)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		res, err := p.Find(context.Background(), Query{
			Title:    "二十二",
			Artist:   "陶喆",
			Album:    "黑色柳丁",
			Duration: 264 * time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(res.LRC, "录音室版") {
			t.Fatalf("expected studio version for studio query, got %q", res.LRC)
		}
	})

	t.Run("ExplicitLiveRejectsStudioOnly", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":201,"name":"黑色柳丁","ar":[{"name":"陶喆"}],"al":{"name":"黑色柳丁"},"dt":271000}
				]}}`)
			case "/api/song/lyric":
				fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]黑色柳丁 录音室版"}}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		_, err := p.Find(context.Background(), Query{
			Title:    "黑色柳丁",
			Artist:   "陶喆",
			Album:    "Soul Power (Live Concert)",
			Duration: 271 * time.Second,
		})
		if err != ErrNotFound {
			t.Fatalf("expected ErrNotFound when only studio candidate exists for live query, got %v", err)
		}
	})

	t.Run("DifferentLiveConcertYearsDoNotMatch", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":301,"name":"黑色柳丁 (Live)","ar":[{"name":"陶喆"}],"al":{"name":"2006 演唱会"},"dt":270000}
				]}}`)
			case "/api/song/lyric":
				fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]2006现场"}}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		_, err := p.Find(context.Background(), Query{
			Title:    "黑色柳丁",
			Artist:   "陶喆",
			Album:    "2003 Live 演唱会",
			Duration: 270 * time.Second,
		})
		if err != ErrNotFound {
			t.Fatalf("expected ErrNotFound for different concert years, got %v", err)
		}
	})

	t.Run("LiveAsOrdinaryWordInTitlePreserved", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":351,"name":"Live and Let Die","ar":[{"name":"Paul McCartney"}],"al":{"name":"Live and Let Die"},"dt":193000},
					{"id":352,"name":"Live and Let Die (Live)","ar":[{"name":"Paul McCartney"}],"al":{"name":"Good Evening New York City"},"dt":193000}
				]}}`)
			case "/api/song/lyric":
				id := r.URL.Query().Get("id")
				if id == "351" {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]When you were young and your heart was an open book"}}`)
				} else {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Live version in New York City"}}`)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		res, err := p.Find(context.Background(), Query{
			Title:    "Live and Let Die",
			Artist:   "Paul McCartney",
			Album:    "Live and Let Die",
			Duration: 193 * time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(res.LRC, "When you were young") {
			t.Fatalf("expected studio 'Live and Let Die' to be selected without losing word Live, got %s", res.LRC)
		}
	})

	t.Run("ShortDurationDifferenceRejected", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":401,"name":"天天","ar":[{"name":"陶喆"}],"al":{"name":"陶喆同名专辑"},"dt":27000}
				]}}`)
			case "/api/song/lyric":
				fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]天天 片段"}}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		_, err := p.Find(context.Background(), Query{
			Title:    "天天",
			Artist:   "陶喆",
			Album:    "陶喆同名专辑",
			Duration: 245 * time.Second, // >60s difference against 27s
		})
		if err != ErrNotFound {
			t.Fatalf("expected ErrNotFound for >60s difference, got %v", err)
		}
	})

	t.Run("AlbumOliverIsNotMisclassifiedAsLive", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":501,"name":"Good Song","ar":[{"name":"Artist"}],"al":{"name":"Oliver"},"dt":200000}
				]}}`)
			case "/api/song/lyric":
				fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Good Song from Oliver"}}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		res, err := p.Find(context.Background(), Query{
			Title:    "Good Song",
			Artist:   "Artist",
			Album:    "Oliver",
			Duration: 200 * time.Second,
		})
		if err != nil {
			t.Fatalf("expected studio album Oliver to match successfully without live requirement, got error %v", err)
		}
		if !strings.Contains(res.LRC, "Good Song from Oliver") {
			t.Fatalf("unexpected LRC: %s", res.LRC)
		}
	})

	t.Run("StudioQueryPrefersStudioWhenRemixIsFirst", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				// Candidate 0 is a Remix with same duration; Candidate 1 is normal Studio
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":601,"name":"Song (Remix)","ar":[{"name":"Artist"}],"al":{"name":"Album"},"dt":200000},
					{"id":602,"name":"Song","ar":[{"name":"Artist"}],"al":{"name":"Album"},"dt":200000}
				]}}`)
			case "/api/song/lyric":
				id := r.URL.Query().Get("id")
				if id == "602" {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Studio Version"}}`)
				} else {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Remix Version"}}`)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		res, err := p.Find(context.Background(), Query{
			Title:    "Song",
			Artist:   "Artist",
			Album:    "Album",
			Duration: 200 * time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(res.LRC, "Studio Version") {
			t.Fatalf("expected studio candidate to beat remix for studio query, got %s", res.LRC)
		}
	})

	t.Run("RemixQueryPrefersRemixCandidateOverStudio", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				// Candidate 0 is standard Studio; Candidate 1 is Remix
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":651,"name":"Song","ar":[{"name":"Artist"}],"al":{"name":"Album"},"dt":200000},
					{"id":652,"name":"Song (Remix)","ar":[{"name":"Artist"}],"al":{"name":"Album"},"dt":200000}
				]}}`)
			case "/api/song/lyric":
				id := r.URL.Query().Get("id")
				if id == "652" {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Remix Version"}}`)
				} else {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Studio Version"}}`)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		res, err := p.Find(context.Background(), Query{
			Title:    "Song (Remix)",
			Artist:   "Artist",
			Album:    "Album",
			Duration: 200 * time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(res.LRC, "Remix Version") {
			t.Fatalf("expected remix query to prefer remix candidate, got %s", res.LRC)
		}
	})

	t.Run("OpaqueEventResidueRedRocksVsSeoul", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":751,"name":"Song (Live at Red Rocks)","ar":[{"name":"Artist"}],"al":{"name":"Live 2010"},"dt":250000},
					{"id":752,"name":"Song (Live in Seoul)","ar":[{"name":"Artist"}],"al":{"name":"Live 2010"},"dt":250000}
				]}}`)
			case "/api/song/lyric":
				id := r.URL.Query().Get("id")
				if id == "751" {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Red Rocks Live"}}`)
				} else {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Seoul Live"}}`)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		res, err := p.Find(context.Background(), Query{
			Title:    "Song (Live at Red Rocks)",
			Artist:   "Artist",
			Album:    "Live 2010",
			Duration: 250 * time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(res.LRC, "Red Rocks Live") {
			t.Fatalf("expected Red Rocks event to match specifically, got %s", res.LRC)
		}
	})

	t.Run("DifferentLiveVenuesWithSameDurationAmbiguous", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":701,"name":"Song (Live in Taipei)","ar":[{"name":"Artist"}],"al":{"name":"Live 2003"},"dt":200000},
					{"id":702,"name":"Song (Live in Tokyo)","ar":[{"name":"Artist"}],"al":{"name":"Live 2003"},"dt":200000}
				]}}`)
			case "/api/song/lyric":
				fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Live"}}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		// Query has no venue; both candidates are live with conflicting venues Taipei vs Tokyo
		_, err := p.Find(context.Background(), Query{
			Title:    "Song (Live)",
			Artist:   "Artist",
			Album:    "Live 2003",
			Duration: 200 * time.Second,
		})
		if err != ErrNotFound {
			t.Fatalf("expected ambiguous different venues to result in ErrNotFound, got %v", err)
		}
	})

	t.Run("MatchingVenueSelectedEvenWhenNotFirst", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":801,"name":"Song (Live in Tokyo)","ar":[{"name":"Artist"}],"al":{"name":"Live 2003"},"dt":200000},
					{"id":802,"name":"Song (Live in Taipei)","ar":[{"name":"Artist"}],"al":{"name":"Live 2003"},"dt":200000}
				]}}`)
			case "/api/song/lyric":
				id := r.URL.Query().Get("id")
				if id == "802" {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Taipei Live"}}`)
				} else {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Tokyo Live"}}`)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		res, err := p.Find(context.Background(), Query{
			Title:    "Song (Live in Taipei)",
			Artist:   "Artist",
			Album:    "Live 2003",
			Duration: 200 * time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(res.LRC, "Taipei Live") {
			t.Fatalf("expected Taipei venue to be selected over Tokyo, got %s", res.LRC)
		}
	})
	t.Run("Generalization_Jay2004InTheNameOfFather_PackagingCDStrip", func(t *testing.T) {
		// YTM query: album "2004無與倫比演唱會", duration 360s
		// Candidates: [0] studio (342s), [1] other live (225s), [2] exact live CD (359s)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":1001,"name":"以父之名","ar":[{"name":"周杰伦"}],"al":{"name":"叶惠美"},"dt":342000},
					{"id":1002,"name":"以父之名","ar":[{"name":"周杰伦"}],"al":{"name":"周杰伦地表最强世界巡回演唱会"},"dt":225000},
					{"id":1003,"name":"以父之名","ar":[{"name":"周杰伦"}],"al":{"name":"周杰伦 2004 无与伦比 演唱会 Live CD"},"dt":359000}
				]}}`)
			case "/api/song/lyric":
				id := r.URL.Query().Get("id")
				if id == "1003" {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]以父之名 2004Live"}}`)
				} else {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]其他版本"}}`)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		res, err := p.Find(context.Background(), Query{
			Title:    "以父之名",
			Artist:   "周杰倫",
			Album:    "2004無與倫比演唱會",
			Duration: 360 * time.Second,
		})
		if err != nil {
			t.Fatalf("expected 2004 Live to match, got error %v", err)
		}
		if !strings.Contains(res.LRC, "2004Live") {
			t.Fatalf("expected 2004 Live lyric, got %s", res.LRC)
		}
	})

	t.Run("Generalization_MaydayNowhereBilingualVariant", func(t *testing.T) {
		// YTM query: album "五月天 諾亞方舟 世界巡迴演唱會Live 2CD (MAYDAY NOWHERE World Tour Live 2CD)", duration 268s
		// Candidates: [0] studio (265s), [1] exact live (267s), [2] soundtrack (265s)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":2001,"name":"我不愿 让你一个人","ar":[{"name":"五月天"}],"al":{"name":"第二人生（明日版）"},"dt":265000},
					{"id":2002,"name":"我不愿让你一个人","ar":[{"name":"五月天"}],"al":{"name":"诺亚方舟 世界巡回演唱会"},"dt":267000},
					{"id":2003,"name":"我不愿让你一个人","ar":[{"name":"五月天"}],"al":{"name":"今夜一起为爱鼓掌 电视剧原声带"},"dt":265000}
				]}}`)
			case "/api/song/lyric":
				id := r.URL.Query().Get("id")
				if id == "2002" {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]我不愿让你一个人 诺亚方舟Live"}}`)
				} else {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]其他版本"}}`)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		res, err := p.Find(context.Background(), Query{
			Title:    "我不願讓你一個人",
			Artist:   "五月天",
			Album:    "五月天 諾亞方舟 世界巡迴演唱會Live 2CD (MAYDAY NOWHERE World Tour Live 2CD)",
			Duration: 268 * time.Second,
		})
		if err != nil {
			t.Fatalf("expected Mayday Nowhere live to match, got error %v", err)
		}
		if !strings.Contains(res.LRC, "诺亚方舟Live") {
			t.Fatalf("expected Nowhere Live lyric, got %s", res.LRC)
		}
	})

	t.Run("Generalization_ColdplayLive2012_ExplicitPackaging", func(t *testing.T) {
		// YTM query: album "Live 2012", title "In My Place (Live 2012)", duration 236s
		// Candidates: [0] other live 2005 (240s), [1] studio (226s), [2] exact live 2012 with Explicit (235s)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":3001,"name":"In My Place (Live at Live 8, Hyde Park, London, 2nd July 2005)","ar":[{"name":"Coldplay"}],"al":{"name":"Live 8 (Live, July 2005)"},"dt":240000},
					{"id":3002,"name":"In My Place","ar":[{"name":"Coldplay"}],"al":{"name":"A Rush of Blood to the Head"},"dt":226000},
					{"id":3003,"name":"In My Place","ar":[{"name":"Coldplay"}],"al":{"name":"Live 2012 (Explicit)"},"dt":235000}
				]}}`)
			case "/api/song/lyric":
				id := r.URL.Query().Get("id")
				if id == "3003" {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]In My Place Live 2012"}}`)
				} else {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]其他版本"}}`)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		res, err := p.Find(context.Background(), Query{
			Title:    "In My Place (Live 2012)",
			Artist:   "Coldplay",
			Album:    "Live 2012",
			Duration: 236 * time.Second,
		})
		if err != nil {
			t.Fatalf("expected Live 2012 to match, got error %v", err)
		}
		if !strings.Contains(res.LRC, "Live 2012") {
			t.Fatalf("expected Live 2012 lyric, got %s", res.LRC)
		}
	})
}
