package main

import (
	"regexp"
	"sort"
	"strings"
)

// Structured abstracts mark their parts with headings: Europe PMC sends them as HTML (<h4>Background</h4>),
// other sources as plain text ("BACKGROUND Sepsis is…", "Methods: We…"). Every abstract is normalised to
// "Background: … Methods: … Results: …" so the page can show labelled paragraphs, and so the summary line
// and summary card are read from the Results and Conclusions rather than the background or study design.

var (
	headTagRe   = regexp.MustCompile(`(?is)<(h[1-6]|b|strong|sec-title|title)[^>]*>\s*([^<]{1,60}?)\s*[:.]?\s*</(?:h[1-6]|b|strong|sec-title|title)>`)
	plainHeadRe *regexp.Regexp // a known section name at the start or after a sentence, before a capital
	labelRe     *regexp.Regexp // a normalised "Name: " label anywhere
	sectionOf   = map[string]string{}
	canonical   = map[string]string{}
	otherSentRe = regexp.MustCompile(`(?i)\b(funded by|clinicaltrials\.gov|NCT\d{6,}|ISRCTN|PROSPERO|copyright|©)`)
)

func buildSectionRes() {
	names := []string{}
	for part, ns := range cat.AbstractSections {
		for _, n := range ns {
			sectionOf[strings.ToLower(n)] = part
			canonical[strings.ToLower(n)] = n
			names = append(names, n)
		}
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) }) // longest first
	alts := []string{}
	for _, n := range names {
		q := regexp.QuoteMeta(n)
		alts = append(alts, q, regexp.QuoteMeta(strings.ToUpper(n)), regexp.QuoteMeta(sentenceCase(n)))
	}
	alt := strings.Join(alts, "|")
	plainHeadRe = regexp.MustCompile(`(^|[.!?]\s+|\s)(` + alt + `)(?:\s*[:.]\s*|\s+)([A-Z0-9(“"])`)
	labelRe = regexp.MustCompile(`(?:^|\s)(` + alt + `):\s+`)
}

func sentenceCase(s string) string {
	if s == "" {
		return s
	}
	return s[:1] + strings.ToLower(s[1:])
}

// cleanAbstract strips markup and writes section headings as "Name: ".
func cleanAbstract(s string) string {
	s = headTagRe.ReplaceAllStringFunc(s, func(m string) string {
		g := headTagRe.FindStringSubmatch(m)
		label := strings.TrimSpace(g[2])
		if c, ok := canonical[strings.ToLower(label)]; ok {
			return " " + c + ": "
		}
		if strings.HasPrefix(strings.ToLower(g[1]), "h") && len(strings.Fields(label)) <= 6 { // an unknown heading
			return " " + label + ": "
		}
		return " " + label + " "
	})
	s = clean(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "Abstract: "), "ABSTRACT ")
	if plainHeadRe == nil {
		return s
	}
	// Plain-text headings. Only at the start or after the end of a sentence, and only before a capital, so the
	// word "results" inside a sentence is never taken for a heading.
	for i := 0; i < 2; i++ { // twice: matches that share a boundary
		s = plainHeadRe.ReplaceAllStringFunc(s, func(m string) string {
			g := plainHeadRe.FindStringSubmatch(m)
			midSentence := g[1] != "" && strings.TrimSpace(g[1]) == ""
			upper := g[2] == strings.ToUpper(g[2])
			colon := strings.Contains(m[len(g[1])+len(g[2]):], ":")
			if midSentence && !upper && !colon {
				return m // a Title-case word inside a sentence is only a heading when a colon follows
			}
			return g[1] + canonical[strings.ToLower(g[2])] + ": " + g[3]
		})
	}
	return strings.TrimSpace(s)
}

type sentence struct{ S, Part string }

// abstractSentences splits an abstract into sentences, each tagged with the part it belongs to
// ("" when the abstract has no headings) and with its heading removed.
func abstractSentences(text string) []sentence {
	out := []sentence{}
	locs := [][]int{}
	if labelRe != nil {
		locs = labelRe.FindAllStringSubmatchIndex(text, -1)
	}
	add := func(part, seg string) {
		for _, s := range sentences(seg) {
			if otherSentRe.MatchString(s) {
				part = "other"
			}
			out = append(out, sentence{s, part})
		}
	}
	if len(locs) == 0 {
		add("", text)
		return out
	}
	add("", text[:locs[0][0]])
	for i, l := range locs {
		end := len(text)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		add(sectionOf[strings.ToLower(text[l[2]:l[3]])], text[l[1]:end])
	}
	return out
}

func cueRe(s string) bool {
	for _, re := range cueRes {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}
