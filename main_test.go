package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

func abs(topic string) string {
	return fmt.Sprintf("%s remains poorly understood despite decades of work. We combined large datasets with new experiments across 40 samples. "+
		"Here we show that %s is controlled by a previously unknown mechanism that explains most of the observed variation. "+
		"The effect was reproduced in two independent cohorts and in a mouse model (e.g. Fig. 2). These results open new routes for diagnosis.", topic, topic)
}

// rec builds a Europe PMC "core" result.
func rec(src, doi, title, journal, issn, essn, publisher string, cites int, pubType ...string) map[string]any {
	r := map[string]any{"id": doi, "source": src, "doi": doi, "title": title, "authorString": "Researcher A, Colleague B.",
		"abstractText": "<h4>Background</h4>" + abs(strings.ToLower(title)), "firstPublicationDate": today(), "pubYear": today()[:4], "citedByCount": cites,
		"pubTypeList": map[string]any{"pubType": append([]string{"research-article"}, pubType...)}}
	if journal != "" {
		r["journalInfo"] = map[string]any{"journal": map[string]any{"title": journal, "issn": issn, "essn": essn}}
	}
	if publisher != "" {
		r["bookOrReportDetails"] = map[string]any{"publisher": publisher}
	}
	return r
}

var epmcQueries []string
var epmcRejectSort bool

func fakeUpstream(t *testing.T, arxivDown bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == "/search":
			q, srt := r.URL.Query().Get("query"), r.URL.Query().Get("sort")
			epmcQueries = append(epmcQueries, q+" | sort="+srt)
			if epmcRejectSort && srt != "" {
				w.WriteHeader(400)
				return
			}
			var res []any
			switch {
			case strings.Contains(q, "SRC:PPR"):
				res = []any{
					rec("PPR", "10.1101/2026.10.22.760932", "Single-cell proteomics foundation model for cells", "", "", "", "bioRxiv", 3),
					rec("PPR", "10.1101/2026.10.14.26365590", "Measles outbreaks and county vaccination coverage nationwide", "", "", "", "", 1),
					rec("PPR", "10.21203/rs.3.rs-1", "A Research Square preprint about genome cells", "", "", "", "Research Square", 9)}
			case strings.Contains(q, `ISSN:"0028-0836"`):
				res = []any{
					rec("MED", "10.1000/t.1", "Neuronal genome surveillance in the human brain", "Nature", "0028-0836", "1476-4687", "", 40),
					rec("MED", "10.1000/t.2", "Randomized phase 3 trial of a bispecific antibody", "The New England journal of medicine", "0028-4793", "1533-4406", "", 12),
					rec("MED", "10.1000/t.3", "Author Correction: an earlier paper", "Nature", "0028-0836", "", "", 0, "Published Erratum"),
					rec("MED", "10.1000/t.5", "A review of cell biology", "Nature", "0028-0836", "", "", 99, "Review")}
			case strings.Contains(q, `ESSN:"1474-760X"`):
				res = []any{rec("MED", "10.1000/t.4", "A toolkit for spatial transcriptomics", "Genome biology", "1474-7596", "1474-760X", "", 5)}
			case strings.Contains(q, "SRC:MED"):
				res = []any{rec("MED", "10.1000/t.9", "Genome surveillance of neurons in a regional cohort", "journal of unknown studies", "9999-9999", "", "", 2)}
			}
			if i := strings.Index(q, "(TITLE_ABS:"); i >= 0 { // keep only records that contain every term
				terms := regexp.MustCompile(`TITLE_ABS:"([^"]+)"`).FindAllStringSubmatch(q, -1)
				kept := []any{}
				for _, x := range res {
					m := x.(map[string]any)
					text := strings.ToLower(m["title"].(string) + " " + m["abstractText"].(string))
					ok := true
					for _, t := range terms {
						ok = ok && strings.Contains(text, t[1])
					}
					if ok {
						kept = append(kept, x)
					}
				}
				res = kept
			}
			json.NewEncoder(w).Encode(map[string]any{"hitCount": len(res), "resultList": map[string]any{"result": res}})
		case strings.HasPrefix(p, "/details/"):
			srv := "biorxiv"
			if strings.Contains(p, "medrxiv") {
				srv = "medrxiv"
			}
			json.NewEncoder(w).Encode(map[string]any{"messages": []any{map[string]any{"total": "2"}}, "collection": []any{
				map[string]any{"doi": "10.1101/" + srv + ".1", "title": "Single-cell atlas of the developing heart " + srv, "authors": "Lee, J.; Kim, S.",
					"date": today(), "version": "2", "type": "new results", "category": "genomics", "abstract": abs("heart development"), "published": "NA"},
				map[string]any{"doi": "10.1101/" + srv + ".1", "title": "older version", "authors": "x", "date": today(), "version": "1",
					"type": "new results", "category": "genomics", "abstract": abs("x"), "published": "NA"}}})
		case strings.HasPrefix(p, "/api/query"):
			if arxivDown {
				w.WriteHeader(503)
				return
			}
			io.WriteString(w, `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:arxiv="http://arxiv.org/schemas/atom"><entry>
<id>http://arxiv.org/abs/2610.11658v1</id><published>`+today()+`T10:00:00Z</published><title>Genome-Wide Maps &amp; Logic of Enhancers</title>
<summary>`+abs("enhancer genome regulation")+`</summary><author><name>C. McLauchlan</name></author><author><name>B. Brown</name></author>
<arxiv:primary_category term="q-bio.GN"/></entry></feed>`)
		default:
			w.WriteHeader(404)
		}
	}))
}

