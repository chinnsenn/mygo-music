/**
 * [INPUT]: 依赖 music 曲目、lyrics 时间线与 MyGo audio.Player 的测试装配能力
 * [OUTPUT]: 提供 Cue、SetLyrics、Hold、Attach、SetStream，构造视图测试所需的播放状态
 * [POS]: internal/player 的测试装配入口；SetLyrics 接管歌词所有权、取消未完成查询，并把歌词标记为非目录来源
 * [PROTOCOL]: 变更时更新此头部，然后检查 AGENTS.md
 */
package player

import (
	"time"

	"github.com/egoist/mygo/audio"

	"mygo-music/internal/lyrics"
	"mygo-music/internal/music"
)

// These set a player up as it would be after loading, without loading
// anything, for the tests of the views that show it.

// Cue makes tracks the queue, in order, with the first current.
func (p *Player) Cue(tracks []music.Track) {
	p.queue = tracks
	p.order = make([]int, len(tracks))
	for i := range p.order {
		p.order[i] = i
	}
	p.at = 0
	p.track = &p.queue[0]
}

// SetLyrics gives the current track lines, as loaded, and drops a lookup still
// on its way: the injected lines are the ones to show. They aren't a
// catalogue's, so turning the fallback off keeps them.
func (p *Player) SetLyrics(lines []lyrics.Line) {
	if p.lyricsStop != nil {
		p.lyricsStop()
		p.lyricsStop = nil
	}
	p.lyrics, p.lyricsLoaded, p.lyricsCatalogue = lines, true, false
}

// Hold makes the player play at pos with nothing to play: its position
// stays there.
func (p *Player) Hold(pos time.Duration) {
	p.pending = pos
	p.playing = true
}

// Attach makes out the current track's audio, playing.
func (p *Player) Attach(out *audio.Player) {
	p.out = out
	p.playing = true
}

// SetStream makes st what the current track streams.
func (p *Player) SetStream(st *music.Stream) { p.info = st }
