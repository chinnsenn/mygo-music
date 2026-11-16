package player

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mygo-music/internal/lyrics"
	"mygo-music/internal/lyrics/source"
	"mygo-music/internal/music"
)

// TestMain keeps the tests off the catalogues: New would otherwise build the
// real chain once the fallback is turned on, and a fake source with no lyrics
// would ask the network.
func TestMain(m *testing.M) {
	newLyricsProvider = func(source.Config) lyricsProvider { return nil }
	os.Exit(m.Run())
}

// swapLyricsProvider replaces the chain factory for one test.
func swapLyricsProvider(f func(source.Config) lyricsProvider) func() {
	was := newLyricsProvider
	newLyricsProvider = f
	return func() { newLyricsProvider = was }
}

// lyricSource is a source whose lyrics capability and answer a test sets.
type lyricSource struct {
	nothing
	can   bool
	got   *music.Lyrics
	err   error
	calls atomic.Int32
}

func (s *lyricSource) Can(what string) bool { return s.can && what == "lyrics" }

func (s *lyricSource) Lyrics(string) (*music.Lyrics, error) {
	s.calls.Add(1)
	return s.got, s.err
}

// fakeLyricsProvider stands in for the catalogue chain.
type fakeLyricsProvider struct {
	mu      sync.Mutex
	queries []source.Query
	res     source.Result
	err     error
	clears  atomic.Int32

	block   bool
	entered chan struct{}
	release chan struct{}
	done    chan struct{}
}

func (f *fakeLyricsProvider) Find(ctx context.Context, q source.Query) (source.Result, error) {
	f.mu.Lock()
	f.queries = append(f.queries, q)
	f.mu.Unlock()
	if f.entered != nil {
		f.entered <- struct{}{}
	}
	if f.block {
		select {
		case <-ctx.Done():
			if f.done != nil {
				close(f.done)
			}
			return source.Result{}, ctx.Err()
		case <-f.release:
		}
	}
	return f.res, f.err
}

func (f *fakeLyricsProvider) asked() []source.Query {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]source.Query(nil), f.queries...)
}

func (f *fakeLyricsProvider) Clear() { f.clears.Add(1) }

func lyricsPlayer() (*Player, chan func()) {
	ch := make(chan func(), 16)
	p := New(func(fn func()) { ch <- fn })
	p.SetLyricFallback(true) // the completion scenarios these tests check
	return p, ch
}

func awaitLyrics(t *testing.T, p *Player, ch chan func()) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for !p.lyricsLoaded {
		select {
		case fn := <-ch:
			fn()
		case <-deadline:
			t.Fatal("lyrics never loaded")
		}
	}
}

func awaitLine(t *testing.T, p *Player, ch chan func(), want string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		lines, loaded := p.Lyrics()
		if loaded && len(lines) == 1 && lines[0].Text == want {
			return
		}
		select {
		case fn := <-ch:
			fn()
		case <-deadline:
			t.Fatalf("lyrics %q never loaded", want)
		}
	}
}

func track() music.Track {
	return music.Track{
		ID:       "t1",
		Name:     "Song",
		Artists:  []music.Artist{{Name: "First"}, {Name: "Second"}},
		Album:    music.Album{Name: "Album"},
		Duration: 123000,
	}
}

// Lyrics the source has win, and the catalogues are never asked.
func TestSourceLyricsWin(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	src := &lyricSource{can: true, got: &music.Lyrics{LRC: "[00:01.00]own"}}
	prov := &fakeLyricsProvider{res: source.Result{LRC: "[00:02.00]found"}}
	p.lyricsLookup = prov

	p.loadLyrics(0, src, track())
	awaitLyrics(t, p, ch)

	lines, _ := p.Lyrics()
	if len(lines) != 1 || lines[0].Text != "own" {
		t.Fatalf("lines %+v", lines)
	}
	if n := len(prov.asked()); n != 0 {
		t.Errorf("asked the catalogues %d times", n)
	}
}