func setup(t *testing.T, arxivDown bool) {
	loadCatalog()
	up := fakeUpstream(t, arxivDown)
	t.Cleanup(up.Close)
	epmcBase, rxivBase, arxivBase = up.URL, up.URL, up.URL
	resetCaches()
	epmcQueries, epmcRejectSort = nil, false
}

func TestSentences(t *testing.T) {
	ss := sentences("Cells divide (e.g. in Fig. 2) quickly and well. Here we show X is true in 40 mice. Done.")
	if len(ss) != 2 || !strings.Contains(ss[0], "e.g. in Fig. 2") {
		t.Fatalf("got %q", ss)
	}
}

func TestDayFeed(t *testing.T) {
	setup(t, true)
	f := getFeed("day", Selection{})
	var arx *sourceStatus
	for i := range f.Sources {
		if f.Sources[i].Name == "arXiv" {
			arx = &f.Sources[i]
		}
	}
	if arx == nil || arx.OK {
		t.Fatalf("arXiv should be reported as failed: %+v", f.Sources)
	}
	if len(f.Stories) < 4 {
		t.Fatalf("want >=4 stories, got %d", len(f.Stories))
	}
	ids := map[string]int{}
	for i, s := range f.Stories {
		ids[s.ID]++
		if strings.Contains(s.Title, "Correction") {
			t.Fatal("correction not filtered")
		}
		if !strings.HasPrefix(s.URL, "https://") || len(s.Signals) != 3 || len(s.KeySentence) < 20 {
			t.Fatalf("bad story %+v", s)
		}
		if i > 0 && s.Score > f.Stories[i-1].Score {
			t.Fatal("not sorted by score")
		}
	}
	if ids["doi:10.1101/biorxiv.1"] != 1 {
		t.Fatal("bioRxiv versions not deduplicated")
	}
	if !strings.HasPrefix(f.Stories[0].KeySentence, "Here we show") {
		t.Fatalf("key sentence: %q", f.Stories[0].KeySentence)
	}
}

func TestArxivAndYear(t *testing.T) {
	setup(t, false)
	f := getFeed("week", Selection{})
	found := false
	for _, s := range f.Stories {
		if s.ID == "arxiv:2610.11658" && s.Title == "Genome-Wide Maps & Logic of Enhancers" && s.Field == "omics" && s.Authors == "C. McLauchlan et al." {
			found = true
		}
	}
	if !found {
		t.Fatalf("arXiv entry missing or mis-parsed: %+v", f.Stories)
	}
	y := getFeed("year", Selection{})
	if y.Stories[0].Signals[1].Label != "Citations" {
		t.Fatal("year view should rank by citations")
	}
}

func TestHTTP(t *testing.T) {
	setup(t, false)
	srv := httptest.NewServer(newMux())
	defer srv.Close()
	res, _ := http.Get(srv.URL + "/")
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(b), "Research <span>Digest") {
		t.Fatal("index not served")
	}
	res, _ = http.Get(srv.URL + "/api/feed?window=decade")
	if res.StatusCode != 400 {
		t.Fatal("bad window should 400")
	}
	req, _ := http.NewRequest("GET", srv.URL+"/api/feed?window=month", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	res, _ = http.DefaultTransport.RoundTrip(req)
	if res.Header.Get("Content-Encoding") != "gzip" {
		t.Fatal("feed not gzipped")
	}
	res, _ = http.Get(srv.URL + "/../main.go")
	if res.StatusCode == 200 {
		b, _ := io.ReadAll(res.Body)
		if strings.Contains(string(b), "package main") {
			t.Fatal("path traversal")
		}
	}
}

func TestSearch(t *testing.T) {
	setup(t, false)
	if got := searchTerms(`the "Spatial Transcriptomics" of CRISPR, editing`); strings.Join(got, "|") != "spatial transcriptomics|crispr|editing" {
		t.Fatalf("terms: %q", got)
	}
	f := buildSearch(context.Background(), "genome surveillance", "year", Selection{})
	if f.Window != "search" || f.Query != "genome surveillance" || len(f.Stories) == 0 {
		t.Fatalf("bad search feed: %+v", f)
	}
	if !strings.Contains(strings.ToLower(f.Stories[0].Title), "genome surveillance") {
		t.Fatalf("best match should rank first, got %q", f.Stories[0].Title)
	}
	for _, s := range f.Stories {
		low := strings.ToLower(s.Title + " " + s.Abstract)
		if !strings.Contains(low, "genome") && !strings.Contains(low, "surveillance") {
			t.Fatalf("unrelated result %q", s.Title)
		}
	}
	srv := httptest.NewServer(newMux())
	defer srv.Close()
	res, _ := http.Get(srv.URL + "/api/search?q=%20")
	if res.StatusCode != 400 {
		t.Fatal("empty query should 400")
	}
	res, _ = http.Get(srv.URL + "/api/search?q=enhancer+genome&range=month")
	var got Feed
	json.NewDecoder(res.Body).Decode(&got)
	if res.StatusCode != 200 || len(got.Stories) == 0 || got.Stories[0].ID != "arxiv:2610.11658" {
		t.Fatalf("arXiv search result missing: %d %+v", res.StatusCode, got.Stories)
	}
}

