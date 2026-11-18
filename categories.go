package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Your saved categories live in a small file on this computer, so they survive restarts, a change of
// port, or a different browser:
//   Windows: %AppData%\ResearchDigest\categories.json   Linux: ~/.config/ResearchDigest/categories.json
// Categories saved under the app's earlier name (DailyDigest) are picked up automatically.
// Only the keywords are saved; papers are always fetched fresh.

type Category struct {
	ID string `json:"id"`
	Q  string `json:"q"`
}

var (
	categoriesMu   sync.Mutex
	categoriesPath = defaultCategoriesPath("ResearchDigest")
	legacyPath     = defaultCategoriesPath("DailyDigest")
	catIDRe        = regexp.MustCompile(`^[a-z0-9]{1,40}$`)
)

func defaultCategoriesPath(app string) string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = "."
	}
	return filepath.Join(dir, app, "categories.json")
}

func loadCategories() []Category {
	categoriesMu.Lock()
	defer categoriesMu.Unlock()
	out := []Category{}
	b, err := os.ReadFile(categoriesPath)
	if err != nil && legacyPath != "" {
		b, err = os.ReadFile(legacyPath) // saved before the rename; written to the new place on the next change
	}
	if err != nil {
		return out
	}
	_ = json.Unmarshal(b, &out)
	return sanitizeCategories(out)
}

func sanitizeCategories(in []Category) []Category {
	out, seen := []Category{}, map[string]bool{}
	for _, c := range in {
		c.Q = strings.TrimSpace(c.Q)
		key := strings.ToLower(c.Q)
		if !catIDRe.MatchString(c.ID) || c.Q == "" || len(c.Q) > 200 || seen[key] || parseQuery(c.Q).Empty() {
			continue
		}
		seen[key] = true
		out = append(out, c)
		if len(out) == 100 {
			break
		}
	}
	return out
}

func saveCategories(cs []Category) error {
	categoriesMu.Lock()
	defer categoriesMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(categoriesPath), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(cs, "", "  ")
	tmp := categoriesPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, categoriesPath) // replace in one step so a crash never leaves half a file
}

func categoriesHandler(writeJSON func(http.ResponseWriter, int, any)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, map[string]any{"categories": loadCategories(), "savedTo": categoriesPath})
		case http.MethodPut:
			// JSON only: browsers cannot send this cross-site without a preflight, which is never answered.
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				writeJSON(w, 415, map[string]string{"error": "Send JSON."})
				return
			}
			var body struct {
				Categories []Category `json:"categories"`
			}
			if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&body); err != nil {
				writeJSON(w, 400, map[string]string{"error": "Could not read the categories."})
				return
			}
			cs := sanitizeCategories(body.Categories)
			if err := saveCategories(cs); err != nil {
				writeJSON(w, 500, map[string]string{"error": "Could not save categories on this computer: " + err.Error()})
				return
			}
			writeJSON(w, 200, map[string]any{"categories": cs, "savedTo": categoriesPath})
		default:
			w.Header().Set("Allow", "GET, PUT")
			writeJSON(w, 405, map[string]string{"error": "Use GET or PUT."})
		}
	}
}
