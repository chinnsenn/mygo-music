package source

import (
	"regexp"
	"strings"
)

type versionKind int

const (
	versionUnknown      versionKind = 0
	versionLive         versionKind = 1 << 0
	versionAcoustic     versionKind = 1 << 1
	versionRemix        versionKind = 1 << 2
	versionInstrumental versionKind = 1 << 3
	versionCover        versionKind = 1 << 4
)

type parsedVersion struct {
	kind      versionKind
	baseTitle string
	tag       string
	year      string
	event     string
}

type trackMeta struct {
	version    parsedVersion
	cleanTitle string
	isLive     bool
	albumCores []string
	albumYear  string
	tagYear    string
}

var (
	// versionSuffixRe matches bracketed tails indicating a track version.
	versionSuffixRe = regexp.MustCompile(`(?i)[（(\[【「『《]\s*([^()\[\]【】「」『』《》]+?)\s*[)）\]】」』》]\s*$`)

	// dashSuffixRe matches hyphen / emdash suffix: " - Live" or " — 现场版"
	dashSuffixRe = regexp.MustCompile(`(?i)\s*[-—–]\s*([^-—–]+)\s*$`)

	// liveTagExact matches live marker words.
	liveTagExact = regexp.MustCompile(`(?i)^(?:live|现场|现场版|現場|現場版|演唱会版|演唱會版|演唱会现场版|演唱會現場版|live\s*version|live\s*concert)$`)

	// non-live edition markers.
	acousticTagRe     = regexp.MustCompile(`(?i)^(?:acoustic|原声|不插电|unplugged)$`)
	remixTagRe        = regexp.MustCompile(`(?i)^(?:remix|混音)$`)
	instrumentalTagRe = regexp.MustCompile(`(?i)^(?:instrumental|伴奏|纯音乐|off\s*vocal|karaoke)$`)
	coverTagRe        = regexp.MustCompile(`(?i)^(?:cover|翻唱|原版翻唱)$`)

	// liveWordTokenRe matches "live" or "concert" as a standalone English word with word boundaries.
	liveWordTokenRe = regexp.MustCompile(`(?i)\b(?:live|concert|tour)\b`)

	// liveChineseTokenRe matches Chinese live keywords.
	liveChineseTokenRe = regexp.MustCompile(`现场|現場|演唱会|演唱會`)

	// livePackagingLongestFirst matches live packaging terms with longest variants first to avoid partial replacements.
	livePackagingLongestFirst = regexp.MustCompile(`(?i)(?:世界巡回演唱会|世界巡迴演唱會|巡回演唱会|巡迴演唱會|演唱会影音全记录珍藏盘|演唱会影音记录珍藏盘|演唱會影音全記錄珍藏盤|演唱會影音記錄珍藏盤|现场原音专辑|現場原音專輯|现场专辑|現場專輯|演唱会现场版|演唱會現場版|演唱会版|演唱會版|演唱会|演唱會|现场版|現場版|现场|現場|\b(?:world\s*tour|live\s*concert|concert|tour|live)\b)`)

	// mediaPackagingRe matches physical format and packaging words on token boundaries.
	mediaPackagingRe = regexp.MustCompile(`(?i)\b(?:\d+cd|cd|dvd|bluray|bd|vinyl|lp|explicit)\b`)
	// eventWrapperWords strips structural wrappers from live tags to get opaque event residue.
	eventWrapperWords = regexp.MustCompile(`(?i)\b(?:live|concert|version|tour|at|in|from)\b|现场版|現場版|演唱会版|演唱會版|现场|現場|演唱会|演唱會`)

	// yearRe matches 4-digit years like 2003, 1999.
	yearRe = regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)

	// albumSplitRe matches outer parallel title in brackets e.g. "Main Title (Bilingual Title)"
	albumSplitRe = regexp.MustCompile(`^(.*?)[（(\[【](.*?)[)）\]】]\s*$`)
)