func TestTopic(t *testing.T) {
	setup(t, false)
	f := buildTopic(context.Background(), "neuronal genome", "week", Selection{})
	if f.Window != "week" || f.Query != "neuronal genome" || len(f.Stories) == 0 || !strings.Contains(f.Stories[0].Title, "Neuronal genome") {
		t.Fatalf("topic feed: %+v", f)
	}
	y := buildTopic(context.Background(), "neuronal genome", "year", Selection{})
	if y.From != daysAgo(365) {
		t.Fatal("year topic should cover 365 days")
	}
	srv := httptest.NewServer(newMux())
	defer srv.Close()
	if res, _ := http.Get(srv.URL + "/api/topic?q=x&window=decade"); res.StatusCode != 400 {
		t.Fatal("bad window should 400")
	}
	if res, _ := http.Get(srv.URL + "/api/topic?q=quantum&window=day"); res.StatusCode != 200 {
		t.Fatal("topic route failed")
	}
}

func TestLongWindowsAndFields(t *testing.T) {
	setup(t, false)
	if fromDate(0) != "" || windowDays["all"] != 0 || windowDays["5y"] != 1826 {
		t.Fatal("all-time and 5-year windows not configured")
	}
	f := getFeed("all", Selection{})
	if f.From != "" || len(f.Stories) == 0 || f.Stories[0].Signals[1].Label != "Citations" {
		t.Fatalf("all-time feed: %+v", f)
	}
	five := getFeed("5y", Selection{})
	if five.From != daysAgo(1826) {
		t.Fatal("5-year feed range")
	}
	ids := []string{}
	for _, fl := range cat.Fields {
		ids = append(ids, fl.ID)
	}
	if strings.Join(ids, ",") != "bioeng,bioinfo,biotech,cellmol,medicine,omics,publichealth" {
		t.Fatalf("fields not alphabetical/new: %v", ids)
	}
	if guessField(Paper{Title: "Dark matter halos", Abstract: "We simulate galaxies and dark matter."}) != "" {
		t.Fatal("physics should fall outside the digest's fields")
	}
	if guessField(Paper{Title: "Measles outbreaks", Abstract: "County-level vaccination coverage predicts outbreaks nationwide."}) != "publichealth" {
		t.Fatal("public health rule")
	}
	if guessField(Paper{Title: "Injectable hydrogel scaffolds", Abstract: "A biomaterial scaffold for tissue engineering."}) != "bioeng" {
		t.Fatal("bioengineering rule")
	}
}

func TestCategoriesSaved(t *testing.T) {
	setup(t, false)
	dir := t.TempDir()
	categoriesPath, legacyPath = dir+"/ResearchDigest/categories.json", dir+"/DailyDigest/categories.json"
	srv := httptest.NewServer(newMux())
	defer srv.Close()
	put := func(body, ctype string) *http.Response {
		req, _ := http.NewRequest("PUT", srv.URL+"/api/categories", strings.NewReader(body))
		req.Header.Set("Content-Type", ctype)
		res, _ := http.DefaultClient.Do(req)
		return res
	}
	if res := put(`{"categories":[]}`, "text/plain"); res.StatusCode != 415 {
		t.Fatal("non-JSON writes must be refused")
	}
	res := put(`{"categories":[{"id":"abc1","q":"CRISPR delivery"},{"id":"BAD ID","q":"x"},{"id":"abc2","q":"crispr delivery"},{"id":"abc3","q":"\"organoids\" kidney"}]}`, "application/json")
	if res.StatusCode != 200 {
		t.Fatalf("save failed: %d", res.StatusCode)
	}
	got := loadCategories() // as a fresh start would read it
	if len(got) != 2 || got[0].Q != "CRISPR delivery" || got[1].Q != `"organoids" kidney` {
		t.Fatalf("saved categories: %+v", got)
	}
	res, _ = http.Get(srv.URL + "/api/categories")
	var body struct{ Categories []Category }
	json.NewDecoder(res.Body).Decode(&body)
	if len(body.Categories) != 2 {
		t.Fatal("GET should return saved categories")
	}
}

func TestStream(t *testing.T) {
	setup(t, false)
	srv := httptest.NewServer(newMux())
	defer srv.Close()
	res, _ := http.Get(srv.URL + "/api/stream?windows=day,all,bogus")
	b, _ := io.ReadAll(res.Body)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if res.StatusCode != 200 || len(lines) != 2 {
		t.Fatalf("stream: %d %q", res.StatusCode, b)
	}
	seen := map[string]bool{}
	for _, l := range lines {
		var x struct {
			Window string
			Feed   Feed
		}
		if err := json.Unmarshal([]byte(l), &x); err != nil || x.Feed.Window != x.Window {
			t.Fatalf("bad line %q", l)
		}
		seen[x.Window] = true
	}
	if !seen["day"] || !seen["all"] {
		t.Fatal("missing windows")
	}
	res, _ = http.Get(srv.URL + "/api/stream?windows=week&q=genome")
	b, _ = io.ReadAll(res.Body)
	if !strings.Contains(string(b), `"query":"genome"`) {
		t.Fatal("topic stream")
	}
	if res, _ := http.Get(srv.URL + "/api/stream?windows=nope"); res.StatusCode != 400 {
		t.Fatal("bad windows should 400")
	}
}

