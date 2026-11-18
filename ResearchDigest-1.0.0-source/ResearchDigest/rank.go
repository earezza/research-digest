package main

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

type Signal struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

type Story struct {
	// Kind labels reviews, letters and other non-research items (shown in searches and categories only).
	Kind        string   `json:"kind,omitempty"`
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Venue       string   `json:"venue"`
	Source      string   `json:"source"`               // journal ISSN, "biorxiv", "medrxiv", "arxiv" or "other"
	ByAuthor    string   `json:"byAuthor,omitempty"`   // the searched author, as the paper writes the name
	AuthorRole  string   `json:"authorRole,omitempty"` // e.g. "last of 12 authors"
	Type        string   `json:"type"`
	Date        string   `json:"date"`
	DOI         string   `json:"doi"`
	URL         string   `json:"url"`
	Authors     string   `json:"authors"`
	Citations   int      `json:"citations"`
	Published   string   `json:"published"`
	Field       string   `json:"field"`
	Score       float64  `json:"score"`
	Signals     []Signal `json:"signals"`
	KeySentence string   `json:"keySentence"`
	Abstract    string   `json:"abstract"`
}

var abbr = regexp.MustCompile(`\b(e\.g|i\.e|et al|Fig|Figs|vs|approx|ca|cf|Dr|No|Eq|Ref|Refs|Ext|Suppl)\.`)

// sentences splits an abstract at . ! ? followed by space and a capital, digit or bracket.
func sentences(text string) []string {
	t := abbr.ReplaceAllStringFunc(text, func(m string) string { return strings.TrimSuffix(m, ".") + "∯" })
	rs := []rune(t)
	out, start := []string{}, 0
	for i := 0; i < len(rs); i++ {
		if rs[i] != '.' && rs[i] != '!' && rs[i] != '?' {
			continue
		}
		j := i + 1
		for j < len(rs) && unicode.IsSpace(rs[j]) {
			j++
		}
		if j == i+1 || j >= len(rs) {
			continue
		}
		if c := rs[j]; unicode.IsUpper(c) || unicode.IsDigit(c) || c == '(' || c == '"' || c == '“' {
			out = append(out, string(rs[start:i+1]))
			start = j
			i = j - 1
		}
	}
	out = append(out, string(rs[start:]))
	res := []string{}
	for _, s := range out {
		s = strings.TrimSpace(strings.ReplaceAll(s, "∯", "."))
		if len(s) > 25 {
			res = append(res, s)
		}
	}
	return res
}

func guessField(p Paper) string {
	if p.FieldHint != "" {
		return p.FieldHint
	}
	text := p.Title + " " + p.Abstract
	for _, r := range fieldRules {
		if r.Re.MatchString(text) {
			return r.Field
		}
	}
	return p.FieldDefault // "" means outside the digest's fields
}

// keySentence is the abstract sentence that states the main result: in a structured abstract, a "we show"
// sentence or the first sentence of the Results, then of the Conclusions; never the background or methods.
func keySentence(p Paper) string {
	all := abstractSentences(p.Abstract)
	structured := false
	for _, s := range all {
		structured = structured || s.Part != ""
	}
	pick := func(ok func(sentence) bool) string {
		for _, s := range all {
			if ok(s) {
				return s.S
			}
		}
		return ""
	}
	if structured {
		for _, try := range []func(sentence) bool{
			func(s sentence) bool {
				return (s.Part == "results" || s.Part == "conclusions") && keySentRe.MatchString(s.S)
			},
			func(s sentence) bool { return s.Part == "results" && cueRe(s.S) },
			func(s sentence) bool { return s.Part == "results" },
			func(s sentence) bool { return s.Part == "conclusions" },
		} {
			if k := pick(try); k != "" {
				return k
			}
		}
	}
	ss := []string{}
	for _, s := range all {
		if s.Part != "other" && s.Part != "background" && s.Part != "methods" {
			ss = append(ss, s.S)
		}
	}
	if len(ss) == 0 { // a heading that covers the whole abstract, e.g. only "Background"
		for _, s := range all {
			if s.Part != "other" {
				ss = append(ss, s.S)
			}
		}
	}
	for _, s := range ss {
		if keySentRe.MatchString(s) {
			return s
		}
	}
	if len(ss) > 0 {
		return ss[min(len(ss)-1, max(1, len(ss)/2))]
	}
	if len(p.Abstract) > 280 {
		return p.Abstract[:280] + "…"
	}
	return p.Abstract
}

type rankCtx struct {
	days        int
	byCitations bool
	noReviews   bool // the reader left reviews out
	maxCites    int
	maxRate     float64
}

func venueValue(p Paper) float64 {
	switch {
	case p.Tier == 1:
		return 1
	case p.Tier == 2 || p.Published != "":
		return 0.7
	}
	return 0.5
}

// citeRate is citations per month since publication, so a paper out for three weeks is not judged
// against one that has had a year to be cited.
func citeRate(p Paper) float64 {
	d, err := time.Parse("2006-01-02", p.Date)
	months := 1.0
	if err == nil {
		months = math.Max(1, time.Since(d).Hours()/24/30.44)
	}
	return float64(p.Citations) / months
}

func attention(p Paper, c rankCtx) float64 {
	if c.byCitations {
		if c.maxRate == 0 {
			return 0
		}
		return math.Log1p(citeRate(p)) / math.Log1p(c.maxRate)
	}
	d, err := time.Parse("2006-01-02", p.Date)
	if err != nil {
		return 0
	}
	age := time.Since(d.Add(12*time.Hour)).Hours() / 24
	return math.Max(0, math.Min(1, 1-age/float64(max(1, c.days))))
}