// Lines timed word by word win over both an LRC and the catalogues.
func TestSourceWordLinesWin(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	src := &lyricSource{can: true, got: &music.Lyrics{
		LRC: "[00:09.00]plain lrc",
		Lines: []music.LyricLine{{
			Start: 0, End: 1000, Text: "ab",
			Words: []music.LyricWord{{Start: 0, End: 500, Text: "a"}, {Start: 500, End: 1000, Text: "b"}},
		}},
	}}
	prov := &fakeLyricsProvider{res: source.Result{LRC: "[00:02.00]found"}}
	p.lyricsLookup = prov

	p.loadLyrics(0, src, track())
	awaitLyrics(t, p, ch)

	lines, _ := p.Lyrics()
	if len(lines) != 1 || lines[0].Text != "ab" || len(lines[0].Words) != 2 || lines[0].Words[1].Text != "b" {
		t.Fatalf("lines %+v", lines)
	}
	if n := len(prov.asked()); n != 0 {
		t.Errorf("asked the catalogues %d times", n)
	}
}

// A source without lyrics, with no answer, with nothing timed, or with an
// error falls back to the catalogues.
func TestCatalogueFillsWhatSourceLacks(t *testing.T) {
	for _, c := range []struct {
		name string
		src  music.Source
	}{
		{"no capability", &lyricSource{can: false}},
		{"nil lyrics", &lyricSource{can: true}},
		{"empty lyrics", &lyricSource{can: true, got: &music.Lyrics{}}},
		{"nothing timed", &lyricSource{can: true, got: &music.Lyrics{LRC: "no timings here"}}},
		{"error", &lyricSource{can: true, err: errors.New("nope")}},
	} {
		t.Run(c.name, func(t *testing.T) {
			p, ch := lyricsPlayer()
			defer p.stop()
			prov := &fakeLyricsProvider{res: source.Result{LRC: "[00:02.00]found"}}
			p.lyricsLookup = prov

			p.loadLyrics(0, c.src, track())
			awaitLyrics(t, p, ch)

			lines, _ := p.Lyrics()
			if len(lines) != 1 || lines[0].Text != "found" {
				t.Fatalf("lines %+v", lines)
			}
			if n := len(prov.asked()); n != 1 {
				t.Errorf("asked the catalogues %d times", n)
			}
		})
	}
}

// The catalogues are asked by name, first artist, album and length.
func TestCatalogueQueryUsesTrackMetadata(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	prov := &fakeLyricsProvider{res: source.Result{LRC: "[00:02.00]found"}}
	p.lyricsLookup = prov

	p.loadLyrics(0, &lyricSource{}, track())
	awaitLyrics(t, p, ch)

	asked := prov.asked()
	if len(asked) != 1 {
		t.Fatalf("asked %d times", len(asked))
	}
	q := asked[0]
	if q.Title != "Song" || q.Artist != "First" || q.Album != "Album" || q.Duration != 123*time.Second {
		t.Errorf("query %+v", q)
	}
}

// A failed lookup still finishes loading: the view shows no lyrics, not a
// spinner forever.
func TestCatalogueFailureLoadsEmpty(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	p.lyricsLookup = &fakeLyricsProvider{err: errors.New("down")}

	p.loadLyrics(0, &lyricSource{}, track())
	awaitLyrics(t, p, ch)

	lines, loaded := p.Lyrics()
	if !loaded || len(lines) != 0 {
		t.Errorf("loaded %v, lines %+v", loaded, lines)
	}
}

// A result for a track no longer current is dropped.
func TestStaleCatalogueResultDropped(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	entered, release := make(chan struct{}, 1), make(chan struct{})
	prov := &fakeLyricsProvider{
		block: true, entered: entered, release: release,
		res: source.Result{LRC: "[00:02.00]stale"},
	}
	p.lyricsLookup = prov

	p.loadLyrics(0, &lyricSource{}, track())
	<-entered
	p.gen++ // the next track started meanwhile
	close(release)

	select {
	case fn := <-ch:
		fn()
	case <-time.After(2 * time.Second):
		t.Fatal("nothing posted")
	}
	if p.lyricsLoaded {
		t.Error("showed a stale track's lyrics")
	}
}