// parseVersion separates the base title from explicit version tags (such as (Live), [Remix]).
// Iterates up to 3 times to peel multiple orthogonal suffixes without losing live or instrumental info.
func parseVersion(s string) parsedVersion {
	s = strings.TrimSpace(s)
	pv := parsedVersion{baseTitle: s}

	for range 3 {
		trimmed := strings.TrimSpace(pv.baseTitle)
		matched := false

		// 1. Check bracket suffix
		if m := versionSuffixRe.FindStringSubmatchIndex(trimmed); m != nil {
			tag := strings.TrimSpace(trimmed[m[2]:m[3]])
			base := strings.TrimSpace(trimmed[:m[0]])
			k := classifySingleTag(tag)
			if k != versionUnknown {
				pv.kind |= k
				if k&versionLive != 0 {
					if pv.year == "" {
						pv.year = extractYear(tag)
					}
					if pv.event == "" {
						pv.event = extractEventResidue(tag)
					}
				}
				pv.baseTitle = base
				pv.tag = tag
				matched = true
			}
		}

		// 2. Check dash / emdash suffix
		if !matched {
			if m := dashSuffixRe.FindStringSubmatchIndex(trimmed); m != nil {
				tag := strings.TrimSpace(trimmed[m[2]:m[3]])
				base := strings.TrimSpace(trimmed[:m[0]])
				k := classifySingleTag(tag)
				if k != versionUnknown {
					pv.kind |= k
					if k&versionLive != 0 {
						if pv.year == "" {
							pv.year = extractYear(tag)
						}
						if pv.event == "" {
							pv.event = extractEventResidue(tag)
						}
					}
					pv.baseTitle = base
					pv.tag = tag
					matched = true
				}
			}
		}

		if !matched {
			break
		}
	}

	return pv
}

// classifySingleTag inspects the extracted tag to determine its kind.
func classifySingleTag(tag string) versionKind {
	lower := strings.ToLower(tag)

	// Exact matches
	switch {
	case liveTagExact.MatchString(lower):
		return versionLive
	case acousticTagRe.MatchString(lower):
		return versionAcoustic
	case remixTagRe.MatchString(lower):
		return versionRemix
	case instrumentalTagRe.MatchString(lower):
		return versionInstrumental
	case coverTagRe.MatchString(lower):
		return versionCover
	}

	// Check if live token is present as a standalone word / token
	if liveWordTokenRe.MatchString(lower) || liveChineseTokenRe.MatchString(lower) {
		return versionLive
	}

	return versionUnknown
}

// extractEventResidue returns the normalized opaque event/venue residue from a live tag
// by stripping live/concert/at/in/from wrappers and years.
func extractEventResidue(tag string) string {
	clean := eventWrapperWords.ReplaceAllString(tag, " ")
	clean = yearRe.ReplaceAllString(clean, " ")
	return normalizeText(clean)
}

// isLiveAlbum reports whether an album name contains standalone live tokens.
// Uses word boundaries for English to never match "Oliver", "Deliverance", etc.
func isLiveAlbum(album string) bool {
	if album == "" {
		return false
	}
	if liveWordTokenRe.MatchString(album) {
		return true
	}
	return liveChineseTokenRe.MatchString(album)
}
func stripAlbumPackaging(s string) string {
	s = mediaPackagingRe.ReplaceAllString(s, " ")
	s = livePackagingLongestFirst.ReplaceAllString(s, " ")
	return s
}

func stripArtistBoundary(core, art string) string {
	if art == "" {
		return core
	}
	artNorm := normalizeText(art)
	if artNorm == "" {
		return core
	}
	if strings.HasPrefix(core, artNorm) {
		return strings.TrimPrefix(core, artNorm)
	}
	if strings.HasSuffix(core, artNorm) {
		return strings.TrimSuffix(core, artNorm)
	}
	// Also check if the artist name is sandwiched between different scripts (e.g. Han artist inside Latin/digits: "duo陈奕迅2010")
	if idx := strings.Index(core, artNorm); idx > 0 {
		end := idx + len(artNorm)
		prevRune := core[idx-1]
		nextRune := byte(0)
		if end < len(core) {
			nextRune = core[end]
		}
		// If the artist contains Han characters, it cannot be a substring of a Latin word like "stairway"
		isHanArtist := false
		for _, r := range art {
			if r >= 0x4e00 && r <= 0x9fa5 {
				isHanArtist = true
				break
			}
		}
		if isHanArtist {
			isPrevLatinOrDigit := (prevRune >= '0' && prevRune <= '9') || (prevRune >= 'a' && prevRune <= 'z')
			isNextLatinOrDigit := end == len(core) || (nextRune >= '0' && nextRune <= '9') || (nextRune >= 'a' && nextRune <= 'z')
			if isPrevLatinOrDigit && isNextLatinOrDigit {
				core = core[:idx] + core[end:]
			}
		}
	}
	return core
}

