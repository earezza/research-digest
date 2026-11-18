package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Research Digest runs without a terminal window and stops when its page closes. Every open tab keeps one
// connection to the program (/api/presence). Closing the tab or the browser window drops that connection at
// once, even for tabs in the background, and the program stops a few seconds after the last one goes; the
// short wait lets a reload reconnect without stopping it. The Quit button on the page stops it straight away
// (offered only on this computer, never to phones on the network).

const (
	afterLastTab = 5 * time.Second // a reload reconnects well within this
	firstContact = 3 * time.Minute // time for the browser to open the first tab
)

type lifecycle struct {
	mu       sync.Mutex
	open     int       // tabs connected now
	deadline time.Time // stop when no tab is connected after this
	auto     bool      // stop by itself (off with -lan or -no-browser: then it is serving others)
	quit     chan struct{}
	once     sync.Once
}

var life = &lifecycle{quit: make(chan struct{})}

func (l *lifecycle) start(auto bool) {
	l.mu.Lock()
	l.auto, l.deadline = auto, time.Now().Add(firstContact)
	l.mu.Unlock()
	if !auto {
		return
	}
	go func() {
		for range time.Tick(time.Second) {
			if l.idle(time.Now()) {
				l.stop()
				return
			}
		}
	}()
}

// idle reports whether no tab has been connected since the deadline.
func (l *lifecycle) idle(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.auto && l.open == 0 && now.After(l.deadline)
}

func (l *lifecycle) connected() {
	l.mu.Lock()
	l.open++
	l.mu.Unlock()
}

func (l *lifecycle) disconnected(now time.Time) {
	l.mu.Lock()
	l.open--
	l.deadline = now.Add(afterLastTab)
	l.mu.Unlock()
}

func (l *lifecycle) stop() { l.once.Do(func() { close(l.quit) }) }

// fromThisComputer is true for requests made on this computer, not by phones on the network.
func fromThisComputer(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// lanAddrs are the addresses phones can open, shown on this computer's page when started with -lan.
var lanAddrs []string

func lifecycleRoutes(mux *http.ServeMux, writeJSON func(http.ResponseWriter, int, any)) {
	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		local := fromThisComputer(r)
		info := map[string]any{"version": version, "canQuit": local}
		if local && len(lanAddrs) > 0 {
			info["lan"] = lanAddrs
		}
		writeJSON(w, 200, info)
	})
	// One long-lived connection per open tab (server-sent events). It carries nothing but a small "still here"
	// every 20 seconds; its closing is what tells the program the tab has gone.
	mux.HandleFunc("/api/presence", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		life.connected()
		debugf("tab connected")
		defer func() { life.disconnected(time.Now()); debugf("tab closed") }()
		fmt.Fprint(w, "retry: 1000\n\n") // reconnect quickly after a reload or a hiccup
		flusher.Flush()
		tick := time.NewTicker(20 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-life.quit:
				fmt.Fprint(w, "event: stopped\ndata: {}\n\n")
				flusher.Flush()
				return
			case <-tick.C:
				fmt.Fprint(w, ": still here\n\n")
				flusher.Flush()
			}
		}
	})
	// Quit only accepts JSON, which other websites cannot send here without a preflight that is never answered.
	mux.HandleFunc("/api/quit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") || !fromThisComputer(r) {
			writeJSON(w, 403, map[string]string{"error": "Quit from the computer running Research Digest."})
			return
		}
		writeJSON(w, 200, map[string]bool{"stopping": true})
		go func() { time.Sleep(200 * time.Millisecond); life.stop() }()
	})
}

// alreadyRunning reports whether Research Digest is already serving on this port, so a second launch just
// opens another tab instead of starting a second copy.
func alreadyRunning(port int) bool {
	c := http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/healthz")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	b := make([]byte, 100)
	n, _ := resp.Body.Read(b)
	return strings.Contains(string(b[:n]), "research-digest")
}

// debugf logs tab activity when DD_DEBUG is set (for testing).
func debugf(format string, a ...any) {
	if os.Getenv("DD_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, time.Now().Format("15:04:05.000 ")+format+"\n", a...)
	}
}