func TestEuropePMC(t *testing.T) {
	setup(t, false)
	f := getFeed("year", Selection{})
	bySource := map[string]Story{}
	for _, s := range f.Stories {
		bySource[s.Source] = s
		if strings.Contains(s.Title, "Correction") || strings.Contains(s.Title, "Research Square") {
			t.Fatalf("should be filtered: %q", s.Title)
		}
		if strings.Contains(s.Title, "review") && s.Kind != "Review" {
			t.Fatalf("a review should be labelled as one: %q (%q)", s.Title, s.Kind)
		}
	}
	if s := bySource["0028-4793"]; s.Venue != "New England Journal of Medicine" || s.Authors != "Researcher A et al." || strings.Contains(s.Abstract, "<h4>") {
		t.Fatalf("NEJM record: %+v", s)
	}
	if s := bySource["1474-760X"]; s.Venue != "Genome Biology" {
		t.Fatalf("electronic ISSN match: %+v", bySource)
	}
	byID := map[string]Story{}
	for _, s := range f.Stories {
		byID[s.ID] = s
	}
	if s := byID["doi:10.1101/2026.10.14.26365590"]; s.Venue != "medRxiv" || s.Type != "preprint" || s.Field != "publichealth" {
		t.Fatalf("medRxiv from DOI: %+v", s)
	}
	if bySource["biorxiv"].Venue != "bioRxiv" {
		t.Fatal("bioRxiv preprint missing")
	}
	q := strings.Join(epmcQueries, "\n")
	if !strings.Contains(q, "sort=CITED desc") || !strings.Contains(q, "FIRST_PDATE:["+daysAgo(365)+" TO "+today()+"]") || !strings.Contains(q, "HAS_ABSTRACT:y") || strings.Contains(q, `NOT PUB_TYPE:"review"`) {
		t.Fatalf("year queries ask for abstracts and include reviews: %s", q)
	}
	epmcQueries = nil
	getFeedN("year", Selection{}, 30, false)
	if q := strings.Join(epmcQueries, "\n"); !strings.Contains(q, `NOT PUB_TYPE:"review"`) {
		t.Fatalf("with reviews left out, the main tabs exclude them in the query: %s", q)
	}
	epmcQueries = nil
	buildTopic(context.Background(), "Sarah Hachmer", "year", Selection{})
	if q := strings.Join(epmcQueries, "\n"); !strings.Contains(q, "HAS_ABSTRACT:y") || strings.Contains(q, "PUB_TYPE") {
		t.Fatalf("searches ask for papers with an abstract and never exclude reviews in the query: %s", q)
	}
	epmcQueries = nil
	getFeed("all", Selection{})
	undated := false
	for _, q := range epmcQueries {
		undated = undated || (strings.Contains(q, `ISSN:"0028-0836"`) && !strings.Contains(q, "FIRST_PDATE"))
	}
	if !undated {
		t.Fatal("all time's own journal query must not limit dates")
	}
}

func TestEuropePMCSortFallback(t *testing.T) {
	setup(t, false)
	epmcRejectSort = true
	f := getFeed("month", Selection{})
	if len(f.Stories) == 0 || !strings.Contains(strings.Join(epmcQueries, "\n"), "sort_cited:y") {
		t.Fatalf("fallback sort not used: %v", epmcQueries)
	}
}

func TestSourceSelection(t *testing.T) {
	setup(t, false)
	sel := parseSelection("1474-760x, nonsense ,ARXIV")
	if sel.Key() != "1474-760X,arxiv" {
		t.Fatalf("selection: %q", sel.Key())
	}
	f := getFeed("week", sel)
	for _, s := range f.Stories {
		if s.Source != "1474-760X" && s.Source != "arxiv" {
			t.Fatalf("story from unselected source: %+v", s)
		}
	}
	names := []string{}
	for _, st := range f.Sources {
		names = append(names, st.Name)
	}
	if strings.Join(names, ",") != "Specialist journals,arXiv" {
		t.Fatalf("only selected sources should be read: %v", names)
	}
	q := strings.Join(epmcQueries, "\n")
	if strings.Contains(q, "0028-0836") || !strings.Contains(q, `ESSN:"1474-760X"`) {
		t.Fatalf("queries should name only the selected journal: %s", q)
	}
	// A search limited to preprint servers never asks for journals.
	epmcQueries = nil
	s := buildSearch(context.Background(), "proteomics", "all", parseSelection("biorxiv"))
	if len(s.Stories) != 1 || s.Stories[0].Source != "biorxiv" || strings.Contains(strings.Join(epmcQueries, ""), "ISSN") {
		t.Fatalf("server-only search: %+v %v", s.Stories, epmcQueries)
	}
	// "Other journals" reaches beyond the digest's list, with a readable name.
	o := buildSearch(context.Background(), "genome surveillance", "year", parseSelection("other"))
	if len(o.Stories) != 1 || o.Stories[0].Venue != "Journal of Unknown Studies" || o.Stories[0].Source != "other" {
		t.Fatalf("other journals: %+v", o.Stories)
	}
}