// albumCoreVariants extracts canonical normalized album core variants.
// Only splits brackets if there is a genuine parallel bilingual release title (e.g. Han main title + Latin translation
// where both sides convey concert/tour meaning), avoiding stripping venues or editions into false shared cores.
func albumCoreVariants(album, qArtist, cArtist string, cAliases []string) []string {
	if album == "" {
		return nil
	}
	rawParts := []string{album}
	if m := albumSplitRe.FindStringSubmatch(album); len(m) > 2 {
		mainPart := strings.TrimSpace(m[1])
		bracketPart := strings.TrimSpace(m[2])
		// True bilingual parallel check:
		// 1. mainPart has Han, bracketPart has Latin.
		// 2. bracketPart must represent a complete standalone tour/live entity (e.g. "World Tour" or "Live Concert")
		// 3. bracketPart must NOT be an event/venue tag (e.g. "Live in Tokyo", "Live at Red Rocks").
		hasHanMain := false
		for _, r := range mainPart {
			if r >= 0x4e00 && r <= 0x9fa5 {
				hasHanMain = true
				break
			}
		}
		hasLatinBracket := false
		for _, r := range bracketPart {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				hasLatinBracket = true
				break
			}
		}
		bracketLower := strings.ToLower(bracketPart)
		isEventTag := strings.Contains(bracketLower, "live in") || strings.Contains(bracketLower, "live at") || strings.Contains(bracketLower, "live from")
		hasTourStructure := strings.Contains(bracketLower, "world tour") || strings.Contains(bracketLower, "tour live") || strings.Contains(bracketLower, "live concert")

		if hasHanMain && hasLatinBracket && hasTourStructure && !isEventTag {
			bracketCore := normalizeText(stripAlbumPackaging(bracketPart))
			if mainPart != "" && bracketCore != "" {
				rawParts = append(rawParts, mainPart, bracketPart)
			}
		}
	}
	var cores []string
	seen := make(map[string]bool)

	for _, part := range rawParts {
		clean := stripAlbumPackaging(part)

		// 2. Normalize canonically
		core := normalizeText(clean)
		if core == "" {
			continue
		}

		// 3. Strip known artist names on boundary prefix or suffix without allocating slice
		core = stripArtistBoundary(core, qArtist)
		core = stripArtistBoundary(core, cArtist)
		for _, alias := range cAliases {
			core = stripArtistBoundary(core, alias)
		}

		if core != "" && !seen[core] {
			seen[core] = true
			cores = append(cores, core)
		}
	}

	return cores
}

func extractYear(s string) string {
	return yearRe.FindString(s)
}

// artistMatches reports whether Query artist matches Candidate artist directly
// or matches one of Candidate's declared primary artist aliases strictly (full normalized equality).
func artistMatches(qArtist, cArtist string, cAliases []string) bool {
	if artistsOverlap(qArtist, cArtist) {
		return true
	}
	qNorm := normalizeText(qArtist)
	if qNorm == "" {
		return false
	}
	for _, alias := range cAliases {
		if normalizeText(alias) == qNorm {
			return true
		}
	}
	return false
}

// parseQueryMeta pre-calculates normalized values for a query once.
func parseQueryMeta(q Query) trackMeta {
	v := parseVersion(q.Title)
	normBase := normalizeTitle(v.baseTitle)
	clean := trimArtist(normBase, q.Artist)

	live := v.kind&versionLive != 0 || isLiveAlbum(q.Album)
	cores := albumCoreVariants(q.Album, q.Artist, "", nil)

	return trackMeta{
		version:    v,
		cleanTitle: clean,
		isLive:     live,
		albumCores: cores,
		albumYear:  extractYear(q.Album),
		tagYear:    v.year,
	}
}

// parseCandidateMeta pre-calculates normalized values for a candidate once.
func parseCandidateMeta(c Candidate, qArtist string) trackMeta {
	v := parseVersion(c.Title)
	normBase := normalizeTitle(v.baseTitle)
	clean := trimArtist(normBase, c.Artist)

	live := v.kind&versionLive != 0 || isLiveAlbum(c.Album)
	cores := albumCoreVariants(c.Album, qArtist, c.Artist, c.ArtistAliases)

	return trackMeta{
		version:    v,
		cleanTitle: clean,
		isLive:     live,
		albumCores: cores,
		albumYear:  extractYear(c.Album),
		tagYear:    v.year,
	}
}

