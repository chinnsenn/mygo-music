package source

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestGeneralizationRealCases tests the 4 real-world generalization cases identified
// across Jay Chou, Mayday, Coldplay, and Eason Chan.
func TestGeneralizationRealCases(t *testing.T) {
	t.Run("JayChou2004LiveCD_MediaPackaging", func(t *testing.T) {
		// YTM query: album "2004無與倫比演唱會", duration 360s
		// Candidate [2] is exact Live CD (359s) with packaging "Live CD"
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
			t.Fatalf("expected 2004 Live CD candidate to match, got error %v", err)
		}
		if !strings.Contains(res.LRC, "2004Live") {
			t.Fatalf("expected 2004 Live lyric, got %s", res.LRC)
		}
	})

	t.Run("MaydayNowhereBilingualVariant", func(t *testing.T) {
		// YTM query: album "五月天 諾亞方舟 世界巡迴演唱會Live 2CD (MAYDAY NOWHERE World Tour Live 2CD)", duration 268s
		// Candidate [1] is exact live "诺亚方舟 世界巡回演唱会" (267s)
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

	t.Run("ColdplayLive2012_ExplicitPackaging", func(t *testing.T) {
		// YTM query: album "Live 2012", title "In My Place (Live 2012)", duration 236s
		// Candidate [2] is Live 2012 (Explicit) (235s)
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

	t.Run("EasonChanDeclaredAlias_DuoConcert2010", func(t *testing.T) {
		// YTM query: Artist "Eason Chan", Title "今天等我來", Album "Eason Chan Duo Concert 2010", duration 332s
		// Candidate: Artist "陈奕迅" with declared alias ["Eason Chan"], Album "DUO 陈奕迅2010演唱会", duration 331s, id 64421
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":64420,"name":"今天等我来","ar":[{"id":2116,"name":"陈奕迅","alias":["Eason Chan"]}],"al":{"name":"68'29\""},"dt":262000},
					{"id":64421,"name":"今天等我来(Live)","ar":[{"id":2116,"name":"陈奕迅","alias":["Eason Chan"]}],"al":{"name":"DUO 陈奕迅2010演唱会"},"dt":331893}
				]}}`)
			case "/api/song/lyric":
				id := r.URL.Query().Get("id")
				if id == "64421" {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]今天等我来 DUO 2010 Live"}}`)
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
			Title:    "今天等我來",
			Artist:   "Eason Chan",
			Album:    "Eason Chan Duo Concert 2010",
			Duration: 332 * time.Second,
		})
		if err != nil {
			t.Fatalf("expected Eason Chan declared alias to match DUO Live, got error %v", err)
		}
		if !strings.Contains(res.LRC, "DUO 2010 Live") {
			t.Fatalf("expected DUO 2010 Live lyric, got %s", res.LRC)
		}
	})
	t.Run("QQ_EasonChanDeclaredAlias_DuoConcert2010", func(t *testing.T) {
		// YTM query: Artist "Eason Chan", Title "浮誇", Album "Eason Chan Duo Concert 2010", duration 276s
		// Candidate: Singer "陈奕迅" with title "陈奕迅 (Eason Chan)", Album "Eason Chan Duo Concert 2010", duration 275s, mid "000mYRdZ4MvEee"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/soso/fcgi-bin/client_search_cp":
				fmt.Fprint(w, `{"data":{"song":{"list":[
					{"mid":"002ejEdb4KTwBw","name":"浮夸","interval":283,"singer":[{"name":"陈奕迅","title":"陈奕迅 (Eason Chan)"}],"album":{"name":"U 87"}},
					{"mid":"000mYRdZ4MvEee","name":"浮夸","interval":275,"singer":[{"name":"陈奕迅","title":"陈奕迅 (Eason Chan)"}],"album":{"name":"Eason Chan Duo Concert 2010"}}
				]}}}`)
			case "/lyric/fcgi-bin/fcg_query_lyric_new.fcg":
				mid := r.URL.Query().Get("songmid")
				if mid == "000mYRdZ4MvEee" {
					fmt.Fprint(w, `{"lyric":"[00:01.00]浮夸 DUO 2010 Live"}`)
				} else {
					fmt.Fprint(w, `{"lyric":"[00:01.00]浮夸 录音室版"}`)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		q := &QQ{Base: srv.URL, Client: srv.Client()}
		res, err := q.Find(context.Background(), Query{
			Title:    "浮誇",
			Artist:   "Eason Chan",
			Album:    "Eason Chan Duo Concert 2010",
			Duration: 276 * time.Second,
		})
		if err != nil {
			t.Fatalf("expected QQ Eason Chan declared alias to match DUO Live, got error %v", err)
		}
		if !strings.Contains(res.LRC, "DUO 2010 Live") {
			t.Fatalf("expected DUO 2010 Live lyric, got %s", res.LRC)
		}
	})

	t.Run("Negative_ColdplayLive2012RejectsBuenosAiresEvenWithSameDuration", func(t *testing.T) {
		// YTM query: Title "God Put a Smile upon Your Face (Live)", Album "Live 2012", Duration 322s
		// Candidate [0] is from a DIFFERENT concert "Live in Buenos Aires" with identical 322s duration
		// Matching MUST reject this candidate (ErrNotFound) rather than falling back to an unverified different concert.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":1331652288,"name":"God Put a Smile upon Your Face (Live in Buenos Aires)","ar":[{"name":"Coldplay"}],"al":{"name":"Live in Buenos Aires"},"dt":322000}
				]}}`)
			case "/api/song/lyric":
				fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Buenos Aires Live"}}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		_, err := p.Find(context.Background(), Query{
			Title:    "God Put a Smile upon Your Face (Live)",
			Artist:   "Coldplay",
			Album:    "Live 2012",
			Duration: 322 * time.Second,
		})
		if err != ErrNotFound {
			t.Fatalf("expected ErrNotFound when only different concert candidate (Buenos Aires) exists, got %v", err)
		}
	})
	t.Run("Negative_MissingArtistAliasCannotMatch", func(t *testing.T) {
		// Query: Artist "Eason Chan", but candidate has no matching alias (e.g. "Jacky Cheung")
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":8881,"name":"浮夸","ar":[{"id":100,"name":"张学友","alias":["Jacky Cheung"]}],"al":{"name":"DUO 2010"},"dt":276000}
				]}}`)
			case "/api/song/lyric":
				fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]错误歌手"}}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		_, err := p.Find(context.Background(), Query{
			Title:    "浮夸",
			Artist:   "Eason Chan",
			Album:    "Eason Chan Duo Concert 2010",
			Duration: 276 * time.Second,
		})
		if err != ErrNotFound {
			t.Fatalf("expected ErrNotFound for non-matching artist alias, got %v", err)
		}
	})

	t.Run("Negative_CoartistAliasCannotImpersonatePrimary", func(t *testing.T) {
		// Candidate has primary "王菲" and second artist "陈奕迅" (alias "Eason Chan")
		// The primary artist of the candidate is Wang Fei, so query for "Eason Chan" must NOT match
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":8882,"name":"因为爱情","ar":[{"id":99,"name":"王菲","alias":["Faye Wong"]},{"id":2116,"name":"陈奕迅","alias":["Eason Chan"]}],"al":{"name":"因为爱情"},"dt":210000}
				]}}`)
			case "/api/song/lyric":
				fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]因为爱情"}}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		_, err := p.Find(context.Background(), Query{
			Title:    "因为爱情",
			Artist:   "Eason Chan",
			Album:    "因为爱情",
			Duration: 210 * time.Second,
		})
		if err != ErrNotFound {
			t.Fatalf("expected ErrNotFound when Eason Chan is only second artist, got %v", err)
		}
	})

	t.Run("Negative_ArtistAirDoesNotStripStairway", func(t *testing.T) {
		// Artist: "Air", Album: "Stairway to Heaven (Live)"
		// Stripping artist must NOT remove "air" from inside "Stairway"
		cores := albumCoreVariants("Stairway to Heaven (Live)", "Air", "Air", nil)
		if len(cores) == 0 {
			t.Fatal("expected non-empty core")
		}
		if !strings.Contains(cores[0], "stairway") {
			t.Fatalf("artist 'Air' stripped inside 'Stairway': %v", cores[0])
		}
	})

	t.Run("Negative_LiveInstrumentalNotEqualPlainLive", func(t *testing.T) {
		// Query requests standard Live (vocal): Title "Song (Live)"
		// Candidate [0] is Live Instrumental (伴奏); Candidate [1] is Studio
		// Candidate [0] must NOT be accepted as plain Live
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":8883,"name":"Song (Live) (伴奏)","ar":[{"name":"Artist"}],"al":{"name":"Live 2010"},"dt":200000},
					{"id":8884,"name":"Song","ar":[{"name":"Artist"}],"al":{"name":"Studio Album"},"dt":200000}
				]}}`)
			case "/api/song/lyric":
				fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Live Instrumental"}}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		_, err := p.Find(context.Background(), Query{
			Title:    "Song (Live)",
			Artist:   "Artist",
			Album:    "Live 2010",
			Duration: 200 * time.Second,
		})
		// Candidate 0 has instrumental so not plain live; Candidate 1 is studio so rejected for live query -> ErrNotFound
		if err != ErrNotFound {
			t.Fatalf("expected ErrNotFound when only Live Instrumental is available for plain live query, got %v", err)
		}
	})
	t.Run("Review_AlbumOliverLiveDoesNotSubstrMatchOrLive", func(t *testing.T) {
		// Album "Oliver Live" must NOT be corrupted to "or" by stripping internal "live" from "Oliver"
		// If query has album "Oliver Live" and candidate has "Or Live", they MUST NOT match
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":9001,"name":"Track X","ar":[{"name":"Artist"}],"al":{"name":"Or Live"},"dt":200000}
				]}}`)
			case "/api/song/lyric":
				fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Or Live Lyric"}}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		_, err := p.Find(context.Background(), Query{
			Title:    "Track X",
			Artist:   "Artist",
			Album:    "Oliver Live",
			Duration: 200 * time.Second,
		})
		if err != ErrNotFound {
			t.Fatalf("expected ErrNotFound: 'Oliver Live' must not substring match 'Or Live', got %v", err)
		}
	})

	t.Run("Review_AlbumBracketsWithVenueTaipeiVsTokyoRejected", func(t *testing.T) {
		// Album "Horizon Live (Tokyo)" vs "Horizon Live (Taipei)"
		// Bracket splitting must NOT create a shared bare "Horizon" core that bypasses venue conflict
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":9002,"name":"Song","ar":[{"name":"Artist"}],"al":{"name":"Horizon Live (Taipei)"},"dt":200000}
				]}}`)
			case "/api/song/lyric":
				fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Taipei Lyric"}}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		_, err := p.Find(context.Background(), Query{
			Title:    "Song",
			Artist:   "Artist",
			Album:    "Horizon Live (Tokyo)",
			Duration: 200 * time.Second,
		})
		if err != ErrNotFound {
			t.Fatalf("expected ErrNotFound for conflicting venue bracket albums, got %v", err)
		}
	})

	t.Run("Review_BareTitleLiveQueryRejectsCandidateLiveInstrumental", func(t *testing.T) {
		// Query has bare title "Song" (q.kind == 0) and live album "Horizon Live"
		// Candidate has title "Song (Live) (Instrumental)" on same live album
		// Candidate carries non-live edition (Instrumental) which is NOT requested by query, so it MUST be rejected
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":9003,"name":"Song (Live) (Instrumental)","ar":[{"name":"Artist"}],"al":{"name":"Horizon Live"},"dt":200000}
				]}}`)
			case "/api/song/lyric":
				fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Instrumental Live"}}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		_, err := p.Find(context.Background(), Query{
			Title:    "Song",
			Artist:   "Artist",
			Album:    "Horizon Live",
			Duration: 200 * time.Second,
		})
		if err != ErrNotFound {
			t.Fatalf("expected ErrNotFound: bare live query must reject Live Instrumental candidate, got %v", err)
		}
	})

	t.Run("Review_CompoundAliasCannotMatchPrimary", func(t *testing.T) {
		// Candidate primary artist alias is compound "Eason Chan & Other"
		// Query artist is single "Eason Chan". Must NOT match via substring truncation
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":9004,"name":"Song","ar":[{"id":2116,"name":"某歌手","alias":["Eason Chan & Other"]}],"al":{"name":"Duo 2010"},"dt":200000}
				]}}`)
			case "/api/song/lyric":
				fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Compound Alias Lyric"}}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		_, err := p.Find(context.Background(), Query{
			Title:    "Song",
			Artist:   "Eason Chan",
			Album:    "Duo 2010",
			Duration: 200 * time.Second,
		})
		if err != ErrNotFound {
			t.Fatalf("expected ErrNotFound for compound alias 'Eason Chan & Other', got %v", err)
		}
	})
	t.Run("RecallBoundary_ColdplayInMyPlaceRank13", func(t *testing.T) {
		// Demonstrates the real-world recall cutoff issue:
		// The exact same-album Live 2012 candidate (Ref: 26830940) appears at rank 13 in NetEase cloudsearch.
		// The first 10 candidates are studio or different tours.
		// A search limit of 10 cuts it off and causes ErrNotFound.
		// A search limit of 20 successfully recalls and matches candidate 13.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				requestedLimit, err := strconv.Atoi(r.URL.Query().Get("limit"))
				if err != nil || requestedLimit <= 0 {
					http.Error(w, "invalid limit", http.StatusBadRequest)
					return
				}
				maxCount := requestedLimit
				var songs []string
				// 12 candidates: first 12 are studio or Live 8 / other tours
				for i := 1; i <= 12 && i <= maxCount; i++ {
					songs = append(songs, fmt.Sprintf(
						`{"id":%d,"name":"In My Place","ar":[{"name":"Coldplay"}],"al":{"name":"Other Album %d"},"dt":226000}`,
						i, i,
					))
				}
				// Candidate at index 13 (rank 13): exact same album Live 2012
				if maxCount >= 13 {
					songs = append(songs, `{"id":26830940,"name":"In My Place (Live 2012)","ar":[{"name":"Coldplay"}],"al":{"name":"Live 2012"},"dt":235000}`)
				}
				fmt.Fprintf(w, `{"result":{"songs":[%s]}}`, strings.Join(songs, ","))
			case "/api/song/lyric":
				id := r.URL.Query().Get("id")
				if id == "26830940" {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]In My Place Live 2012 at rank 13"}}`)
				} else {
					fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Other album lyric"}}`)
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
			t.Fatalf("expected candidate at rank 13 to be recalled and matched, got error %v", err)
		}
		if !strings.Contains(res.LRC, "rank 13") {
			t.Fatalf("expected candidate 26830940 lyric, got %s", res.LRC)
		}
	})
	// 汉字主标题带地点括号（如东京 vs 台北）不可因汉字主标题被错误拆分成共享 core
	t.Run("Review_HanMainWithVenueBracketTaipeiVsTokyoRejected", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/cloudsearch/pc":
				fmt.Fprint(w, `{"result":{"songs":[
					{"id":9005,"name":"我不愿让你一个人","ar":[{"name":"五月天"}],"al":{"name":"诺亚方舟 Live (Live in Taipei)"},"dt":268000}
				]}}`)
			case "/api/song/lyric":
				fmt.Fprint(w, `{"lrc":{"lyric":"[00:01.00]Taipei Live Lyric"}}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		p := &NetEase{Base: srv.URL, Client: srv.Client()}
		_, err := p.Find(context.Background(), Query{
			Title:    "我不愿让你一个人",
			Artist:   "五月天",
			Album:    "诺亚方舟 Live (Live in Tokyo)",
			Duration: 268 * time.Second,
		})
		if err != ErrNotFound {
			t.Fatalf("expected ErrNotFound for conflicting Han-main venue bracket albums, got %v", err)
		}
	})
}