// Stopping cancels the lookup in flight, and nothing is shown for it.
func TestStopCancelsCatalogue(t *testing.T) {
	p, ch := lyricsPlayer()
	entered, done := make(chan struct{}, 1), make(chan struct{})
	prov := &fakeLyricsProvider{block: true, entered: entered, done: done}
	p.lyricsLookup = prov

	p.loadLyrics(0, &lyricSource{}, track())
	<-entered
	p.stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("lookup was not cancelled")
	}
	select {
	case fn := <-ch:
		fn()
		t.Fatal("posted lyrics after stop")
	case <-time.After(100 * time.Millisecond):
	}
	if p.lyricsLoaded {
		t.Error("loaded lyrics after stop")
	}
}

// Starting the next track cancels the lookup for the one before it.
func TestNextTrackCancelsCatalogue(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	entered, done := make(chan struct{}, 1), make(chan struct{})
	prov := &fakeLyricsProvider{block: true, entered: entered, done: done}
	p.lyricsLookup = prov

	p.loadLyrics(0, &lyricSource{}, track())
	<-entered
	p.gen++
	p.lyricsLookup = &fakeLyricsProvider{res: source.Result{LRC: "[00:03.00]next"}}
	p.loadLyrics(1, &lyricSource{}, track())

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("old lookup was not cancelled")
	}
	awaitLyrics(t, p, ch)
	lines, _ := p.Lyrics()
	if len(lines) != 1 || lines[0].Text != "next" {
		t.Errorf("lines %+v", lines)
	}
}

// A track with no name or first artist, and a podcast episode, ask no
// catalogue.
func TestIncompleteMetadataSkipsCatalogue(t *testing.T) {
	for _, c := range []struct {
		name string
		t    music.Track
	}{
		{"no title", music.Track{Artists: []music.Artist{{Name: "A"}}}},
		{"no artists", music.Track{Name: "Song"}},
		{"no artist name", music.Track{Name: "Song", Artists: []music.Artist{{}}}},
		{"podcast", music.Track{Name: "Episode", Artists: []music.Artist{{Name: "A"}}, ShowID: "show"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			p, ch := lyricsPlayer()
			defer p.stop()
			prov := &fakeLyricsProvider{res: source.Result{LRC: "[00:02.00]found"}}
			p.lyricsLookup = prov

			p.loadLyrics(0, &lyricSource{}, c.t)
			awaitLyrics(t, p, ch)

			if n := len(prov.asked()); n != 0 {
				t.Errorf("asked the catalogues %d times", n)
			}
		})
	}
}

// Loading lyrics never waits on the catalogues before it returns, so audio
// starts while they are still being asked.
func TestLyricsLookupDoesNotBlock(t *testing.T) {
	p, _ := lyricsPlayer()
	entered := make(chan struct{}, 1)
	p.lyricsLookup = &fakeLyricsProvider{block: true, entered: entered}

	done := make(chan struct{})
	go func() {
		p.loadLyrics(0, &lyricSource{}, track())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("loadLyrics waited on the catalogue")
	}
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("catalogue never asked")
	}
	p.stop()
}

// SetLyrics cancels a lookup still on its way, so injected lines stay.
func TestSetLyricsCancelsLookup(t *testing.T) {
	p, ch := lyricsPlayer()
	entered, done := make(chan struct{}, 1), make(chan struct{})
	p.lyricsLookup = &fakeLyricsProvider{block: true, entered: entered, done: done}

	p.loadLyrics(0, &lyricSource{}, track())
	<-entered
	p.SetLyrics([]lyrics.Line{{Text: "injected"}})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("lookup was not cancelled")
	}
	lines, loaded := p.Lyrics()
	if !loaded || len(lines) != 1 || lines[0].Text != "injected" {
		t.Errorf("loaded %v, lines %+v", loaded, lines)
	}
	select {
	case fn := <-ch:
		fn()
		t.Fatal("posted over injected lyrics")
	case <-time.After(100 * time.Millisecond):
	}
}

