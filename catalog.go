package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"regexp"
	"strings"
)

//go:embed web
var webFS embed.FS

// Catalog is shared with the mobile engine (web/catalog.json), so both read the same sources and rules.
type Catalog struct {
	Windows []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
		Days  int    `json:"days"`
	} `json:"windows"`
	Fields []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
		Hint  string `json:"hint"`
	} `json:"fields"`
	Journals        [][]any           `json:"journals"`
	ArxivCategories []string          `json:"arxivCategories"`
	ArxivField      map[string]string `json:"arxivField"`
	RxivField       map[string]string `json:"rxivField"`
	FieldRules      [][2]string       `json:"fieldRules"`
	Cues            []string          `json:"cues"`
	JunkTitle       string            `json:"junkTitle"`
	KeySentence     string            `json:"keySentence"`
	// Section names used in structured abstracts, grouped by part: background, methods, results, conclusions, other.
	AbstractSections map[string][]string `json:"abstractSections"`
}

type journal struct {
	Name, ISSN, Field string
	Tier              int
}

type fieldRule struct {
	Field string
	Re    *regexp.Regexp
}

var (
	cat         Catalog
	journals    []journal
	issnInfo    = map[string]journal{}
	fieldRules  []fieldRule
	cueRes      []*regexp.Regexp
	junkRe      *regexp.Regexp
	keySentRe   *regexp.Regexp
	windowDays  = map[string]int{}
	windowLabel = map[string]string{}
	fieldLabel  = map[string]string{}
)

func ci(p string) *regexp.Regexp { return regexp.MustCompile("(?i)" + p) }

func loadCatalog() {
	b, err := fs.ReadFile(webFS, "web/catalog.json")
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(b, &cat); err != nil {
		panic(err)
	}
	for _, w := range cat.Windows {
		windowDays[w.ID] = w.Days
		windowLabel[w.ID] = w.Label
	}
	for _, f := range cat.Fields {
		fieldLabel[f.ID] = f.Label
	}
	for _, j := range cat.Journals {
		jj := journal{Name: j[0].(string), ISSN: strings.ToUpper(j[1].(string)), Tier: int(j[2].(float64)), Field: j[3].(string)}
		journals = append(journals, jj)
		issnInfo[jj.ISSN] = jj
	}
	for _, r := range cat.FieldRules {
		fieldRules = append(fieldRules, fieldRule{r[0], ci(r[1])})
	}
	for _, c := range cat.Cues {
		cueRes = append(cueRes, ci(c))
	}
	junkRe = ci(cat.JunkTitle)
	keySentRe = ci(cat.KeySentence)
	buildSectionRes()
}
