/**
 * [INPUT]: 依赖 unicode, strings, regexp, sync 以及 github.com/longbridgeapp/opencc 的 t2s 字典转换能力
 * [OUTPUT]: 对外提供 normalizeTitle, normalizeArtist, artistsOverlap, normalizeText 供匹配打分使用
 * [POS]: internal/lyrics/source 的曲目元数据归一化与繁简 canonical 比对层，被 source.go best() 及 Chain.Find 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 AGENTS.md
 */
package source

import (
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/longbridgeapp/opencc"
)

// Metadata from YouTube is decorated: titles carry "Official Music Video",
// remaster years and quoted Japanese titles, and artists come as one string of
// several. Matching normalizes both sides before comparing them, so a lookup
// is not lost to decoration.

var (
	// noiseGroup matches a bracketed tail that says nothing about the song.
	noiseGroup = regexp.MustCompile(`(?i)[（(\[【「『《]\s*(?:official\s*(?:music\s*)?video|music\s*video|official\s*audio|lyric\s*video|lyrics?|audio|mv|m/v|visuali[sz]er|hd|4k|explicit|remaster(?:ed)?(?:\s*\d{4})?|from\s+the\s+album[^)）\]】」』》]*)\s*[)）\]】」』》]`)
	// noiseWord matches a decoration that stands on its own.
	noiseWord = regexp.MustCompile(`(?i)\b(?:official\s*(?:music\s*)?video|music\s*video|official\s*audio|lyric\s*video|lyrics?|audio|mv|visuali[sz]er|remaster(?:ed)?(?:\s*\d{4})?)\b`)
	// feature finds "feat. X" after a space or an opening bracket, and the
	// names that follow it.
	feature = regexp.MustCompile(`(?i)(?:^|[\s（(\[])(?:feat\.?|ft\.?|featuring)\s+([^)）\]]*)`)
	// topicTail is YouTube's "- Topic" channel suffix.
	topicTail = regexp.MustCompile(`(?i)\s*-\s*topic\s*$`)
)

// normalizeTitle returns the bare title of s, without decorations.
func normalizeTitle(s string) string { title, _ := splitTitle(s); return title }

// splitTitle returns the bare title and the featured artists it named.
func splitTitle(s string) (string, []string) {
	s = strings.ReplaceAll(s, "\u3000", " ")
	s = strings.TrimSpace(topicTail.ReplaceAllString(s, ""))
	var featured []string
	for range 4 {
		m := feature.FindStringSubmatchIndex(s)
		if m == nil {
			break
		}
		names, cut := s[m[2]:m[3]], m[1]
		if i := strings.IndexAny(s[m[3]:], ")）]"); i >= 0 {
			names, cut = s[m[2]:m[3]+i], m[3]+i+1
		}
		for _, name := range strings.FieldsFunc(names, func(r rune) bool {
			return r == '&' || r == '，' || r == ','
		}) {
			if n := strings.TrimSpace(name); n != "" {
				featured = append(featured, n)
			}
		}
		s = strings.TrimRight(s[:m[0]], " （([") + " " + strings.TrimLeft(s[cut:], " )）]")
		s = strings.TrimSpace(s)
	}
	s = noiseGroup.ReplaceAllString(s, " ")
	s = noiseWord.ReplaceAllString(s, " ")
	return normalizeText(s), featured
}

// normalizeArtist returns the first artist of a credit, normalized.
func normalizeArtist(s string) string {
	lower := strings.ToLower(s)
	for _, sep := range []string{"/", "&", "、", ",", "，", ";", "；", " feat", " ft"} {
		if i := strings.Index(lower, sep); i > 0 {
			s, lower = s[:i], lower[:i]
		}
	}
	return normalizeText(s)
}

// artistsOverlap reports whether two credits name the same first artist.
func artistsOverlap(a, b string) bool {
	x, y := normalizeArtist(a), normalizeArtist(b)
	switch {
	case x == "" || y == "":
		return false
	case x == y:
		return true
	case len(x) >= 2 && strings.Contains(y, x):
		return true
	case len(y) >= 2 && strings.Contains(x, y):
		return true
	}
	return false
}

// convT2S is a lazily initialized, read-only OpenCC Traditional-to-Simplified converter.
// The underlying dictionary is embedded statically; creation failure indicates an internal invariant breakage.
var convT2S = sync.OnceValue(func() *opencc.OpenCC {
	cc, err := opencc.New("t2s")
	if err != nil {
		panic("source: failed to initialize embedded OpenCC t2s dictionary")
	}
	return cc
})

// normalizeText lowercases s, keeps only letters and digits, and converts
// any Chinese characters (unicode.Han) from Traditional to Simplified canonical
// form, so punctuation, spacing and script variants never cause a false mismatch.
// Non-Chinese scripts (pure Latin, Japanese kana, Korean hangul, etc.) bypass
// OpenCC conversion naturally without additional allocations.
func normalizeText(s string) string {
	s = strings.ToLower(s)
	hasHan := false
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			if !hasHan && unicode.Is(unicode.Han, r) {
				hasHan = true
			}
		}
	}
	clean := b.String()
	if !hasHan {
		return clean
	}

	conv, err := convT2S().Convert(clean)
	if err != nil {
		panic("source: opencc conversion invariant failure")
	}
	return conv
}