// Turning the fallback off keeps the source's lines, without asking it again.
func TestLyricFallbackOffKeepsSourceLyrics(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	p.Cue([]music.Track{track()})
	src := &lyricSource{can: true, got: &music.Lyrics{LRC: "[00:01.00]own"}}
	prov := &fakeLyricsProvider{res: source.Result{LRC: "[00:02.00]found"}}
	p.lyricsLookup = prov

	p.loadLyrics(0, src, track())
	awaitLyrics(t, p, ch)
	p.SetLyricFallback(false)

	lines, loaded := p.Lyrics()
	if !loaded || len(lines) != 1 || lines[0].Text != "own" {
		t.Errorf("loaded %v, lines %+v", loaded, lines)
	}
	if n := src.calls.Load(); n != 1 {
		t.Errorf("asked the source %d times", n)
	}
	if n := len(prov.asked()); n != 0 {
		t.Errorf("asked the catalogues %d times", n)
	}
}

// A load with the fallback off still uses the source, and never asks the
// catalogues.
func TestLyricFallbackOffAsksNoCatalogue(t *testing.T) {
	for _, c := range []struct {
		name string
		src  music.Source
		want string
	}{
		{"source has lyrics", &lyricSource{can: true, got: &music.Lyrics{LRC: "[00:01.00]own"}}, "own"},
		{"source has none", &lyricSource{}, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			p, ch := lyricsPlayer()
			defer p.stop()
			p.Cue([]music.Track{track()})
			prov := &fakeLyricsProvider{res: source.Result{LRC: "[00:02.00]found"}}
			p.lyricsLookup = prov

			p.SetLyricFallback(false)
			p.loadLyrics(0, c.src, track())
			awaitLyrics(t, p, ch)

			lines, loaded := p.Lyrics()
			if !loaded {
				t.Fatal("lyrics never loaded")
			}
			if c.want == "" {
				if len(lines) != 0 {
					t.Errorf("lines %+v", lines)
				}
			} else if len(lines) != 1 || lines[0].Text != c.want {
				t.Errorf("lines %+v", lines)
			}
			if n := len(prov.asked()); n != 0 {
				t.Errorf("asked the catalogues %d times", n)
			}
		})
	}
}

// Turning the fallback off drops lines a catalogue gave, as the switch says,
// while the source's own lines would stay.
func TestLyricFallbackOffDropsCatalogueLyrics(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	p.Cue([]music.Track{track()})
	prov := &fakeLyricsProvider{res: source.Result{LRC: "[00:02.00]found"}}
	p.lyricsLookup = prov

	p.loadLyrics(0, &lyricSource{}, track())
	awaitLine(t, p, ch, "found")
	p.SetLyricFallback(false)

	lines, loaded := p.Lyrics()
	if !loaded || len(lines) != 0 {
		t.Errorf("loaded %v, lines %+v", loaded, lines)
	}
	if n := len(prov.asked()); n != 1 {
		t.Errorf("asked the catalogues %d times", n)
	}
}

// Turning the fallback off cancels a lookup on its way, and its answer does
// not replace the lines shown.
func TestLyricFallbackOffCancelsLookup(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	p.Cue([]music.Track{track()})
	entered, done := make(chan struct{}, 1), make(chan struct{})
	prov := &fakeLyricsProvider{
		block: true, entered: entered, done: done,
		res: source.Result{LRC: "[00:02.00]stale"},
	}
	p.lyricsLookup = prov

	p.loadLyrics(0, &lyricSource{}, track())
	<-entered
	p.SetLyricFallback(false)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("lookup was not cancelled")
	}
	awaitLyrics(t, p, ch)
	lines, loaded := p.Lyrics()
	if !loaded || len(lines) != 0 {
		t.Errorf("loaded %v, lines %+v", loaded, lines)
	}
}

// Turning the fallback on fills in the current track's missing lyrics.
func TestLyricFallbackOnFillsCurrentTrack(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	p.Cue([]music.Track{track()})
	prov := &fakeLyricsProvider{res: source.Result{LRC: "[00:02.00]found"}}
	p.lyricsLookup = prov

	p.SetLyricFallback(false)
	p.loadLyrics(0, &lyricSource{}, track())
	awaitLyrics(t, p, ch)
	if n := len(prov.asked()); n != 0 {
		t.Fatalf("asked the catalogues %d times while off", n)
	}

	p.SetLyricFallback(true)
	awaitLine(t, p, ch, "found")
	if n := len(prov.asked()); n != 1 {
		t.Errorf("asked the catalogues %d times", n)
	}
}

