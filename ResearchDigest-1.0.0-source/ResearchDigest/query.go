package main

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// A search can name authors as well as keywords:
//   CRISPR delivery                  keywords in title or abstract
//   author:"Jennifer Doudna"         papers by that author (also au: and by:)
//   CRISPR author:Doudna             both
//   Jennifer Doudna / Doudna JA      looks like a person's name: searched as an author and as keywords

type Name struct {
	Surname  string `json:"surname"`
	Initials string `json:"initials"`
	Display  string `json:"display"`
}

type Query struct {
	Raw     string
	Terms   []string
	Authors []Name
	Auto    bool // the whole query looked like a name, so it is tried both ways
}

func (q Query) authorNames() []string {
	out := []string{}
	for _, n := range q.Authors {
		out = append(out, n.Display)
	}
	return out
}

func (q Query) Empty() bool { return len(q.Terms) == 0 && len(q.Authors) == 0 }

// Mode tells the page how the query was read.
func (q Query) Mode() string {
	switch {
	case len(q.Authors) == 0:
		return "keywords"
	case q.Auto:
		return "both"
	}
	return "author"
}

var authorFieldRe = regexp.MustCompile(`(?i)\b(?:author|au|by):\s*(?:"([^"]+)"|(\S+))`)

func parseQuery(raw string) Query {
	q := Query{Raw: strings.TrimSpace(raw)}
	rest := authorFieldRe.ReplaceAllStringFunc(q.Raw, func(m string) string {
		sub := authorFieldRe.FindStringSubmatch(m)
		if n, ok := parseName(firstNonEmpty(sub[1], sub[2])); ok && len(q.Authors) < 4 {
			q.Authors = append(q.Authors, n)
		}
		return " "
	})
	q.Terms = searchTerms(rest)
	if len(q.Authors) == 0 && looksLikeName(q.Raw) {
		if n, ok := parseName(q.Raw); ok {
			q.Authors, q.Auto = []Name{n}, true
		}
	}
	return q
}

var particles = map[string]bool{"van": true, "von": true, "de": true, "der": true, "den": true, "da": true, "di": true, "del": true, "della": true, "du": true, "la": true, "le": true, "dos": true, "das": true, "ter": true, "ten": true, "bin": true, "al": true, "el": true}

var initialsRe = regexp.MustCompile(`^(\p{Lu}\.?-?){1,4}$`)
var properRe = regexp.MustCompile(`^\p{Lu}[\p{Ll}'’]+(-\p{Lu}?[\p{Ll}'’]+)*$`)

// looksLikeName: two to four capitalised words or initials, such as "Jennifer A. Doudna" or "Doudna JA".
func looksLikeName(s string) bool {
	if strings.ContainsAny(s, "\"0123456789:()") {
		return false
	}
	toks := strings.Fields(strings.ReplaceAll(s, ",", " "))
	if len(toks) < 2 || len(toks) > 4 {
		return false
	}
	words := 0
	for _, t := range toks {
		switch {
		case properRe.MatchString(t):
			words++
		case initialsRe.MatchString(t), particles[t]:
		default:
			return false
		}
	}
	return words >= 1 && words <= 3
}

func initialsOf(tokens []string) string {
	var b strings.Builder
	for _, t := range tokens {
		for _, part := range strings.FieldsFunc(t, func(r rune) bool { return r == '.' || r == '-' }) {
			if initialsRe.MatchString(part) && strings.ToUpper(part) == part {
				b.WriteString(part) // "JA" or "J"
			} else if r := []rune(part); len(r) > 0 {
				b.WriteRune(unicode.ToUpper(r[0]))
			}
		}
	}
	return b.String()
}

// parseName reads "Jennifer A. Doudna", "J. A. Doudna", "Doudna JA", "Doudna, Jennifer" or "Doudna".
func parseName(s string) (Name, bool) {
	s = strings.TrimSpace(strings.Trim(s, `"'`))
	if s == "" || len(s) > 80 {
		return Name{}, false
	}
	n := Name{Display: s}
	if i := strings.Index(s, ","); i > 0 { // "Doudna, Jennifer A."
		n.Surname = strings.TrimSpace(s[:i])
		n.Initials = initialsOf(strings.Fields(s[i+1:]))
		return n, n.Surname != ""
	}
	toks := strings.Fields(s)
	last := strings.ReplaceAll(toks[len(toks)-1], ".", "")
	if len(toks) >= 2 && initialsRe.MatchString(toks[len(toks)-1]) && strings.ToUpper(last) == last { // "Doudna JA"
		n.Surname, n.Initials = strings.Join(toks[:len(toks)-1], " "), last
		return n, true
	}
	cut := len(toks) - 1 // surname is the last word plus any particles before it ("van der Waals")
	for cut > 0 && particles[strings.ToLower(toks[cut-1])] {
		cut--
	}
	n.Surname, n.Initials = strings.Join(toks[cut:], " "), initialsOf(toks[:cut])
	return n, true
}

var fold = strings.NewReplacer("á", "a", "à", "a", "ä", "a", "â", "a", "ã", "a", "å", "a", "é", "e", "è", "e", "ë", "e", "ê", "e",
	"í", "i", "ì", "i", "ï", "i", "î", "i", "ó", "o", "ò", "o", "ö", "o", "ô", "o", "õ", "o", "ø", "o", "ú", "u", "ù", "u", "ü", "u", "û", "u",
	"ñ", "n", "ç", "c", "ß", "ss", "’", "'", "-", " ")

func normName(s string) string {
	return strings.Join(strings.Fields(fold.Replace(strings.ToLower(s))), " ")
}

