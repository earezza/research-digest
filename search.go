package main

import (
	"context"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Keyword search across the same open sources: OpenAlex (journals and preprint servers) and arXiv.
var searchRanges = map[string]int{"month": 30, "year": 365, "5y": 1826, "all": 0}
var searchLabels = map[string]string{"month": "past month", "year": "past year", "5y": "past 5 years", "all": "any time"}

var termRe = regexp.MustCompile(`"[^"]+"|[^\s"]+`)
var stop = map[string]bool{"a": true, "an": true, "and": true, "or": true, "the": true, "of": true, "in": true, "on": true, "for": true, "to": true, "with": true, "by": true, "at": true, "from": true}

// searchTerms splits a query into lower-case words and "quoted phrases", dropping filler words.
func searchTerms(q string) []string {
	out := []string{}
	for _, t := range termRe.FindAllString(q, 12) {
		t = strings.ToLower(strings.Trim(t, `"`))
		t = strings.TrimSpace(strings.Trim(t, ".,;:!?()[]{}"))
		if t != "" && !stop[t] {
			out = append(out, t)
		}
	}
	return out
}

func arxivQueryTerm(t string) string {
	if strings.Contains(t, " ") {
		return `all:"` + t + `"`
	}
	return "all:" + t
}

func arxivSearch(ctx context.Context, terms []string, from string, max int) ([]Paper, error) {
	parts := make([]string, len(terms))
	for i, t := range terms {
		parts[i] = arxivQueryTerm(t)
	}
	query := strings.Join(parts, " AND ")
	if from != "" {
		query += " AND submittedDate:[" + strings.ReplaceAll(from, "-", "") + "0000 TO " + strings.ReplaceAll(today(), "-", "") + "2359]"
	}
	q := url.Values{"search_query": {query}, "sortBy": {"relevance"}, "sortOrder": {"descending"}, "max_results": {strconv.Itoa(max)}}
	b, err := get(ctx, arxivBase+"/api/query?"+q.Encode())
	if err != nil {
		return nil, err
	}
	return parseArxiv(b)
}

// termsClause asks Europe PMC for papers with every keyword (or quoted phrase) in the title or abstract.
func termsClause(terms []string) string {
	parts := make([]string, len(terms))
	for i, t := range terms {
		t = strings.NewReplacer(`"`, "", `\`, "").Replace(t)
		parts[i] = `TITLE_ABS:"` + t + `"`
	}
	return "(" + strings.Join(parts, " AND ") + ")"
}

func searchTasks(q string, terms []string, from string, sel Selection) []task {
	tc, dc := termsClause(terms), dateClause(from)
	ts := []task{}
	if js := sel.Journals(0); len(js) > 0 {
		ts = append(ts, task{"Digest journals", func(c context.Context) ([]Paper, error) {
			return epmcPapers(c, epmcQuery(tc, journalClause(js), dc, withAbstract), "", 60, nil)
		}})
	}
	if sel.Has("other") {
		ts = append(ts, task{"Other journals (PubMed)", func(c context.Context) ([]Paper, error) {
			return epmcPapers(c, epmcQuery(tc, "SRC:MED", dc, withAbstract), "", 60, nil)
		}})
	}
	if sel.AnyServer() {
		ts = append(ts, task{"Preprint servers", func(c context.Context) ([]Paper, error) {
			return epmcPreprints(c, sel, from, "", tc+" AND "+withAbstract, 100)
		}})
	}
	if sel.Has("arxiv") {
		ts = append(ts, task{"arXiv", func(c context.Context) ([]Paper, error) { return arxivSearch(c, terms, from, 40) }})
	}
	return ts
}

func containsTerm(text, t string) bool {
	if strings.Contains(t, " ") || len(t) < 4 {
		return regexp.MustCompile(`\b` + regexp.QuoteMeta(t) + `\b`).MatchString(text)
	}
	return strings.Contains(text, t) // prefix-friendly: "edit" matches "editing"
}

// searchRank orders results by how well they match, then by venue and citations.
func searchRank(lists [][]Paper, terms []string, n int) []Story {
	return matchRank(lists, Query{Terms: terms}, n, nil)
}

// matchRank orders keyword matches. With a window context (custom categories) it also weighs
// recency or citations the way the Today/Week/Month/Year tabs do.
func matchRank(lists [][]Paper, q Query, n int, win *rankCtx) []Story {
	terms := q.Terms
	type hit struct {
		p   Paper
		rel float64
	}
	best := map[string]*hit{}
	titles := map[string]string{}
	maxCites, maxRate := 0, 0.0
	for _, list := range lists {
		for i, p := range list {
			if !digestable(p) { // research or a review with an abstract; no editorials, letters or short items
				continue
			}
			rel := 1 - float64(i)/float64(len(list)) // each source returns best matches first
			key := strings.TrimSpace(nonAlnum.ReplaceAllString(strings.ToLower(p.Title), " "))
			if id, ok := titles[key]; ok && id != p.ID {
				if h := best[id]; h != nil && rel > h.rel {
					h.rel = rel
				}
				continue
			}
			if junkRe.MatchString(p.Title) {
				continue
			}
			titles[key] = p.ID
			if h, ok := best[p.ID]; !ok {
				best[p.ID] = &hit{p, rel}
			} else if rel > h.rel {
				h.rel = rel
			}
			maxCites = max(maxCites, p.Citations)
			maxRate = math.Max(maxRate, citeRate(p))
		}
	}
	type scored struct {
		s     Story
		score float64
	}
	out := []scored{}
	for _, h := range best {
		title, all := strings.ToLower(h.p.Title), strings.ToLower(h.p.Title+" "+h.p.Abstract)
		inTitle, inAny := 0, 0
		for _, t := range terms {
			if containsTerm(title, t) {
				inTitle++
			}
			if containsTerm(all, t) {
				inAny++
			}
		}
		// Authors are checked against the paper's own author list, so a namesake in the text never counts.
		byAuthor, role, authOK := "", "", len(q.Authors) > 0
		for i, nm := range q.Authors {
			shown, r, ok := authorMatch(h.p, nm)
			if !ok {
				authOK = false
				break
			}
			if i == 0 {
				byAuthor, role = shown, r
			}
		}
		switch {
		case len(q.Authors) == 0 && inAny == 0:
			continue // matched only in full text; too loose to show
		case q.Auto && !authOK && inAny == 0:
			continue
		case len(q.Authors) > 0 && !q.Auto && (!authOK || (len(terms) > 0 && inAny == 0)):
			continue
		}
		cover, titleHit := 0.0, 0.0
		if len(terms) > 0 {
			cover, titleHit = float64(inAny)/float64(len(terms)), float64(inTitle)/float64(len(terms))
		}
		cites := 0.0
		if maxCites > 0 {
			cites = math.Log1p(float64(h.p.Citations)) / math.Log1p(float64(maxCites))
		}
		score := 0.35*h.rel + 0.25*cover + 0.15*titleHit + 0.15*venueValue(h.p) + 0.10*cites
		if win != nil {
			wc := *win
			wc.maxCites, wc.maxRate = maxCites, maxRate
			score = 0.25*h.rel + 0.20*cover + 0.10*titleHit + 0.20*venueValue(h.p) + 0.25*attention(h.p, wc)
		}
		if authOK && (len(terms) == 0 || q.Auto) { // papers by the author come first, newest first
			score = 1 + dateScore(h.p)
			if win != nil {
				wc := *win
				wc.maxCites, wc.maxRate = maxCites, maxRate
				score = 1 + 0.5*attention(h.p, wc) + 0.3*venueValue(h.p) + 0.2*h.rel
			}
		} else if authOK {
			score += 1
		}
		p := h.p
		out = append(out, scored{Story{ID: p.ID, Title: p.Title, Venue: p.Venue, Source: p.SourceID, Type: p.Type, Date: p.Date, DOI: p.DOI, URL: p.URL,
			Authors: p.Authors, Citations: p.Citations, Published: p.Published, Field: guessField(p), Score: math.Round(score*100) / 10,
			Signals: []Signal{}, KeySentence: matchSentence(p, terms), Abstract: p.Abstract, ByAuthor: byAuthor, AuthorRole: role, Kind: p.Kind}, score})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].s.Date > out[j].s.Date
	})
	chosen := []int{}
	for i := 0; i < len(out) && i < n; i++ {
		chosen = append(chosen, i)
	}
	chosen = coverWithin(chosen, len(out), n, func(i int) string { return out[i].s.Field }, func(i int) string { return paperClass(out[i].s.Type, out[i].s.Kind) })
	res := []Story{}
	for _, i := range chosen {
		res = append(res, out[i].s)
	}
	return res
}

// dateScore is between 0 and 1 and higher for newer papers.
func dateScore(p Paper) float64 {
	d, err := time.Parse("2006-01-02", p.Date)
	if err != nil {
		return 0
	}
	return math.Max(0, 1-time.Since(d).Hours()/24/36500)
}

// matchSentence prefers the abstract sentence that mentions the most search terms, so the reader sees why it matched.
func matchSentence(p Paper, terms []string) string {
	bestN, bestS := 0, ""
	for _, st := range abstractSentences(p.Abstract) { // headings removed; funding and registration lines skipped
		if st.Part == "other" {
			continue
		}
		s := st.S
		n, low := 0, strings.ToLower(s)
		for _, t := range terms {
			if containsTerm(low, t) {
				n++
			}
		}
		if n > bestN {
			bestN, bestS = n, s
		}
	}
	if bestS != "" {
		return bestS
	}
	return keySentence(p)
}

func buildSearch(ctx context.Context, raw, rng string, sel Selection) Feed {
	t0 := time.Now()
	q := parseQuery(raw)
	from := ""
	if d := searchRanges[rng]; d > 0 {
		from = daysAgo(d)
	}
	statuses, results := runTasks(ctx, queryTasks(q, from, sel))
	return Feed{Window: "search", Label: "Search · " + searchLabels[rng], From: from, Query: raw, Mode: q.Mode(), Authors: q.authorNames(),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339), TookMs: time.Since(t0).Milliseconds(), RankedBy: "relevance",
		Scanned: uniqueCount(results), Sources: statuses, Stories: matchRank(results, q, 40, nil)}
}

// buildTopic pulls papers for a reader-defined category within one time window, straight from the sources.
// Longer windows include everything the shorter ones found.
func buildTopic(ctx context.Context, raw, window string, sel Selection) Feed {
	return buildTopicN(ctx, raw, window, sel, storiesPerFeed, true)
}

func buildTopicN(ctx context.Context, raw, window string, sel Selection, n int, reviews bool) Feed {
	t0 := time.Now()
	q := parseQuery(raw)
	days := windowDays[window]
	statuses, results := nested(window, func(w string) pool {
		return pools.get("topic:"+w+":"+sel.Key()+":"+strings.ToLower(raw), func() pool {
			st, ls := runTasks(context.Background(), queryTasks(q, fromDate(windowDays[w]), sel))
			return pool{st, ls}
		})
	})
	wc := rankCtx{days: days, byCitations: byCitations(window), noReviews: !reviews}
	if !reviews {
		kept := make([][]Paper, len(results))
		for i, l := range results {
			kept[i] = withoutReviews(l)
		}
		results = kept
	}
	return Feed{Window: window, Label: windowLabel[window], From: fromDate(days), Query: raw, Mode: q.Mode(), Authors: q.authorNames(),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339), TookMs: time.Since(t0).Milliseconds(), RankedBy: "category",
		Scanned: uniqueCount(results), Sources: statuses, Stories: matchRank(results, q, n, &wc)}
}