func TestLongerRangesContainShorterOnes(t *testing.T) {
	setup(t, false)
	ids := func(f Feed) map[string]bool {
		m := map[string]bool{}
		for _, s := range f.Stories {
			m[s.ID] = true
		}
		return m
	}
	prev := map[string]bool{}
	for _, w := range []string{"day", "week", "month", "year", "5y", "all"} {
		cur := ids(getFeed(w, Selection{}))
		for id := range prev {
			if !cur[id] {
				t.Fatalf("%s is missing %s, which a shorter range showed", w, id)
			}
		}
		prev = cur
	}
	// Today's bioRxiv preprint (read from bioRxiv itself) also reaches This year.
	if !ids(getFeed("year", Selection{}))["doi:10.1101/biorxiv.1"] {
		t.Fatal("year should include today's preprints")
	}
	// Categories nest the same way.
	m, y := buildTopic(context.Background(), "genome", "month", Selection{}), buildTopic(context.Background(), "genome", "year", Selection{})
	for id := range ids(m) {
		if !ids(y)[id] {
			t.Fatalf("category: year missing %s", id)
		}
	}
}

func TestCitationRate(t *testing.T) {
	recent := Paper{Citations: 6, Date: daysAgo(20)}
	old := Paper{Citations: 24, Date: daysAgo(360)}
	if citeRate(recent) <= citeRate(old) {
		t.Fatalf("6 citations in 3 weeks should outrank 24 in a year: %.2f vs %.2f", citeRate(recent), citeRate(old))
	}
}

func TestLimitsOnlyWhenFull(t *testing.T) {
	loadCatalog()
	ps := []Paper{}
	for i := 0; i < 10; i++ { // ten papers from one journal: more than the per-journal limit, fewer than the tab holds
		ps = append(ps, Paper{ID: fmt.Sprint("p", i), Title: fmt.Sprint("Trial ", i, " in patients"), Abstract: abs("therapy"), Venue: "NEJM", Type: "journal",
			Date: today(), Tier: 1, FieldHint: "medicine"})
	}
	if got := rank(ps, rankCtx{days: 30, byCitations: true}, 30); len(got) != 10 {
		t.Fatalf("all 10 should show when there is room, got %d", len(got))
	}
	if got := rank(ps, rankCtx{days: 30, byCitations: true}, 8); len(got) != 8 {
		t.Fatalf("a full tab still fills: %d", len(got))
	}
}

func TestParseNames(t *testing.T) {
	cases := map[string][2]string{
		"Jennifer A. Doudna": {"Doudna", "JA"}, "J. A. Doudna": {"Doudna", "JA"}, "Doudna JA": {"Doudna", "JA"},
		"Doudna, Jennifer": {"Doudna", "J"}, "Doudna": {"Doudna", ""}, "Johannes van der Waals": {"van der Waals", "J"},
		"Lee, J.": {"Lee", "J"}, "Jean-Luc Picard": {"Picard", "JL"},
	}
	for in, want := range cases {
		n, ok := parseName(in)
		if !ok || n.Surname != want[0] || n.Initials != want[1] {
			t.Errorf("parseName(%q) = %+v, want %v", in, n, want)
		}
	}
	for _, s := range []string{"Jennifer Doudna", "Doudna JA", "Feng Zhang", "J. A. Doudna"} {
		if !looksLikeName(s) {
			t.Errorf("%q should look like a name", s)
		}
	}
	for _, s := range []string{"gene therapy", "CRISPR delivery in mice", `"spatial transcriptomics"`, "covid 19", "Doudna"} {
		if looksLikeName(s) {
			t.Errorf("%q should not look like a name", s)
		}
	}
	q := parseQuery(`CRISPR author:"Jennifer Doudna" by:Zhang`)
	if strings.Join(q.Terms, ",") != "crispr" || len(q.Authors) != 2 || q.Auto || q.Mode() != "author" {
		t.Fatalf("parseQuery: %+v", q)
	}
	if a := parseQuery("Feng Zhang"); !a.Auto || a.Mode() != "both" || a.Authors[0].Surname != "Zhang" {
		t.Fatalf("auto name: %+v", a)
	}
	if c := authorClause([]Name{{Surname: "Doudna", Initials: "JA"}}); c != `((AUTH:"Doudna JA" OR AUTH:"Doudna J"))` {
		t.Fatalf("clause %s", c)
	}
}

func TestAuthorMatch(t *testing.T) {
	p := Paper{AuthorList: []string{"Smith J", "Doudna JA", "Lee K"}}
	if shown, role, ok := authorMatch(p, Name{Surname: "doudna", Initials: "J"}); !ok || shown != "Doudna JA" || role != "author 2 of 3" {
		t.Fatalf("got %q %q %v", shown, role, ok)
	}
	if _, _, ok := authorMatch(p, Name{Surname: "Doudna", Initials: "K"}); ok {
		t.Fatal("different first initial must not match")
	}
	if _, role, _ := authorMatch(Paper{AuthorList: []string{"Jennifer A. Doudna", "Feng Zhang"}}, Name{Surname: "Zhang", Initials: "F"}); role != "last of 2 authors" {
		t.Fatalf("role %q", role)
	}
	if _, _, ok := authorMatch(Paper{AuthorList: []string{"Müller AB"}}, Name{Surname: "Muller"}); !ok {
		t.Fatal("accents should be ignored")
	}
}

