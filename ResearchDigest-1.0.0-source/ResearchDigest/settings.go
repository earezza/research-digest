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

// Your display settings live in a small file next to your saved categories, so they survive restarts, a change
// of port and a different browser (browser storage alone is lost whenever the program has to use another port):
//   Windows: %AppData%\ResearchDigest\settings.json   Mac: ~/Library/Application Support/ResearchDigest/settings.json
//   Linux: ~/.config/ResearchDigest/settings.json

type Settings struct {
	Sources        []string `json:"sources"`        // ticked sources; empty means all
	SourcesKnown   []string `json:"sourcesKnown"`   // the sources that existed when they were ticked (newer ones get ticked)
	PerRange       int      `json:"perRange"`       // papers per time range, 5–200
	IncludeReviews *bool    `json:"includeReviews"` // nil until chosen (reviews are shown by default)
}

var (
	settingsMu   sync.Mutex
	settingsPath = filepath.Join(filepath.Dir(categoriesPath), "settings.json")
	sourceIDRe   = regexp.MustCompile(`^[a-z]{2,10}$|^[0-9]{4}-[0-9]{3}[0-9X]$`)
)

func cleanIDs(in []string) []string {
	out, seen := []string{}, map[string]bool{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if sourceIDRe.MatchString(s) && !seen[s] && len(out) < 200 {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func sanitizeSettings(s Settings) Settings {
	s.Sources, s.SourcesKnown = cleanIDs(s.Sources), cleanIDs(s.SourcesKnown)
	if s.PerRange != 0 {
		s.PerRange = min(max(s.PerRange, 5), 200)
	}
	return s
}

func loadSettings() Settings {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	var s Settings
	if b, err := os.ReadFile(settingsPath); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return sanitizeSettings(s)
}

func saveSettings(s Settings) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	tmp := settingsPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, settingsPath) // replace in one step so a crash never leaves half a file
}

func settingsHandler(writeJSON func(http.ResponseWriter, int, any)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, map[string]any{"settings": loadSettings(), "savedTo": settingsPath})
		case http.MethodPut:
			// JSON only: browsers cannot send this cross-site without a preflight, which is never answered.
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				writeJSON(w, 415, map[string]string{"error": "Send JSON."})
				return
			}
			var body struct {
				Settings Settings `json:"settings"`
			}
			if err := json.NewDecoder(io.LimitReader(r.Body, 32<<10)).Decode(&body); err != nil {
				writeJSON(w, 400, map[string]string{"error": "Could not read the settings."})
				return
			}
			s := sanitizeSettings(body.Settings)
			if err := saveSettings(s); err != nil {
				writeJSON(w, 500, map[string]string{"error": "Could not save settings on this computer: " + err.Error()})
				return
			}
			writeJSON(w, 200, map[string]any{"settings": s, "savedTo": settingsPath})
		default:
			w.Header().Set("Allow", "GET, PUT")
			writeJSON(w, 405, map[string]string{"error": "Use GET or PUT."})
		}
	}
}