// Setting the fallback to the value it already has asks nothing.
func TestLyricFallbackRepeatedSettingAsksNothing(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	p.Cue([]music.Track{track()})
	prov := &fakeLyricsProvider{res: source.Result{LRC: "[00:02.00]found"}}
	p.lyricsLookup = prov

	p.SetLyricFallback(false)
	p.SetLyricFallback(false)
	p.loadLyrics(0, &lyricSource{}, track())
	awaitLyrics(t, p, ch)
	p.SetLyricFallback(false)
	if n := len(prov.asked()); n != 0 {
		t.Fatalf("asked the catalogues %d times while off", n)
	}

	p.SetLyricFallback(true)
	awaitLine(t, p, ch, "found")
	if n := len(prov.asked()); n != 1 {
		t.Fatalf("asked the catalogues %d times", n)
	}
	p.SetLyricFallback(true)
	if n := len(prov.asked()); n != 1 {
		t.Errorf("asked the catalogues %d times after the same setting", n)
	}
}

// New starts with the catalogues off, without building a chain.
func TestNewStartsWithoutCatalogueChain(t *testing.T) {
	var built int
	provider := &fakeLyricsProvider{res: source.Result{LRC: "[00:01.00]external"}}
	defer swapLyricsProvider(func(source.Config) lyricsProvider { built++; return provider })()
	ch := make(chan func(), 1)
	p := New(func(fn func()) { ch <- fn })
	defer p.stop()
	p.loadLyrics(0, &lyricSource{}, track())
	awaitLyrics(t, p, ch)
	if built != 0 || len(provider.asked()) != 0 {
		t.Fatalf("a new player queried external lyrics: built %d chains", built)
	}
}

// A config change with the fallback off builds no chain, asks nothing, and
// keeps the config for when it is turned on.
func TestLyricConfigWhileOffBuildsNothing(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	p.SetLyricFallback(false)
	p.Cue([]music.Track{track()})

	var built []source.Config
	defer swapLyricsProvider(func(c source.Config) lyricsProvider {
		built = append(built, c)
		return nil
	})()
	cfg := source.DefaultConfig()
	cfg.RequestTimeoutSeconds = 3
	p.SetLyricConfig(cfg)

	select {
	case fn := <-ch:
		fn()
		t.Fatal("a config change with the fallback off posted a lookup")
	case <-time.After(100 * time.Millisecond):
	}
	if len(built) != 0 || p.lyricsLookup != nil {
		t.Errorf("built %d chains with the fallback off", len(built))
	}
	if p.lyricConfig != cfg {
		t.Error("the config in force was not kept")
	}
}

// Turning the fallback on builds the chain with the config in force, and asks
// it for the current track's missing lyrics.
func TestLyricConfigAppliedWhenFallbackTurnsOn(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	p.SetLyricFallback(false)
	p.Cue([]music.Track{track()})

	var built []source.Config
	prov := &fakeLyricsProvider{res: source.Result{LRC: "[00:02.00]built"}}
	defer swapLyricsProvider(func(c source.Config) lyricsProvider {
		built = append(built, c)
		return prov
	})()
	cfg := source.DefaultConfig()
	cfg.TimeoutSeconds = 3
	p.SetLyricConfig(cfg)
	p.SetLyricFallback(true)

	awaitLine(t, p, ch, "built")
	if len(built) != 1 || built[0] != cfg {
		t.Fatalf("built with %+v", built)
	}
}

// A config change while the fallback is on cancels the lookup made under the
// old config, and the old answer never shows.
func TestLyricConfigChangeDropsOldLookup(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	p.Cue([]music.Track{track()})

	entered, done := make(chan struct{}, 1), make(chan struct{})
	old := &fakeLyricsProvider{
		block: true, entered: entered, done: done,
		res: source.Result{LRC: "[00:02.00]old"},
	}
	p.lyricsLookup = old
	p.loadLyrics(0, &lyricSource{}, track())
	<-entered

	fresh := &fakeLyricsProvider{res: source.Result{LRC: "[00:02.00]fresh"}}
	var built []source.Config
	defer swapLyricsProvider(func(c source.Config) lyricsProvider {
		built = append(built, c)
		return fresh
	})()
	cfg := source.DefaultConfig()
	cfg.CacheHitHours = 1
	p.SetLyricConfig(cfg)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the old lookup was not cancelled")
	}
	awaitLine(t, p, ch, "fresh")
	if len(built) != 1 || built[0] != cfg {
		t.Errorf("built with %+v", built)
	}
}

