// Research Digest: a live research newsletter. Run it and it opens in your browser; no terminal window,
// keys, accounts or sign-ups. It reads Europe PMC, arXiv, bioRxiv and medRxiv, which are all free and open.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

var version = "1.0.0"

func openBrowser(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case "darwin":
		cmd = exec.Command("open", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	hideWindow(cmd)
	_ = cmd.Start()
}

func lanAddresses(port int) []string {
	out := []string{}
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && !ipn.IP.IsLoopback() && ipn.IP.To4() != nil {
			out = append(out, fmt.Sprintf("http://%s:%d", ipn.IP, port))
		}
	}
	return out
}

func main() {
	port := flag.Int("port", 8787, "port to serve on (a free one is chosen if it is taken)")
	lan := flag.Bool("lan", false, "also serve to other devices on your network, e.g. your phone; keeps running until Quit")
	noBrowser := flag.Bool("no-browser", false, "do not open a browser window; keeps running until Quit or Ctrl+C")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("Research Digest", version)
		return
	}
	loadCatalog()

	host := "127.0.0.1"
	if *lan {
		host = "0.0.0.0"
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, *port))
	if err != nil {
		// Already open? Then a second launch just opens another tab.
		if alreadyRunning(*port) {
			if !*noBrowser {
				openBrowser(fmt.Sprintf("http://localhost:%d", *port))
			}
			return
		}
		ln, err = net.Listen("tcp", host+":0")
		if err != nil {
			showError("Research Digest could not start: " + err.Error())
			os.Exit(1)
		}
	}
	actual := ln.Addr().(*net.TCPAddr).Port
	local := fmt.Sprintf("http://localhost:%d", actual)
	if *lan {
		lanAddrs = lanAddresses(actual)
	}

	// Shown only when started from a terminal; the program itself runs without one.
	fmt.Println("Research Digest", version)
	fmt.Println("Open:", local)
	fmt.Println("Your saved categories:", categoriesPath)
	if len(lanAddrs) > 0 {
		fmt.Println("On your phone (same Wi-Fi):", strings.Join(lanAddrs, "  "))
	}
	fmt.Println("Use Quit on the page, or close its tabs, to stop.")

	life.start(!*lan && !*noBrowser)
	if !*noBrowser {
		go func() { time.Sleep(300 * time.Millisecond); openBrowser(local) }()
	}
	srv := &http.Server{Handler: newMux(), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		showError("Research Digest stopped: " + err.Error())
		os.Exit(1)
	case <-life.quit:
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}
