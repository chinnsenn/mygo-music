package player

import (
	"context"
	"strings"

	"mygo-music/internal/lyrics"
	"mygo-music/internal/lyrics/source"
	"mygo-music/internal/music"
)

// lyricsProvider looks one track up in the catalogues of lyrics.go: the
// source package's Chain, or a test's stand-in.
type lyricsProvider interface {
	Find(context.Context, source.Query) (source.Result, error)
}

// lyricsClearer is a provider that can forget what it found without touching
// what the app shows or what plays.
type lyricsClearer interface {
	Clear()
}

// newLyricsProvider builds the catalogue chain the config asks for, only when
// the fallback is enabled. Existing chains can keep their caches while it is
// disabled. Tests replace the factory to keep lookups offline.
var newLyricsProvider = func(config source.Config) lyricsProvider {
	return source.NewConfiguredChain(config)
}

// SetLyricConfig keeps the config the catalogues are asked with, for the
// chain built when the fallback is on. A different config drops the chain:
// with the fallback on, the current track is asked again when its source has
// no lyrics or a catalogue gave them, and the lookup on its way is cancelled
// so its answer cannot outlive the config; with it off nothing is built and
// nothing is asked. The same config does nothing. Source lyrics, playback,
// the queue and the position stay as they are.
func (p *Player) SetLyricConfig(config source.Config) {
	config = config.Normalized()
	if config == p.lyricConfig {
		return
	}
	p.lyricConfig = config
	p.lyricsLookup = nil
	if !p.lyricFallback || p.track == nil {
		return
	}
	if p.lyricsLoaded && len(p.lyrics) > 0 && !p.lyricsCatalogue {
		return
	}
	p.loadLyrics(p.gen, p.sourceOf(p.track), *p.track)
}

// ClearLyricCache empties the catalogues' memory of what they found, so the
// next lookup asks them again. The lyrics shown and what plays stay as they
// are; a chain that was never built has nothing to clear.
func (p *Player) ClearLyricCache() {
	if c, ok := p.lyricsLookup.(lyricsClearer); ok {
		c.Clear()
	}
}

// SetLyricFallback lets the catalogues fill in lyrics the current source
// lacks, or stops them. Turning it off keeps what the source or the app gave,
// drops what a catalogue gave, and cancels a lookup on its way; turning it on
// builds the chain for the config in force and asks the catalogues for the
// current track when it has no lyrics. It leaves playback, the queue and the
// position alone.
func (p *Player) SetLyricFallback(enabled bool) {
	if p.lyricFallback == enabled {
		return
	}
	p.lyricFallback = enabled
	if enabled {
		if p.lyricsLookup == nil {
			p.lyricsLookup = newLyricsProvider(p.lyricConfig)
		}
		if p.track == nil || p.lyricsLoaded && (len(p.lyrics) > 0 || p.lyricsCatalogue) {
			return
		}
		p.loadLyrics(p.gen, p.sourceOf(p.track), *p.track)
		return
	}
	inFlight := p.lyricsStop != nil
	if inFlight {
		p.lyricsStop()
		p.lyricsStop = nil
	}
	if p.lyricsCatalogue {
		p.lyrics, p.lyricsLoaded, p.lyricsCatalogue = nil, true, false
	}
	if inFlight && !p.lyricsLoaded && p.track != nil {
		// The source's answer may have been on its way; ask it again without
		// the catalogues, so its lines still show.
		p.loadLyrics(p.gen, p.sourceOf(p.track), *p.track)
	}
}

// loadLyrics asks the source of t for its lyrics first, then the catalogues
// when it has none and the fallback is on: no capability, no answer, nothing
// timed, or an error. It starts a goroutine so a slow catalogue never holds up
// audio, and cancels the one before it. The fallback is read here, on the main
// thread, so the goroutine never reads the switch while it changes. Lyrics has
// no context, so the source is asked first and the cancellation is checked
// before the catalogues are asked.
func (p *Player) loadLyrics(gen int, src music.Source, t music.Track) {
	if p.lyricsStop != nil {
		p.lyricsStop()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.lyricsStop = cancel
	fallback := p.lyricFallback
	if fallback && p.lyricsLookup == nil {
		// The chain is built here, when a lookup first needs it, so a player
		// with the fallback off never makes one.
		p.lyricsLookup = newLyricsProvider(p.lyricConfig)
	}
	provider := p.lyricsLookup
	post := p.post

	go func() {
		lines, found := sourceLyrics(src, t)
		if ctx.Err() != nil {
			return
		}
		catalogue := !found && fallback
		if catalogue {
			lines = catalogueLyrics(ctx, provider, t)
		}
		if ctx.Err() != nil {
			return
		}
		post(func() {
			defer cancel()
			if gen != p.gen || ctx.Err() != nil {
				return
			}
			p.lyricsStop = nil
			p.lyrics, p.lyricsLoaded = lines, true
			p.lyricsCatalogue = catalogue
		})
	}()
}

// sourceLyrics returns the lines t's source has, and whether they are timed:
// an LRC with lines, or lines timed word by word. Anything else lets the
// catalogues be asked.
func sourceLyrics(src music.Source, t music.Track) ([]lyrics.Line, bool) {
	if src == nil || !src.Can("lyrics") {
		return nil, false
	}
	l, err := src.Lyrics(t.ID)
	if err != nil || l == nil {
		return nil, false
	}
	lines := lyrics.FromSource(l)
	return lines, len(lines) > 0
}

// catalogueLyrics asks the catalogues about t, and returns the timed lines
// they answer, or none. Only LRC and its translation are read: the sources'
// plain text has no timings to show.
func catalogueLyrics(ctx context.Context, provider lyricsProvider, t music.Track) []lyrics.Line {
	q, ok := catalogueQuery(t)
	if !ok || provider == nil {
		return nil
	}
	res, err := provider.Find(ctx, q)
	if err != nil || ctx.Err() != nil {
		return nil
	}
	return lyrics.FromSource(&music.Lyrics{LRC: res.LRC, Translation: res.Translation})
}

// catalogueQuery builds what the catalogues are asked about t. A track with no
// name or first artist has nothing to match, and a podcast episode is not a
// song: neither sends a query.
func catalogueQuery(t music.Track) (source.Query, bool) {
	if strings.TrimSpace(t.Name) == "" || t.ShowID != "" {
		return source.Query{}, false
	}
	if len(t.Artists) == 0 || strings.TrimSpace(t.Artists[0].Name) == "" {
		return source.Query{}, false
	}
	return source.Query{
		Title:    t.Name,
		Artist:   t.Artists[0].Name,
		Album:    t.Album.Name,
		Duration: t.Length(),
	}, true
}
