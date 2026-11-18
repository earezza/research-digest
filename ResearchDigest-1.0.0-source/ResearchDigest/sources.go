package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Paper is one candidate article from any source.
type Paper struct {
	ID, Title, Abstract, Venue, Type, Date, DOI, URL, Authors, FieldHint, Published string
	SourceID, FieldDefault                                                          string // journal ISSN, "biorxiv", "medrxiv", "arxiv" or "other"
	AuthorList                                                                      []string
	// Kind is "Review", "Letter", "Editorial"… for items that are not original research. The main tabs show
	// research only (with a real abstract); searches and your categories show everything an author published.
	Kind            string
	Citations, Tier int
	pre             float64
	fieldGuess      string
}

var httpClient = &http.Client{Transport: &http.Transport{
	Proxy: http.ProxyFromEnvironment, MaxIdleConnsPerHost: 16, IdleConnTimeout: 90 * time.Second, ForceAttemptHTTP2: true,
}}

const userAgent = "ResearchDigest/1.0 (desktop research newsletter)"

// Base URLs are variables so tests can point them at a local fake server.
var (
	epmcBase  = "https://www.ebi.ac.uk/europepmc/webservices/rest"
	rxivBase  = "https://api.biorxiv.org"
	arxivBase = "https://export.arxiv.org"
)

// For testing against a local stand-in only; normal use never sets these.
func init() {
	for env, dst := range map[string]*string{"DD_EPMC_BASE": &epmcBase, "DD_RXIV_BASE": &rxivBase, "DD_ARXIV_BASE": &arxivBase} {
		if v := os.Getenv(env); v != "" {
			*dst = v
		}
	}
}

func get(ctx context.Context, u string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("User-Agent", userAgent)
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("%s returned %d", req.URL.Host, res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 64<<20))
}

func today() string { return time.Now().UTC().Format("2006-01-02") }

// fromDate is the first day of a window; 0 days means all time (no start date).
func fromDate(days int) string {
	if days <= 0 {
		return ""
	}
	return daysAgo(days)
}
func daysAgo(n int) string { return time.Now().UTC().AddDate(0, 0, -n).Format("2006-01-02") }

var tagRe = regexp.MustCompile(`<[^>]+>`)
var wsRe = regexp.MustCompile(`\s+`)

// clean removes markup and decodes HTML codes such as &lt; and &#8201; (a thin space, used in "69 521").
func clean(s string) string {
	return strings.TrimSpace(wsRe.ReplaceAllString(html.UnescapeString(tagRe.ReplaceAllString(s, " ")), " "))
}

func firstAuthor(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return names[0] + " et al."
}

/* ---------------- Europe PMC (journals, and preprints for long ranges and searches) ---------------- */
// Europe PMC is free and needs no key or account. It indexes PubMed (all the digest's journals) and the
// bioRxiv, medRxiv and arXiv preprints, with abstracts and citation counts.

type flexStrings []string // a JSON string or list of strings

func (f *flexStrings) UnmarshalJSON(b []byte) error {
	var one string
	if json.Unmarshal(b, &one) == nil {
		*f = []string{one}
		return nil
	}
	var many []string
	err := json.Unmarshal(b, &many)
	*f = many
	return err
}

type epmcResult struct {
	ID                   string `json:"id"`
	Source               string `json:"source"`
	PMID                 string `json:"pmid"`
	DOI                  string `json:"doi"`
	Title                string `json:"title"`
	AuthorString         string `json:"authorString"`
	AbstractText         string `json:"abstractText"`
	FirstPublicationDate string `json:"firstPublicationDate"`
	PubYear              string `json:"pubYear"`
	CitedByCount         int    `json:"citedByCount"`
	JournalInfo          *struct {
		Journal struct {
			Title string `json:"title"`
			ISSN  string `json:"issn"`
			ESSN  string `json:"essn"`
		} `json:"journal"`
	} `json:"journalInfo"`
	BookOrReportDetails *struct {
		Publisher string `json:"publisher"`
	} `json:"bookOrReportDetails"`
	PubTypeList *struct {
		PubType flexStrings `json:"pubType"`
	} `json:"pubTypeList"`
}

var (
	notResearch = regexp.MustCompile(`(?i)\b(review|editorial|comment|news|letter|interview|biography|congress)\b`)
	neverShown  = regexp.MustCompile(`(?i)\b(erratum|correction|retraction|published erratum|expression of concern)\b`)
)

