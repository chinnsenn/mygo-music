/*
[INPUT]: 依赖 mygo/ui 的 Select/Button、internal/i18n 的翻译入口，以及 internal/lyrics/source 的 Config 字段
[OUTPUT]: 对外提供 app.lyricSettings 参数行及 lyricSettingsGroup 紧凑子表单，绘制来源、超时、缓存参数与清除按钮
[POS]: internal/app 设置窗口的歌词子设置：无内层卡片，说明通过悬停提示呈现；总开关控制可见性，经 changed 保存并交给播放器
[PROTOCOL]: 变更时更新此头部，然后检查 AGENTS.md
*/
package app

import (
	"github.com/egoist/mygo/ui"

	"mygo-music/internal/lyrics/source"
)

// lyricSources are the catalogues a lyric slot may name, in the order the
// settings offer them. The empty ID is None, which any number of slots may be.
var lyricSources = []struct{ id, label string }{
	{"netease", N("NetEase Cloud Music")},
	{"qq", N("QQ Music")},
	{"kugou", N("Kugou")},
}

// The numbers the lyric settings offer, by row. Zero is off, or no cache,
// and stays zero: the row never puts it back to the default.
var (
	lyricTimeoutSeconds  = []int{5, 8, 12, 20, 30, 60}
	lyricHTTPTimeSeconds = []int{3, 5, 8, 12, 20, 30}
	lyricHitHours        = []int{0, 1, 6, 12, 24, 72}
	lyricMissMinutes     = []int{0, 5, 15, 30, 60, 240}
	lyricCacheEntries    = []int{0, 128, 256, 512, 1024, 4096}
)

// lyricSettingsGroup keeps subordinate preferences compact, using indentation
// for hierarchy and contextual help instead of another bordered settings card.
func (a *app) lyricSettingsGroup(c *ui.Context, p palette, query string) {
	a.lyricSettings(c, func(label, desc string, control func()) {
		if query != "" && !settingsMatch(query, label, desc) {
			return
		}
		ui.Row(c).MinHeight(36).Gap(16).AlignItems(ui.Center).Tooltip(desc).Children(func() {
			ui.Text(c, label).FontSize(12.5).TextColor(p.muted).Grow(1).Shrink(1).MinWidth(0)
			control()
		})
	})
}

// lyricSettings draws preferences inside the enabled fallback's child group.
// Hiding the group preserves the saved lookup and cache preferences.
func (a *app) lyricSettings(c *ui.Context, row settingsRow) {
	cfg := &a.settings.LyricConfig
	a.lyricSourceRow(c, row, cfg, 0, "lyric-source-first", N("First Lyric Source"), N("Where cross-source lyrics are looked up first; the song's own source still comes first."))
	a.lyricSourceRow(c, row, cfg, 1, "lyric-source-second", N("Second Lyric Source"), N("Asked when the first source has no lyrics."))
	a.lyricSourceRow(c, row, cfg, 2, "lyric-source-third", N("Third Lyric Source"), N("Asked last."))
	a.lyricNumberRow(c, row, "lyric-lookup-timeout", N("Lookup Timeout"), N("How long all the lyric sources may take together."),
		lyricTimeoutSeconds, "second", N("Off"), cfg.TimeoutSeconds, func(v int) {
			cfg.TimeoutSeconds = v
			a.changed()
		})
	a.lyricNumberRow(c, row, "lyric-http-timeout", N("HTTP Request Timeout"), N("How long one request to a lyric source may take."),
		lyricHTTPTimeSeconds, "second", N("Off"), cfg.RequestTimeoutSeconds, func(v int) {
			cfg.RequestTimeoutSeconds = v
			a.changed()
		})
	a.lyricNumberRow(c, row, "lyric-hit-cache", N("Cache Hit Duration"), N("How long found lyrics are kept before asking again."),
		lyricHitHours, "hour", N("No cache"), cfg.CacheHitHours, func(v int) {
			cfg.CacheHitHours = v
			a.changed()
		})
	a.lyricNumberRow(c, row, "lyric-miss-cache", N("Cache Miss Duration"), N("How long a source that had no lyrics is remembered."),
		lyricMissMinutes, "minute", N("No cache"), cfg.CacheMissMinutes, func(v int) {
			cfg.CacheMissMinutes = v
			a.changed()
		})
	a.lyricNumberRow(c, row, "lyric-cache-size", N("Cache Capacity"), N("How many songs' lyrics are remembered."),
		lyricCacheEntries, "song", N("Off"), cfg.CacheEntries, func(v int) {
			cfg.CacheEntries = v
			a.changed()
		})
	row(T("Clear Lyric Cache"), T("Delete the lyrics kept from cross-source lookups."), func() {
		if ui.Button(c.Key("clear-lyric-cache"), T("Clear")).Clicked() {
			if a.pl != nil {
				a.pl.ClearLyricCache()
			}
		}
	})
}