func TestAuthorSearch(t *testing.T) {
	setup(t, false)
	f := buildSearch(context.Background(), `author:"Colleague B"`, "year", Selection{})
	if f.Mode != "author" || len(f.Stories) == 0 {
		t.Fatalf("author search: %+v", f)
	}
	for i, s := range f.Stories {
		if s.ByAuthor != "Colleague B" || s.AuthorRole != "last of 2 authors" {
			t.Fatalf("story %d not attributed: %+v", i, s)
		}
		if i > 0 && s.Date > f.Stories[i-1].Date {
			t.Fatal("author results should be newest first")
		}
	}
	q := strings.Join(epmcQueries, "\n")
	if !strings.Contains(q, `AUTH:"Colleague B"`) || !strings.Contains(q, "sort=P_PDATE_D desc") {
		t.Fatalf("author queries: %s", q)
	}
	if n := buildSearch(context.Background(), "author:Nobody", "year", Selection{}); len(n.Stories) != 0 {
		t.Fatalf("names not in the author list must not match: %+v", n.Stories)
	}
	// Keywords and author together: only that author's papers that mention the keyword.
	m := buildSearch(context.Background(), "genome author:Researcher", "year", Selection{})
	for _, s := range m.Stories {
		if s.ByAuthor == "" || !strings.Contains(strings.ToLower(s.Title+s.Abstract), "genome") {
			t.Fatalf("mixed search: %+v", s)
		}
	}
	// Categories can follow an author too.
	if c := buildTopic(context.Background(), `author:"Colleague B"`, "month", Selection{}); c.Mode != "author" || len(c.Stories) == 0 {
		t.Fatalf("author category: %+v", c)
	}
	// The search route accepts an author-only query.
	srv := httptest.NewServer(newMux())
	defer srv.Close()
	if res, _ := http.Get(srv.URL + "/api/search?q=" + url.QueryEscape("author:Doudna")); res.StatusCode != 200 {
		t.Fatal("author-only search refused")
	}
}

func TestCategoriesFromOldName(t *testing.T) {
	dir := t.TempDir()
	categoriesPath, legacyPath = dir+"/ResearchDigest/categories.json", dir+"/DailyDigest/categories.json"
	os.MkdirAll(dir+"/DailyDigest", 0o700)
	os.WriteFile(legacyPath, []byte(`[{"id":"old1","q":"CRISPR delivery"}]`), 0o600)
	if got := loadCategories(); len(got) != 1 || got[0].Q != "CRISPR delivery" {
		t.Fatalf("old categories not picked up: %+v", got)
	}
	saveCategories(loadCategories())
	if _, err := os.Stat(categoriesPath); err != nil {
		t.Fatal("should be written to the new folder")
	}
}

// A longer range's main list can be all highly cited papers from other fields; a field or paper-type filter
// must still find the papers a shorter range showed, so each field keeps a few of each type as extras.
func TestFilterNeverEmptiesLongerRange(t *testing.T) {
	loadCatalog()
	ps := []Paper{}
	for i := 0; i < 60; i++ {
		ps = append(ps, Paper{ID: fmt.Sprint("m", i), Title: fmt.Sprint("Landmark trial ", i), Abstract: abs("therapy"), Venue: fmt.Sprint("Journal ", i%9),
			Type: "journal", Date: "2012-01-01", Citations: 5000 + i, Tier: 1, FieldHint: []string{"medicine", "cellmol", "omics", "publichealth"}[i%4]})
	}
	ps = append(ps, Paper{ID: "new", Title: "A new microfluidic sensor", Abstract: abs("sensor"), Venue: "bioRxiv", Type: "preprint",
		Date: today(), Citations: 0, Tier: 3, FieldHint: "bioeng"})
	got := rank(ps, rankCtx{days: 0, byCitations: true}, 30)
	found := false
	for _, s := range got {
		found = found || s.ID == "new"
	}
	if len(got) != 30 || !found {
		t.Fatalf("a range shows exactly 30 papers (got %d), including the only bioengineering preprint: %v", len(got), found)
	}
	// the same holds for searches and categories
	lists := [][]Paper{ps}
	q := parseQuery("trial sensor")
	q.Terms = []string{"trial"} // every paper matches; the preprint ranks last
	for i := range lists[0] {
		lists[0][i].Abstract += " trial"
	}
	res := matchRank(lists, q, 30, &rankCtx{days: 0, byCitations: true})
	found = false
	for _, s := range res {
		found = found || s.ID == "new"
	}
	if !found {
		t.Fatal("search results should keep the bioengineering preprint as an extra")
	}
}