// digestable reports whether a paper is shown anywhere (tabs, searches and categories): original research or a
// review, with a real abstract. Editorials, news, comments, letters and short items without one are left out.
func digestable(p Paper) bool { return (p.Kind == "" || isReview(p)) && len(p.Abstract) > 200 }

// isReview: a review article (a journal paper that summarises published work). Reviews are their own paper
// type next to peer-reviewed research and preprints, and the reader can leave them out everywhere.
func isReview(p Paper) bool { return p.Kind == "Review" && p.Type != "preprint" }

// paperClass splits papers into the three types the page filters by: "research", "preprint" and "review".
func paperClass(typ, kind string) string {
	switch {
	case typ == "preprint":
		return "preprint"
	case kind == "Review":
		return "review"
	}
	return "research"
}

// withoutReviews drops review articles (when the reader turned reviews off).
func withoutReviews(ps []Paper) []Paper {
	out := ps[:0:0]
	for _, p := range ps {
		if !isReview(p) {
			out = append(out, p)
		}
	}
	return out
}

func epmcSearch(ctx context.Context, query, sortBy string, pageSize int) ([]epmcResult, error) {
	run := func(q, s string) ([]epmcResult, error) {
		v := url.Values{"query": {q}, "format": {"json"}, "resultType": {"core"}, "pageSize": {strconv.Itoa(pageSize)}}
		if s != "" {
			v.Set("sort", s)
		}
		b, err := get(ctx, epmcBase+"/search?"+v.Encode())
		if err != nil {
			return nil, err
		}
		var r struct {
			ResultList struct {
				Result []epmcResult `json:"result"`
			} `json:"resultList"`
		}
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, err
		}
		return r.ResultList.Result, nil
	}
	out, err := run(query, sortBy)
	if err != nil && sortBy != "" && ctx.Err() == nil { // older sort syntax as a fallback
		legacy := " sort_date:y"
		if strings.HasPrefix(sortBy, "CITED") {
			legacy = " sort_cited:y"
		}
		out, err = run(query+legacy, "")
	}
	return out, err
}

var rxivSuffix = regexp.MustCompile(`^10\.1101/\d{4}\.\d{2}\.\d{2}\.(\d+)`)

// preprintServer names the server behind a Europe PMC preprint record, or "" for servers the digest does not read.
func preprintServer(r epmcResult) string {
	pub := ""
	if r.BookOrReportDetails != nil {
		pub = strings.ToLower(r.BookOrReportDetails.Publisher)
	}
	doi := strings.ToLower(r.DOI)
	switch {
	case strings.Contains(pub, "medrxiv"):
		return "medrxiv"
	case strings.Contains(pub, "biorxiv"):
		return "biorxiv"
	case strings.Contains(pub, "arxiv") || strings.HasPrefix(doi, "10.48550/arxiv"):
		return "arxiv"
	case strings.HasPrefix(doi, "10.1101/"):
		if m := rxivSuffix.FindStringSubmatch(doi); m != nil && len(m[1]) == 8 {
			return "medrxiv" // medRxiv numbers have eight digits, bioRxiv six
		}
		return "biorxiv"
	}
	return ""
}

var smallWords = map[string]bool{"of": true, "and": true, "the": true, "in": true, "for": true, "on": true, "&": true}