// coresIntersect reports whether query and candidate share at least one exact normalized album core.
func coresIntersect(qCores, cCores []string) bool {
	for _, qC := range qCores {
		for _, cC := range cCores {
			if qC == cC {
				return true
			}
		}
	}
	return false
}

// candidateTier calculates compatibility rank:
// Tier 4: Query is live, candidate is live, confirmed same album core / event / year.
// Tier 3: Query is live, candidate is live, uncontradicted generic live.
// Tier 2: Normal studio/neutral query and candidate (or query has specific non-live edition matching candidate).
// Tier 1: Query is studio/neutral, but candidate is tagged edition (live/remix/acoustic) -> deprioritized.
//
//	OR Query explicitly asks for non-live edition (e.g. Remix) but candidate is standard studio/unknown.
//
// Tier 0: Incompatible (query live vs candidate studio, acoustic vs live, different concert years/events/cores).
func candidateTier(qM, cM trackMeta) int {
	// 1. Explicit orthogonal version kinds compatibility
	// If query explicitly requests a kind (e.g. Instrumental or Remix), candidate must carry it
	// Non-live edition conflict check (e.g. Instrumental vs non-Instrumental, Remix vs Acoustic)
	qNonLive := qM.version.kind &^ versionLive
	cNonLive := cM.version.kind &^ versionLive
	if qM.isLive {
		// When query is live, non-live edition must match strictly (bare live cannot take Live Instrumental)
		if qNonLive != cNonLive {
			return 0
		}
	} else if qM.version.kind != versionUnknown && cM.version.kind != versionUnknown {
		if qNonLive != cNonLive {
			return 0
		}
	}
	if qM.isLive {
		// When query is live, candidates without live evidence are rejected
		if !cM.isLive {
			return 0
		}

		// Check year conflict across album or tag
		if hasYearConflict(qM, cM) {
			return 0
		}

		// Check opaque event residue conflict (e.g. Red Rocks vs Seoul, Taipei vs Tokyo)
		if qM.version.event != "" && cM.version.event != "" && qM.version.event != cM.version.event {
			return 0
		}

		// Album core check
		if len(qM.albumCores) > 0 && len(cM.albumCores) > 0 {
			if coresIntersect(qM.albumCores, cM.albumCores) {
				return 4 // Confirmed same concert album
			}
			// Both have distinct album cores -> different concerts
			return 0
		}

		return 3 // Live match without contradicting album core
	}

	// Query is NOT live
	if cM.isLive {
		return 1 // Deprioritized so live does not usurp studio
	}

	// Non-live editions:
	// Case A: Query is standard studio (versionUnknown), candidate is tagged non-live edition (remix/acoustic...)
	if qM.version.kind == versionUnknown && cM.version.kind != versionUnknown {
		return 1 // Deprioritized so tagged edition does not usurp normal studio
	}

	// Case B: Query explicitly asked for a specific non-live edition (remix, acoustic...), but candidate is standard studio
	if qM.version.kind != versionUnknown && cM.version.kind == versionUnknown {
		return 1 // Deprioritized below exact matching edition candidate
	}

	return 2 // Standard studio/neutral match, or exact edition match
}

// hasYearConflict checks whether explicit non-empty years in query and candidate contradict.
func hasYearConflict(a, b trackMeta) bool {
	if a.albumYear != "" && b.albumYear != "" && a.albumYear != b.albumYear {
		return true
	}
	if a.tagYear != "" && b.tagYear != "" && a.tagYear != b.tagYear {
		return true
	}
	if a.albumYear != "" && b.tagYear != "" && a.albumYear != b.tagYear {
		return true
	}
	if a.tagYear != "" && b.albumYear != "" && a.tagYear != b.albumYear {
		return true
	}
	return false
}

// hasDistinctConcertIdentity reports whether two live candidates have demonstrably
// different concert identities (different non-empty album cores, years, or event residues).
func hasDistinctConcertIdentity(a, b trackMeta) bool {
	if len(a.albumCores) > 0 && len(b.albumCores) > 0 && !coresIntersect(a.albumCores, b.albumCores) {
		return true
	}
	if hasYearConflict(a, b) {
		return true
	}
	if a.version.event != "" && b.version.event != "" && a.version.event != b.version.event {
		return true
	}
	return false
}