// initialsCompatible: one set of initials is the other's, in order, with letters missing. "J" matches "JA" and
// also "FJ" (F. Jeffrey Dilworth, who goes by his middle name); "JA" matches "J"; "JB" does not match "JA".
func initialsCompatible(a, b string) bool {
	a, b = strings.ToUpper(a), strings.ToUpper(b)
	if a == "" || b == "" {
		return true
	}
	if len(a) < len(b) {
		a, b = b, a
	}
	i := 0
	for _, r := range a {
		if i < len(b) && rune(b[i]) == r {
			i++
		}
	}
	return i == len(b)
}

// authorMatch finds the queried person in a paper's author list and says where they appear.
func authorMatch(p Paper, n Name) (shown, role string, ok bool) {
	for i, a := range p.AuthorList {
		an, good := parseName(a)
		if !good || normName(an.Surname) != normName(n.Surname) || !initialsCompatible(an.Initials, n.Initials) {
			continue
		}
		total := len(p.AuthorList)
		switch {
		case total == 1:
			role = "sole author"
		case i == 0:
			role = fmt.Sprintf("first of %d authors", total)
		case i == total-1:
			role = fmt.Sprintf("last of %d authors", total)
		default:
			role = fmt.Sprintf("author %d of %d", i+1, total)
		}
		return a, role, true
	}
	return "", "", false
}

// surnameClause searches by surname only (any initials).
func surnameClause(ns []Name) string {
	parts := []string{}
	for _, n := range ns {
		parts = append(parts, `AUTH:"`+strings.NewReplacer(`"`, "", `\`, "").Replace(n.Surname)+`"`)
	}
	return "(" + strings.Join(parts, " AND ") + ")"
}

func authorClause(ns []Name) string {
	parts := []string{}
	for _, n := range ns {
		s := strings.NewReplacer(`"`, "", `\`, "").Replace(n.Surname)
		alts := []string{`AUTH:"` + s + `"`}
		if n.Initials != "" {
			alts = []string{`AUTH:"` + s + " " + n.Initials + `"`}
			if len(n.Initials) > 1 {
				alts = append(alts, `AUTH:"`+s+" "+n.Initials[:1]+`"`)
			}
		}
		parts = append(parts, "("+strings.Join(alts, " OR ")+")")
	}
	return "(" + strings.Join(parts, " AND ") + ")"
}

func arxivAuthorSearch(ctx context.Context, ns []Name, terms []string, from string, max int) ([]Paper, error) {
	parts := []string{}
	for _, n := range ns {
		for _, w := range strings.Fields(normName(n.Surname)) {
			if !particles[w] {
				parts = append(parts, "au:"+w)
			}
		}
	}
	for _, t := range terms {
		parts = append(parts, arxivQueryTerm(t))
	}
	query := strings.Join(parts, " AND ")
	if from != "" {
		query += " AND submittedDate:[" + strings.ReplaceAll(from, "-", "") + "0000 TO " + strings.ReplaceAll(today(), "-", "") + "2359]"
	}
	v := url.Values{"search_query": {query}, "sortBy": {"submittedDate"}, "sortOrder": {"descending"}, "max_results": {strconv.Itoa(max)}}
	b, err := get(ctx, arxivBase+"/api/query?"+v.Encode())
	if err != nil {
		return nil, err
	}
	return parseArxiv(b)
}

// queryTasks fetches a query from every ticked source: keyword part, author part, or both.
func queryTasks(q Query, from string, sel Selection) []task {
	ts := []task{}
	if len(q.Terms) > 0 && (len(q.Authors) == 0 || q.Auto) {
		ts = append(ts, searchTasks(q.Raw, q.Terms, from, sel)...)
	}
	if len(q.Authors) == 0 {
		return ts
	}
	ac, dc, tc := authorClause(q.Authors), dateClause(from), ""
	extraTerms := []string{}
	if !q.Auto && len(q.Terms) > 0 {
		tc, extraTerms = termsClause(q.Terms), q.Terms
	}
	who := q.Authors[0].Display
	if js := sel.Journals(0); len(js) > 0 {
		ts = append(ts, task{"Digest journals · by " + who, func(c context.Context) ([]Paper, error) {
			return epmcPapers(c, epmcQuery(ac, tc, journalClause(js), dc, withAbstract), sortNewest, 100, nil)
		}})
		// Indexes differ in how they write initials ("Doudna J", "Doudna JA"), so the digest's journals are also
		// searched by surname alone; every result is then checked against the paper's own author list.
		if sc := surnameClause(q.Authors); sc != ac {
			ts = append(ts, task{"Digest journals · surname " + who, func(c context.Context) ([]Paper, error) {
				return epmcPapers(c, epmcQuery(sc, tc, journalClause(js), dc, withAbstract), sortNewest, 100, nil)
			}})
		}
	}
	if sel.Has("other") {
		ts = append(ts, task{"Other journals · by " + who, func(c context.Context) ([]Paper, error) {
			return epmcPapers(c, epmcQuery(ac, tc, "SRC:MED", dc, withAbstract), sortNewest, 100, nil)
		}})
	}
	if sel.AnyServer() {
		ts = append(ts, task{"Preprint servers · by " + who, func(c context.Context) ([]Paper, error) {
			extra := ac + " AND " + withAbstract
			if tc != "" {
				extra += " AND " + tc
			}
			return epmcPreprints(c, sel, from, sortNewest, extra, 60)
		}})
	}
	if sel.Has("arxiv") {
		ts = append(ts, task{"arXiv · by " + who, func(c context.Context) ([]Paper, error) { return arxivAuthorSearch(c, q.Authors, extraTerms, from, 40) }})
	}
	return ts
}