func titleCase(s string) string {
	ws := strings.Fields(s)
	for i, w := range ws {
		if i > 0 && smallWords[strings.ToLower(w)] {
			ws[i] = strings.ToLower(w)
		} else if len(w) > 0 && w == strings.ToLower(w) {
			ws[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(ws, " ")
}

func fromEPMC(r epmcResult) (Paper, bool) {
	kind := ""
	if r.PubTypeList != nil {
		for _, t := range r.PubTypeList.PubType {
			if neverShown.MatchString(t) {
				return Paper{}, false
			}
			if m := notResearch.FindString(t); m != "" && kind == "" {
				kind = strings.ToUpper(m[:1]) + strings.ToLower(m[1:])
			}
		}
	}
	p := Paper{Title: clean(r.Title), Abstract: cleanAbstract(r.AbstractText), Date: firstNonEmpty(r.FirstPublicationDate, r.PubYear+"-01-01"),
		DOI: r.DOI, Citations: r.CitedByCount, Authors: epmcFirstAuthor(r.AuthorString), AuthorList: epmcAuthorList(r.AuthorString), Kind: kind}
	if p.Title == "" {
		return p, false
	}
	if r.Source == "PPR" {
		srv := preprintServer(r)
		if srv == "" {
			return p, false
		}
		p.Type, p.Tier, p.SourceID, p.Venue = "preprint", 3, srv, serverName[srv]
		if srv == "medrxiv" {
			p.FieldDefault = "medicine"
		}
	} else {
		p.Type, p.Tier, p.SourceID = "journal", 2, "other"
		if r.JournalInfo != nil {
			jr := r.JournalInfo.Journal
			p.Venue = titleCase(strings.TrimSuffix(regexp.MustCompile(`\s*\(.*\)$`).ReplaceAllString(jr.Title, ""), "."))
			for _, i := range []string{jr.ISSN, jr.ESSN} {
				if j, ok := issnInfo[strings.ToUpper(i)]; ok && i != "" {
					p.Venue, p.Tier, p.FieldHint, p.SourceID = j.Name, j.Tier, j.Field, j.ISSN
					break
				}
			}
		}
		p.Venue = firstNonEmpty(p.Venue, "Journal")
	}
	switch {
	case p.DOI != "":
		p.ID, p.URL = "doi:"+strings.ToLower(p.DOI), "https://doi.org/"+p.DOI
	case r.PMID != "":
		p.ID, p.URL = "pmid:"+r.PMID, "https://pubmed.ncbi.nlm.nih.gov/"+r.PMID+"/"
	default:
		p.ID, p.URL = "epmc:"+r.Source+":"+r.ID, "https://europepmc.org/article/"+r.Source+"/"+r.ID
	}
	return p, true
}

func epmcAuthorList(s string) []string {
	out := []string{}
	for _, a := range strings.Split(strings.TrimSuffix(strings.TrimSpace(s), "."), ", ") {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	return out
}

func epmcFirstAuthor(s string) string {
	s = strings.TrimSuffix(strings.TrimSpace(s), ".")
	if s == "" {
		return ""
	}
	parts := strings.Split(s, ", ")
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + " et al."
}

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}

func journalClause(js []journal) string {
	parts := make([]string, 0, 2*len(js))
	for _, j := range js {
		parts = append(parts, `ISSN:"`+j.ISSN+`"`, `ESSN:"`+j.ISSN+`"`)
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

func dateClause(from string) string {
	if from == "" {
		return ""
	}
	return "FIRST_PDATE:[" + from + " TO " + today() + "]"
}

func epmcQuery(parts ...string) string {
	keep := []string{}
	for _, p := range parts {
		if p != "" {
			keep = append(keep, p)
		}
	}
	return strings.Join(keep, " AND ")
}

// Conditions for the main tabs: papers with an abstract, and no reviews when the reader left them out (so they
// don't take candidate places). Searches and categories ask for papers with an abstract too; their reviews are
// left out afterwards if the reader turned them off.
const (
	withAbstract = "HAS_ABSTRACT:y"
	notReview    = `NOT PUB_TYPE:"review"`
)

func mainConditions(reviews bool) string {
	if reviews {
		return withAbstract
	}
	return withAbstract + " AND " + notReview
}

func epmcPapers(ctx context.Context, query, sortBy string, pageSize int, keep func(Paper) bool) ([]Paper, error) {
	rs, err := epmcSearch(ctx, query, sortBy, pageSize)
	if err != nil {
		return nil, err
	}
	out := []Paper{}
	for _, r := range rs {
		if p, ok := fromEPMC(r); ok && (keep == nil || keep(p)) {
			out = append(out, p)
		}
	}
	return out, nil
}

const sortNewest, sortCited = "P_PDATE_D desc", "CITED desc"

func epmcJournals(ctx context.Context, js []journal, from, sortBy, extra string, pageSize int) ([]Paper, error) {
	return epmcPapers(ctx, epmcQuery(journalClause(js), dateClause(from), extra), sortBy, pageSize, nil)
}

func epmcPreprints(ctx context.Context, sel Selection, from, sortBy, extra string, pageSize int) ([]Paper, error) {
	return epmcPapers(ctx, epmcQuery("SRC:PPR", extra, dateClause(from)), sortBy, pageSize, func(p Paper) bool { return sel.Has(p.SourceID) })
}

/* ---------------- which sources to read ---------------- */

var serverName = map[string]string{"biorxiv": "bioRxiv", "medrxiv": "medRxiv", "arxiv": "arXiv"}

// Selection is the set of sources the reader ticked; empty means every source.
type Selection struct{ ids map[string]bool }

func parseSelection(s string) Selection {
	sel := Selection{}
	for _, id := range strings.Split(s, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := issnInfo[strings.ToUpper(id)]; ok {
			id = strings.ToUpper(id)
		} else if _, ok := serverName[strings.ToLower(id)]; ok || strings.EqualFold(id, "other") {
			id = strings.ToLower(id)
		} else {
			continue
		}
		if sel.ids == nil {
			sel.ids = map[string]bool{}
		}
		sel.ids[id] = true
	}
	return sel
}

func (s Selection) All() bool          { return len(s.ids) == 0 }
func (s Selection) Has(id string) bool { return s.All() || s.ids[id] }
func (s Selection) Key() string {
	if s.All() {
		return "all"
	}
	ks := make([]string, 0, len(s.ids))
	for k := range s.ids {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return strings.Join(ks, ",")
}
func (s Selection) Journals(tier int) []journal {
	out := []journal{}
	for _, j := range journals {
		if (tier == 0 || j.Tier == tier) && s.Has(j.ISSN) {
			out = append(out, j)
		}
	}
	return out
}
func (s Selection) AnyServer() bool { return s.Has("biorxiv") || s.Has("medrxiv") || s.Has("arxiv") }

/* ---------------- bioRxiv / medRxiv ---------------- */

// flexInt accepts 123 or "123".
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	n, _ := strconv.Atoi(s)
	*f = flexInt(n)
	return nil
}

type rxivRow struct {
	DOI, Title, Authors, Date, Type, Category, Abstract, Published string
	Version                                                        flexInt
}
type rxivPage struct {
	Messages []struct {
		Total flexInt `json:"total"`
	} `json:"messages"`
	Collection []rxivRow `json:"collection"`
}

func rxiv(ctx context.Context, server, from string, maxPages int) ([]Paper, error) {
	base := fmt.Sprintf("%s/details/%s/%s/%s", rxivBase, server, from, today())
	fetch := func(cursor int) (rxivPage, error) {
		var p rxivPage
		b, err := get(ctx, fmt.Sprintf("%s/%d/json", base, cursor))
		if err == nil {
			err = json.Unmarshal(b, &p)
		}
		return p, err
	}
	first, err := fetch(0)
	if err != nil {
		return nil, err
	}
	total := 0
	if len(first.Messages) > 0 {
		total = int(first.Messages[0].Total)
	}
	pages := (total + 99) / 100
	n := min(maxPages, pages)
	picks := map[int]bool{}
	for i := 1; i < n; i++ { // spread pages across the date range
		picks[int(math.Round(float64(i*(pages-1))/float64(max(1, n-1))))] = true
	}
	delete(picks, 0)
	rows := append([]rxivRow{}, first.Collection...)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for pg := range picks {
		wg.Add(1)
		go func(pg int) {
			defer wg.Done()
			if p, err := fetch(pg * 100); err == nil {
				mu.Lock()
				rows = append(rows, p.Collection...)
				mu.Unlock()
			}
		}(pg)
	}
	wg.Wait()
	latest := map[string]rxivRow{}
	for _, r := range rows {
		if prev, ok := latest[r.DOI]; !ok || r.Version > prev.Version {
			latest[r.DOI] = r
		}
	}
	venue, hint := "bioRxiv", ""
	if server == "medrxiv" {
		venue = "medRxiv"
	}
	out := []Paper{}
	for _, r := range latest {
		if r.Type == "withdrawn" || len(r.Abstract) <= 200 {
			continue
		}
		names := []string{}
		for _, a := range strings.Split(r.Authors, ";") {
			if a = strings.TrimSpace(a); a != "" {
				names = append(names, a)
			}
		}
		fh := hint
		if mapped, ok := cat.RxivField[strings.ToLower(r.Category)]; ok {
			fh = mapped
		} else if server == "medrxiv" {
			fh = "medicine"
		}
		pub := ""
		if r.Published != "" && r.Published != "NA" {
			pub = r.Published
		}
		out = append(out, Paper{ID: "doi:" + strings.ToLower(r.DOI), Title: clean(r.Title), Abstract: cleanAbstract(r.Abstract), Venue: venue,
			Type: "preprint", Date: r.Date, DOI: r.DOI, URL: "https://doi.org/" + r.DOI, Authors: firstAuthor(names), Tier: 3,
			FieldHint: fh, Published: pub, SourceID: server, AuthorList: names})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out, nil
}

/* ---------------- arXiv ---------------- */

type arxivFeed struct {
	Entries []struct {
		ID        string `xml:"id"`
		Published string `xml:"published"`
		Title     string `xml:"title"`
		Summary   string `xml:"summary"`
		DOI       string `xml:"doi"`
		Authors   []struct {
			Name string `xml:"name"`
		} `xml:"author"`
		Primary struct {
			Term string `xml:"term,attr"`
		} `xml:"primary_category"`
	} `xml:"entry"`
}

var arxivVer = regexp.MustCompile(`v\d+$`)

func parseArxiv(b []byte) ([]Paper, error) {
	var f arxivFeed
	if err := xml.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	out := []Paper{}
	for _, e := range f.Entries {
		short := arxivVer.ReplaceAllString(regexp.MustCompile(`^https?://arxiv\.org/abs/`).ReplaceAllString(strings.TrimSpace(e.ID), ""), "")
		names := []string{}
		for _, a := range e.Authors {
			names = append(names, strings.TrimSpace(a.Name))
		}
		field := ""
		for k, v := range cat.ArxivField {
			if e.Primary.Term == k || strings.HasPrefix(e.Primary.Term, k) {
				field = v
				break
			}
		}
		date := e.Published
		if len(date) >= 10 {
			date = date[:10]
		}
		p := Paper{ID: "arxiv:" + short, Title: clean(e.Title), Abstract: cleanAbstract(e.Summary), Venue: "arXiv", Type: "preprint", Date: date,
			DOI: strings.TrimSpace(e.DOI), URL: "https://arxiv.org/abs/" + short, Authors: firstAuthor(names), Tier: 3, FieldHint: field, SourceID: "arxiv", AuthorList: names}
		if p.Title != "" && len(p.Abstract) > 200 {
			out = append(out, p)
		}
	}
	return out, nil
}

func arxiv(ctx context.Context, from string, maxResults int) ([]Paper, error) {
	cats := make([]string, len(cat.ArxivCategories))
	for i, c := range cat.ArxivCategories {
		cats[i] = "cat:" + c
	}
	rng := fmt.Sprintf("submittedDate:[%s0000 TO %s2359]", strings.ReplaceAll(from, "-", ""), strings.ReplaceAll(today(), "-", ""))
	q := url.Values{"search_query": {"(" + strings.Join(cats, " OR ") + ") AND " + rng}, "sortBy": {"submittedDate"},
		"sortOrder": {"descending"}, "max_results": {strconv.Itoa(maxResults)}}
	b, err := get(ctx, arxivBase+"/api/query?"+q.Encode())
	if err != nil {
		return nil, err
	}
	return parseArxiv(b)
}

/* ---------------- per-window plan ---------------- */

type task struct {
	name string
	run  func(context.Context) ([]Paper, error)
}

// candidates is how many papers to fetch per journal group: enough for a range showing n papers
// (150 most cited, or the newest 200 or 300 for Today and This week; more when the reader asks for over 75).
func candidates(window string, n int) int {
	base := 150
	switch window {
	case "day":
		base = 200
	case "week":
		base = 300
	}
	return min(max(base, 2*n), 1000)
}

func plan(window string, sel Selection) []task {
	return planSized(window, sel, candidates(window, storiesPerFeed), true)
}

func planSized(window string, sel Selection, perPage int, reviews bool) []task {
	cond := mainConditions(reviews)
	days := windowDays[window]
	from := fromDate(days)
	ts := []task{}
	short := window == "day" || window == "week"
	order := sortCited
	if short {
		order = sortNewest
	}
	for _, tier := range []int{1, 2} {
		if js := sel.Journals(tier); len(js) > 0 {
			name := map[int]string{1: "Leading journals", 2: "Specialist journals"}[tier]
			ts = append(ts, task{name, func(c context.Context) ([]Paper, error) { return epmcJournals(c, js, from, order, cond, perPage) }})
		}
	}
	if short { // the preprint servers' own APIs have the newest papers
		wide := window == "week"
		pick := func(a, b int) int {
			if wide {
				return b
			}
			return a
		}
		if sel.Has("biorxiv") {
			ts = append(ts, task{"bioRxiv", func(c context.Context) ([]Paper, error) { return rxiv(c, "biorxiv", from, pick(3, 6)) }})
		}
		if sel.Has("medrxiv") {
			ts = append(ts, task{"medRxiv", func(c context.Context) ([]Paper, error) { return rxiv(c, "medrxiv", from, pick(2, 3)) }})
		}
		if sel.Has("arxiv") {
			ts = append(ts, task{"arXiv", func(c context.Context) ([]Paper, error) { return arxiv(c, from, pick(150, 300)) }})
		}
		return ts
	}
	if sel.AnyServer() {
		ts = append(ts, task{"Preprint servers", func(c context.Context) ([]Paper, error) {
			return epmcPreprints(c, sel, from, sortCited, withAbstract, 300)
		}})
	}
	return ts
}
