package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type sourceStatus struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Count int    `json:"count"`
	Ms    int64  `json:"ms"`
	Error string `json:"error,omitempty"`
}

type Feed struct {
	Window      string         `json:"window"`
	Label       string         `json:"label"`
	From        string         `json:"from"`
	GeneratedAt string         `json:"generatedAt"`
	TookMs      int64          `json:"tookMs"`
	RankedBy    string         `json:"rankedBy"`
	Scanned     int            `json:"scanned"`
	Sources     []sourceStatus `json:"sources"`
	Stories     []Story        `json:"stories"`
	Query       string         `json:"query,omitempty"`
	Mode        string         `json:"mode,omitempty"`    // keywords, author or both
	Authors     []string       `json:"authors,omitempty"` // author names the search looked for
}

var (
	sourceTimeout  = 15 * time.Second
	storiesPerFeed = 20
	shareFor       = 2 * time.Minute // only in memory: people opening at nearly the same moment share one fetch
)

// runTasks runs every source at once; a slow or failing source is reported, never fatal.
func runTasks(ctx context.Context, tasks []task) ([]sourceStatus, [][]Paper) {
	statuses := make([]sourceStatus, len(tasks))
	results := make([][]Paper, len(tasks))
	var wg sync.WaitGroup
	for i, t := range tasks {
		wg.Add(1)
		go func(i int, t task) {
			defer wg.Done()
			c, cancel := context.WithTimeout(ctx, sourceTimeout)
			defer cancel()
			s := time.Now()
			ps, err := t.run(c)
			st := sourceStatus{Name: t.name, OK: err == nil, Count: len(ps), Ms: time.Since(s).Milliseconds()}
			if err != nil {
				msg := err.Error()
				if len(msg) > 120 {
					msg = msg[:120]
				}
				st.Error = msg
			}
			statuses[i], results[i] = st, ps
		}(i, t)
	}
	wg.Wait()
	return statuses, results
}

// Longer windows rank by how much other researchers already cite a paper; short ones by how new it is.
func byCitations(window string) bool { return window != "day" && window != "week" }

// pool is what one time range's own fetch returned, before ranking.
type pool struct {
	statuses []sourceStatus
	lists    [][]Paper
}

// shorterWindows lists the ranges inside window (Today and This week are inside This month, and so on).
func shorterWindows(window string) []string {
	out := []string{}
	for _, w := range cat.Windows {
		if w.ID == window {
			return out
		}
		out = append(out, w.ID)
	}
	return out
}

// nested gathers a range's own papers plus everything its shorter ranges found, so a paper shown under
// This month is always a candidate for This year, Past 5 years and All time as well.
func nested(window string, own func(w string) pool) (statuses []sourceStatus, lists [][]Paper) {
	ws := append(shorterWindows(window), window)
	got := make([]pool, len(ws))
	var wg sync.WaitGroup
	for i, w := range ws {
		wg.Add(1)
		go func(i int, w string) { defer wg.Done(); got[i] = own(w) }(i, w)
	}
	wg.Wait()
	for i, p := range got {
		lists = append(lists, p.lists...)
		if i == len(got)-1 {
			statuses = p.statuses
		}
	}
	return statuses, lists
}

func uniqueCount(lists [][]Paper) int {
	seen := map[string]bool{}
	for _, l := range lists {
		for _, p := range l {
			seen[p.ID] = true
		}
	}
	return len(seen)
}

// perRange is how many papers each time range shows (the reader can change it; 30 by default).
func perRange(r *http.Request) int {
	n, err := strconv.Atoi(r.URL.Query().Get("n"))
	if err != nil || n == 0 {
		return storiesPerFeed
	}
	return min(max(n, 5), 200)
}

func feedPool(w string, sel Selection, n int, reviews bool) pool {
	size := candidates(w, n)
	return pools.get("feed:"+w+":"+sel.Key()+":"+strconv.Itoa(size)+":"+strconv.FormatBool(reviews), func() pool {
		st, ls := runTasks(context.Background(), planSized(w, sel, size, reviews))
		return pool{st, ls}
	})
}

// includeReviews: reviews are shown unless the page asks for them to be left out (reviews=0).
func includeReviews(r *http.Request) bool { return r.URL.Query().Get("reviews") != "0" }

func buildFeed(ctx context.Context, window string, sel Selection) Feed {
	return buildFeedN(ctx, window, sel, storiesPerFeed, true)
}

func buildFeedN(ctx context.Context, window string, sel Selection, n int, reviews bool) Feed {
	t0 := time.Now()
	statuses, lists := nested(window, func(w string) pool { return feedPool(w, sel, n, reviews) })
	all := []Paper{}
	for _, l := range lists {
		all = append(all, l...)
	}
	c := rankCtx{days: windowDays[window], byCitations: byCitations(window), noReviews: !reviews}
	return Feed{Window: window, Label: windowLabel[window], From: fromDate(windowDays[window]), GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		TookMs: time.Since(t0).Milliseconds(), RankedBy: "heuristic", Scanned: uniqueCount(lists), Sources: statuses, Stories: rank(all, c, n)}
}

// Results are kept in memory only, for shareFor, so tabs and people opening at once share one fetch.
type cacheEntry[T any] struct {
	done chan struct{}
	v    T
	at   time.Time
}
type memo[T any] struct {
	sync.Mutex
	m map[string]*cacheEntry[T]
}