func TestStructuredAbstracts(t *testing.T) {
	loadCatalog()
	html := "<h4>Background</h4>Sepsis kills 20% of patients admitted to hospital. <h4>Methods</h4>We enrolled 1,204 adults in 12 hospitals. <h4>Results</h4>Mortality was 18.2% with early antibiotics and 24.6% with usual care. <h4>Conclusions</h4>Early antibiotics saved lives."
	got := cleanAbstract(html)
	want := "Background: Sepsis kills 20% of patients admitted to hospital. Methods: We enrolled 1,204 adults in 12 hospitals. Results: Mortality was 18.2% with early antibiotics and 24.6% with usual care. Conclusions: Early antibiotics saved lives."
	if got != want {
		t.Fatalf("cleanAbstract:\n got %q\nwant %q", got, want)
	}
	for in, out := range map[string]string{
		"BACKGROUND Sepsis is common. METHODS We did it. RESULTS It worked.": "Background: Sepsis is common. Methods: We did it. Results: It worked.",
		"Background Sepsis is common. Results showed little.":                "Background: Sepsis is common. Results showed little.",
		"Our Results suggest a role. The Methods We used are new.":           "Our Results suggest a role. The Methods We used are new.",
		"Objective: To test X. Design, Setting, and Participants: A trial.":  "Objective: To test X. Design, Setting, and Participants: A trial.",
	} {
		if g := cleanAbstract(in); g != out {
			t.Errorf("cleanAbstract(%q)\n got %q\nwant %q", in, g, out)
		}
	}
	p := Paper{Abstract: got}
	if k := keySentence(p); k != "Mortality was 18.2% with early antibiotics and 24.6% with usual care." {
		t.Errorf("key sentence should be the first result, without its heading: %q", k)
	}
	parts := []string{}
	for _, s := range abstractSentences(got) {
		parts = append(parts, s.Part)
	}
	if strings.Join(parts, ",") != "background,methods,results,conclusions" {
		t.Errorf("parts: %v", parts)
	}
}

func TestLifecycleStopsWhenLastTabCloses(t *testing.T) {
	l := &lifecycle{quit: make(chan struct{})}
	t0 := time.Now()
	l.auto, l.deadline = true, t0.Add(firstContact)
	if l.idle(t0.Add(time.Minute)) {
		t.Fatal("should wait for the browser to open the first tab")
	}
	l.connected()
	if l.idle(t0.Add(time.Hour)) {
		t.Fatal("an open tab keeps it running, however long, even in the background")
	}
	l.disconnected(t0.Add(time.Hour)) // reload: the old page goes…
	l.connected()                     // …and the new one connects
	if l.idle(t0.Add(time.Hour + 10*time.Second)) {
		t.Fatal("a reload must not stop it")
	}
	l.connected() // a second tab
	l.disconnected(t0.Add(2 * time.Hour))
	if l.idle(t0.Add(2*time.Hour + time.Minute)) {
		t.Fatal("one tab still open")
	}
	l.disconnected(t0.Add(3 * time.Hour)) // the last tab closes
	if l.idle(t0.Add(3*time.Hour + afterLastTab - time.Second)) {
		t.Fatal("waits a moment for a reload")
	}
	if !l.idle(t0.Add(3*time.Hour + afterLastTab + time.Second)) {
		t.Fatal("should stop a few seconds after the last tab closed")
	}
	l3 := &lifecycle{quit: make(chan struct{}), auto: false, deadline: t0}
	if l3.idle(t0.Add(time.Hour)) {
		t.Fatal("with -lan or -no-browser it keeps running")
	}
}

func TestQuitOnlyFromThisComputer(t *testing.T) {
	loadCatalog()
	h := newMux()
	for _, c := range []struct {
		remote, ctype string
		want          int
	}{{"192.168.1.20:5000", "application/json", 403}, {"127.0.0.1:5000", "text/plain", 403}} {
		r := httptest.NewRequest("POST", "/api/quit", strings.NewReader("{}"))
		r.RemoteAddr, r.Header = c.remote, http.Header{"Content-Type": {c.ctype}}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.want {
			t.Errorf("quit from %s (%s): %d, want %d", c.remote, c.ctype, w.Code, c.want)
		}
	}
	r := httptest.NewRequest("GET", "/api/info", nil)
	r.RemoteAddr = "192.168.1.20:5000"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if !strings.Contains(w.Body.String(), `"canQuit":false`) {
		t.Errorf("phones are not offered Quit: %s", w.Body.String())
	}
}