// A config change asks again for lines a catalogue gave, keeping the source's
// own lyrics, the queue, play state and position.
func TestLyricConfigChangeRefreshesCatalogueLyrics(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	p.Cue([]music.Track{track()})
	p.playing = true

	old := &fakeLyricsProvider{res: source.Result{LRC: "[00:02.00]old"}}
	p.lyricsLookup = old
	p.loadLyrics(0, &lyricSource{}, track())
	awaitLine(t, p, ch, "old")

	fresh := &fakeLyricsProvider{res: source.Result{LRC: "[00:02.00]new"}}
	defer swapLyricsProvider(func(source.Config) lyricsProvider { return fresh })()
	cfg := source.DefaultConfig()
	cfg.CacheEntries = 10
	p.SetLyricConfig(cfg)

	awaitLine(t, p, ch, "new")
	if !p.playing || p.at != 0 || len(p.queue) != 1 {
		t.Errorf("play state changed: playing %v, at %d, queue %d", p.playing, p.at, len(p.queue))
	}
	if !p.lyricsCatalogue {
		t.Error("catalogue lyrics are no longer marked as such")
	}
}

// A config change keeps the source's own lyrics, without building a chain or
// asking it again.
func TestLyricConfigChangeKeepsSourceLyrics(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	p.Cue([]music.Track{track()})

	src := &lyricSource{can: true, got: &music.Lyrics{LRC: "[00:01.00]own"}}
	p.loadLyrics(0, src, track())
	awaitLyrics(t, p, ch)

	var built int
	defer swapLyricsProvider(func(source.Config) lyricsProvider { built++; return nil })()
	cfg := source.DefaultConfig()
	cfg.CacheMissMinutes = 1
	p.SetLyricConfig(cfg)

	if built != 0 || p.lyricsLookup != nil {
		t.Errorf("built %d chains over source lyrics", built)
	}
	if n := src.calls.Load(); n != 1 {
		t.Errorf("asked the source %d times", n)
	}
	lines, loaded := p.Lyrics()
	if !loaded || len(lines) != 1 || lines[0].Text != "own" {
		t.Errorf("loaded %v, lines %+v", loaded, lines)
	}
}

// Setting the config already in force rebuilds nothing and asks nothing.
func TestLyricConfigSameDoesNothing(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	p.Cue([]music.Track{track()})

	prov := &fakeLyricsProvider{res: source.Result{LRC: "[00:02.00]found"}}
	p.lyricsLookup = prov
	p.loadLyrics(0, &lyricSource{}, track())
	awaitLine(t, p, ch, "found")

	var built int
	defer swapLyricsProvider(func(source.Config) lyricsProvider { built++; return nil })()
	p.SetLyricConfig(p.lyricConfig)

	if built != 0 {
		t.Errorf("built %d chains for the same config", built)
	}
	if n := len(prov.asked()); n != 1 {
		t.Errorf("asked the catalogues %d times", n)
	}
}

// Clearing the cache calls the current provider's Clear, keeps the lyrics
// shown, and lets the next lookup ask again.
func TestClearLyricCacheKeepsLyrics(t *testing.T) {
	p, ch := lyricsPlayer()
	defer p.stop()
	p.Cue([]music.Track{track()})

	entered := make(chan struct{}, 2)
	prov := &fakeLyricsProvider{res: source.Result{LRC: "[00:02.00]found"}, entered: entered}
	p.lyricsLookup = prov
	p.loadLyrics(0, &lyricSource{}, track())
	awaitLine(t, p, ch, "found")
	<-entered

	p.ClearLyricCache()
	if n := prov.clears.Load(); n != 1 {
		t.Fatalf("cleared %d times", n)
	}
	lines, loaded := p.Lyrics()
	if !loaded || len(lines) != 1 || lines[0].Text != "found" {
		t.Errorf("loaded %v, lines %+v", loaded, lines)
	}

	p.loadLyrics(0, &lyricSource{}, track())
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the catalogue was not asked again after the cache was cleared")
	}
}