// lyricSourceRow offers the catalogues for one slot. Picking one that another
// slot holds swaps the two, so a catalogue is still asked once, while None may
// fill any number of them.
func (a *app) lyricSourceRow(c *ui.Context, row settingsRow, cfg *source.Config, slot int, key, label, desc string) {
	row(T(label), T(desc), func() {
		names := make([]string, len(lyricSources)+1)
		names[0] = T("None")
		chosen := names[0]
		for i, src := range lyricSources {
			names[i+1] = T(src.label)
			if src.id == cfg.Sources[slot] {
				chosen = names[i+1]
			}
		}
		if ui.Select(c.Key(key), &chosen, names).Label(T(label)).Width(170).Changed() {
			for i, name := range names {
				if name == chosen {
					id := ""
					if i > 0 {
						id = lyricSources[i-1].id
					}
					lyricSwapSource(cfg, slot, id)
					a.changed()
					return
				}
			}
		}
	})
}

// lyricSwapSource puts id in one slot. When another slot holds it, that slot
// takes the one this had, so the catalogues keep their places; None is exempt
// and may repeat.
func lyricSwapSource(cfg *source.Config, slot int, id string) {
	was := cfg.Sources[slot]
	if id != "" {
		for i := range cfg.Sources {
			if i != slot && cfg.Sources[i] == id {
				cfg.Sources[i] = was
				break
			}
		}
	}
	cfg.Sources[slot] = id
}

// lyricNumberRow offers preset numbers, in the row's unit. A value the file
// already holds but the presets do not is offered too, so the row shows what
// is actually set; zero shows as zero's label rather than a default.
func (a *app) lyricNumberRow(c *ui.Context, row settingsRow, key, label, desc string, values []int, unit, zero string, cur int, pick func(int)) {
	row(T(label), T(desc), func() {
		labels := make([]string, len(values), len(values)+1)
		chosen := ""
		for i, v := range values {
			labels[i] = lyricNumberLabel(v, unit, zero)
			if v == cur {
				chosen = labels[i]
			}
		}
		if chosen == "" {
			chosen = lyricNumberLabel(cur, unit, zero)
			labels = append(labels, chosen)
		}
		if ui.Select(c.Key(key), &chosen, labels).Label(T(label)).Width(170).Changed() {
			for i, name := range labels {
				if name == chosen {
					value := cur
					if i < len(values) {
						value = values[i]
					}
					pick(value)
					return
				}
			}
		}
	})
}

// lyricNumberLabel names one number of a row: its unit spelled out, with
// counts of one in the singular. Zero is the row's own word for off.
func lyricNumberLabel(n int, unit, zero string) string {
	if n <= 0 {
		return T(zero)
	}
	switch unit {
	case "second":
		if n == 1 {
			return T("1 second")
		}
		return Tf("%d seconds", n)
	case "minute":
		if n == 1 {
			return T("1 minute")
		}
		return Tf("%d minutes", n)
	case "hour":
		if n == 1 {
			return T("1 hour")
		}
		return Tf("%d hours", n)
	}
	if n == 1 {
		return T("1 song")
	}
	return Tf("%d songs", n)
}