func (c *memo[T]) get(key string, build func() T) T {
	c.Lock()
	if c.m == nil {
		c.m = map[string]*cacheEntry[T]{}
	}
	if e := c.m[key]; e != nil {
		select {
		case <-e.done:
			if time.Since(e.at) < shareFor {
				c.Unlock()
				return e.v
			}
		default:
			c.Unlock()
			<-e.done
			return e.v
		}
	}
	e := &cacheEntry[T]{done: make(chan struct{})}
	c.m[key] = e
	if len(c.m) > 300 { // searches add keys; drop finished, expired ones
		for k, old := range c.m {
			select {
			case <-old.done:
				if time.Since(old.at) >= shareFor {
					delete(c.m, k)
				}
			default:
			}
		}
	}
	c.Unlock()
	e.v = build()
	e.at = time.Now()
	close(e.done)
	return e.v
}

var (
	feeds memo[Feed]
	pools memo[pool]
)

func resetCaches() { feeds = memo[Feed]{}; pools = memo[pool]{} }

// getFeed shares one in-flight build between simultaneous opens; nothing is kept on disk.
func getFeed(window string, sel Selection) Feed { return getFeedN(window, sel, storiesPerFeed, true) }

func getFeedN(window string, sel Selection, n int, reviews bool) Feed {
	return getCached("feed:"+window+":"+sel.Key()+":"+strconv.Itoa(n)+":"+strconv.FormatBool(reviews), func() Feed {
		return buildFeedN(context.Background(), window, sel, n, reviews)
	})
}

func getTopicN(q, window string, sel Selection, n int, reviews bool) Feed {
	return getCached("topic:"+window+":"+sel.Key()+":"+strconv.Itoa(n)+":"+strconv.FormatBool(reviews)+":"+strings.ToLower(q), func() Feed {
		return buildTopicN(context.Background(), q, window, sel, n, reviews)
	})
}

func getCached(key string, build func() Feed) Feed { return feeds.get(key, build) }

type gzWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (w gzWriter) Write(b []byte) (int, error) { return w.gz.Write(b) }

// Flush lets streamed responses reach the page as each part is ready.
func (w gzWriter) Flush() {
	_ = w.gz.Flush()
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func withGzip(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			h.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")
		gz, _ := gzip.NewWriterLevel(w, gzip.BestSpeed)
		defer gz.Close()
		h.ServeHTTP(gzWriter{w, gz}, r)
	})
}

func newMux() http.Handler {
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc("/api/feed", func(w http.ResponseWriter, r *http.Request) {
		win := r.URL.Query().Get("window")
		if win == "" {
			win = "day"
		}
		if _, ok := windowDays[win]; !ok {
			writeJSON(w, 400, map[string]string{"error": "Unknown window. Use day, week, month or year."})
			return
		}
		writeJSON(w, 200, getFeedN(win, parseSelection(r.URL.Query().Get("sources")), perRange(r), includeReviews(r)))
	})
	mux.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		rng := r.URL.Query().Get("range")
		if _, ok := searchRanges[rng]; !ok {
			rng = "year"
		}
		if parseQuery(q).Empty() || len(q) > 200 {
			writeJSON(w, 400, map[string]string{"error": "Enter keywords or an author (up to 200 characters)."})
			return
		}
		sel := parseSelection(r.URL.Query().Get("sources"))
		writeJSON(w, 200, getCached("search:"+rng+":"+sel.Key()+":"+strings.ToLower(q), func() Feed { return buildSearch(context.Background(), q, rng, sel) }))
	})
	mux.HandleFunc("/api/topic", func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		win := r.URL.Query().Get("window")
		if _, ok := windowDays[win]; !ok {
			writeJSON(w, 400, map[string]string{"error": "Unknown window. Use day, week, month or year."})
			return
		}
		if parseQuery(q).Empty() || len(q) > 200 {
			writeJSON(w, 400, map[string]string{"error": "A category needs one or more keywords (up to 200 characters)."})
			return
		}
		sel := parseSelection(r.URL.Query().Get("sources"))
		writeJSON(w, 200, getTopicN(q, win, sel, perRange(r), includeReviews(r)))
	})
	mux.HandleFunc("/api/categories", categoriesHandler(writeJSON))
	mux.HandleFunc("/api/settings", settingsHandler(writeJSON))
	// /api/stream sends several time ranges over one connection, one JSON line per range as each is ready,
	// so the page never has more than a couple of requests open (browsers allow only ~6 per address).
	mux.HandleFunc("/api/stream", func(w http.ResponseWriter, r *http.Request) {
		wins := []string{}
		for _, win := range strings.Split(r.URL.Query().Get("windows"), ",") {
			if _, ok := windowDays[win]; ok && len(wins) < len(windowDays) {
				wins = append(wins, win)
			}
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if len(wins) == 0 || len(q) > 200 || (q != "" && parseQuery(q).Empty()) {
			writeJSON(w, 400, map[string]string{"error": "Ask for one or more of: day, week, month, year, 5y, all (and optional keywords)."})
			return
		}
		type line struct {
			Window string `json:"window"`
			Feed   Feed   `json:"feed"`
		}
		sel, n, reviews := parseSelection(r.URL.Query().Get("sources")), perRange(r), includeReviews(r)
		ch := make(chan line, len(wins))
		for _, win := range wins {
			go func(win string) {
				if q == "" {
					ch <- line{win, getFeedN(win, sel, n, reviews)}
					return
				}
				ch <- line{win, getTopicN(q, win, sel, n, reviews)}
			}(win)
		}
		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		enc := json.NewEncoder(w)
		flusher, _ := w.(http.Flusher)
		for range wins {
			select {
			case l := <-ch:
				_ = enc.Encode(l)
				if flusher != nil {
					flusher.Flush()
				}
			case <-r.Context().Done():
				return
			}
		}
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok research-digest")) })
	lifecycleRoutes(mux, writeJSON)
	sub, _ := fs.Sub(webFS, "web")
	static := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		static.ServeHTTP(w, r)
	})
	return withGzip(mux)
}