// Everywhere, only original research and reviews with an abstract are shown: no letters, editorials or short items.
func TestOnlyResearchAndReviews(t *testing.T) {
	loadCatalog()
	long := strings.Repeat("We show a finding in detail. ", 12)
	review := Paper{ID: "r", Title: "Cardiac metabolism, a review", Abstract: long, Kind: "Review", Venue: "Circulation", Type: "journal", Date: today(), Tier: 2, FieldHint: "medicine", AuthorList: []string{"Kent JM", "Moreau L"}}
	noAbs := Paper{ID: "n", Title: "Heart failure in focus", Abstract: "", Kind: "Review", Venue: "Circulation", Type: "journal", Date: today(), Tier: 2, FieldHint: "medicine", AuthorList: []string{"Kent JM"}}
	letter := Paper{ID: "l", Title: "Troponin after marathons", Abstract: long, Kind: "Letter", Venue: "Circulation", Type: "journal", Date: today(), Tier: 2, FieldHint: "medicine", AuthorList: []string{"Okafor C", "Kent JM"}}
	editorial := Paper{ID: "e", Title: "A new era for cardiology", Abstract: long, Kind: "Editorial", Venue: "Circulation", Type: "journal", Date: today(), Tier: 2, FieldHint: "medicine", AuthorList: []string{"Kent JM"}}
	research := Paper{ID: "x", Title: "A trial of something", Abstract: long, Venue: "Circulation", Type: "journal", Date: today(), Tier: 2, FieldHint: "medicine", AuthorList: []string{"Kent JM"}}
	all := []Paper{review, noAbs, letter, editorial, research}
	ids := func(ss []Story) string {
		out := []string{}
		for _, s := range ss {
			out = append(out, s.ID)
		}
		sort.Strings(out)
		return strings.Join(out, ",")
	}
	if got := ids(rank(all, rankCtx{days: 365, byCitations: true}, 30)); got != "r,x" {
		t.Errorf("main tab: %s", got)
	}
	if got := ids(matchRank([][]Paper{all}, parseQuery("Jane Kent"), 30, &rankCtx{days: 365, byCitations: true})); got != "r,x" {
		t.Errorf("author search: %s", got)
	}
}

// The reported case: a review in Skeletal Muscle by Sarah Hachmer and F. Jeffrey Dilworth ("Dilworth FJ").
func TestMiddleNameAuthors(t *testing.T) {
	p := Paper{AuthorList: []string{"Hachmer S", "Dilworth FJ"}}
	for _, q := range []string{"Jeffrey Dilworth", "F. Jeffrey Dilworth", "Dilworth FJ", "Dilworth", "Sarah Hachmer"} {
		n, _ := parseName(q)
		if _, _, ok := authorMatch(p, n); !ok {
			t.Errorf("%q should match %v", q, p.AuthorList)
		}
	}
	n, _ := parseName("John Dilworth B")
	_ = n
	for _, c := range [][3]string{{"JA", "J", "y"}, {"FJ", "J", "y"}, {"FJ", "F", "y"}, {"JA", "JB", "n"}, {"FJ", "JF", "n"}, {"J", "K", "n"}} {
		if got := initialsCompatible(c[0], c[1]); got != (c[2] == "y") {
			t.Errorf("initialsCompatible(%s, %s) = %v", c[0], c[1], got)
		}
	}
}

// Reviews are their own paper type: in the main tabs (with an abstract) unless the reader leaves them out.
func TestReviewsAsAType(t *testing.T) {
	loadCatalog()
	long := strings.Repeat("We review how enhancers integrate signals in muscle stem cells. ", 8)
	rev := Paper{ID: "rev", Title: "Enhancers in muscle stem cells", Abstract: long, Kind: "Review", Venue: "Skeletal Muscle", Type: "journal", Date: today(), Tier: 2, FieldHint: "cellmol"}
	res := Paper{ID: "res", Title: "A new finding in muscle", Abstract: strings.Repeat("We show a new finding. ", 15), Venue: "Skeletal Muscle", Type: "journal", Date: today(), Tier: 2, FieldHint: "cellmol"}
	ids := func(ss []Story) string {
		out := []string{}
		for _, s := range ss {
			out = append(out, s.ID+"/"+paperClass(s.Type, s.Kind))
		}
		return strings.Join(out, ",")
	}
	if got := ids(rank([]Paper{rev, res}, rankCtx{days: 365, byCitations: true}, 30)); !strings.Contains(got, "rev/review") || !strings.Contains(got, "res/research") {
		t.Errorf("reviews included: %s", got)
	}
	if got := ids(rank([]Paper{rev, res}, rankCtx{days: 365, byCitations: true, noReviews: true}, 30)); got != "res/research" {
		t.Errorf("reviews left out: %s", got)
	}
	if paperClass("preprint", "Review") != "preprint" || paperClass("journal", "Letter") != "research" {
		t.Error("paper classes")
	}
}

// The Sources selection, papers per range and Include reviews are saved in a file, like categories.
func TestSettingsSaved(t *testing.T) {
	dir := t.TempDir()
	old := settingsPath
	settingsPath = dir + "/ResearchDigest/settings.json"
	defer func() { settingsPath = old }()
	h := newMux()
	put := func(body string, ctype string) int {
		r := httptest.NewRequest("PUT", "/api/settings", strings.NewReader(body))
		r.Header.Set("Content-Type", ctype)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if c := put(`{"settings":{"sources":["biorxiv","2044-5040","bad id!"],"sourcesKnown":["biorxiv"],"perRange":500,"includeReviews":false}}`, "application/json"); c != 200 {
		t.Fatalf("save: %d", c)
	}
	if c := put(`{}`, "text/plain"); c != 415 {
		t.Fatalf("other websites cannot save settings: %d", c)
	}
	s := loadSettings()
	if strings.Join(s.Sources, ",") != "biorxiv,2044-5040" || s.PerRange != 200 || s.IncludeReviews == nil || *s.IncludeReviews {
		t.Fatalf("saved settings: %+v", s)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/settings", nil))
	if !strings.Contains(w.Body.String(), `"perRange":200`) || !strings.Contains(w.Body.String(), "settings.json") {
		t.Fatalf("read back: %s", w.Body.String())
	}
}