func cueValue(p Paper) float64 {
	text, n := p.Title+" "+p.Abstract, 0
	for _, re := range cueRes {
		if re.MatchString(text) {
			n++
		}
	}
	return math.Min(1, float64(n)/4)
}

func prescore(p Paper, c rankCtx) float64 {
	pen := 0.0
	if len(p.Abstract) < 500 {
		pen = 0.15
	}
	if isReview(p) {
		pen += 0.10 // reviews are often the most cited; keep them from crowding out new findings
	}
	return math.Max(0, 0.45*venueValue(p)+0.30*attention(p, c)+0.25*cueValue(p)-pen)
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// rank dedupes, scores and orders papers, keeping a field and venue spread so no one source floods the list.
func rank(papers []Paper, c rankCtx, n int) []Story {
	research := papers[:0:0] // the main tabs show original research with a real abstract only
	for _, p := range papers {
		if digestable(p) && !(c.noReviews && isReview(p)) {
			research = append(research, p)
		}
	}
	papers = research
	seen, titles := map[string]bool{}, map[string]bool{}
	pool := []Paper{}
	for _, p := range papers {
		t := strings.TrimSpace(nonAlnum.ReplaceAllString(strings.ToLower(p.Title), " "))
		if seen[p.ID] || titles[t] || junkRe.MatchString(p.Title) {
			continue
		}
		seen[p.ID], titles[t] = true, true
		pool = append(pool, p)
	}
	for _, p := range pool {
		c.maxCites = max(c.maxCites, p.Citations)
		c.maxRate = math.Max(c.maxRate, citeRate(p))
	}
	for i := range pool {
		pool[i].pre = prescore(pool[i], c)
		pool[i].fieldGuess = guessField(pool[i])
	}
	sort.SliceStable(pool, func(i, j int) bool {
		if pool[i].pre != pool[j].pre {
			return pool[i].pre > pool[j].pre
		}
		return pool[i].Date > pool[j].Date
	})
	// Limits per field and journal keep one source from filling a tab, but only matter when there are more
	// papers than room: first pick within the limits, then fill any free places in score order.
	perField, perVenue := map[string]int{}, map[string]int{}
	fieldCap, venueCap := (n+3)/4, (n+4)/5
	picked := make([]bool, len(pool))
	chosen := []int{}
	for i, p := range pool {
		if len(chosen) >= n {
			break
		}
		if p.fieldGuess == "" { // not one of the digest's fields (e.g. physics or computer science)
			continue
		}
		if perField[p.fieldGuess] >= fieldCap || perVenue[p.Venue] >= venueCap {
			continue
		}
		perField[p.fieldGuess]++
		perVenue[p.Venue]++
		picked[i] = true
		chosen = append(chosen, i)
	}
	for i, p := range pool {
		if len(chosen) >= n {
			break
		}
		if !picked[i] && p.fieldGuess != "" {
			picked[i] = true
			chosen = append(chosen, i)
		}
	}
	sort.Ints(chosen) // score order
	chosen = coverWithin(chosen, len(pool), n, func(i int) string { return pool[i].fieldGuess }, func(i int) string { return paperClass(pool[i].Type, pool[i].Kind) })
	sort.SliceStable(chosen, func(a, b int) bool { return chosen[a] < chosen[b] }) // keep score order
	out := []Story{}
	for _, i := range chosen {
		p := pool[i]
		second := Signal{"Recency", attention(p, c)}
		if c.byCitations {
			second.Label = "Citations"
		}
		out = append(out, Story{ID: p.ID, Title: p.Title, Venue: p.Venue, Source: p.SourceID, Type: p.Type, Date: p.Date, DOI: p.DOI, URL: p.URL,
			Authors: p.Authors, Citations: p.Citations, Published: p.Published, Field: p.fieldGuess,
			Score:       math.Round(p.pre*100) / 10,
			Signals:     []Signal{{"Venue", venueValue(p)}, second, {"Result cues", cueValue(p)}},
			KeySentence: keySentence(p), Abstract: p.Abstract, Kind: p.Kind})
	}
	return out
}

// coverWithin makes sure every field and paper type (peer-reviewed research, preprint or review) found among a range's candidates
// has at least one of its n places, so filtering a range by field or type never comes up empty while the range
// has a match (a longer range includes a shorter one's candidates, so if Past 5 years shows a field's preprint,
// All time can too). The range still shows exactly n papers: a missing field and type takes the place of the
// lowest-ranked paper whose field and type appear more than once. At most a third of the places change.
// chosen holds indices into the candidates (which are in score order); the result is in score order too.
func coverWithin(chosen []int, total, n int, field, class func(int) string) []int {
	type key struct{ f, class string }
	k := func(i int) key { return key{field(i), class(i)} }
	in, have := map[int]bool{}, map[key]int{}
	for _, i := range chosen {
		in[i] = true
		have[k(i)]++
	}
	missing, seen := []int{}, map[key]bool{}
	for i := 0; i < total; i++ {
		if c := k(i); c.f != "" && !in[i] && have[c] == 0 && !seen[c] {
			seen[c] = true
			missing = append(missing, i)
		}
	}
	if limit := max(1, n/3); len(missing) > limit {
		missing = missing[:limit]
	}
	for _, m := range missing {
		if len(chosen) < n {
			chosen = append(chosen, m)
			have[k(m)]++
			continue
		}
		for j := len(chosen) - 1; j >= 0; j-- {
			if have[k(chosen[j])] > 1 {
				have[k(chosen[j])]--
				chosen[j] = m
				have[k(m)]++
				break
			}
		}
	}
	sort.Ints(chosen)
	return chosen
}
