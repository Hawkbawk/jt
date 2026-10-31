// Command fakejira serves an in-memory fake of the Jira Cloud API with demo
// data, and points a jt settings file at it, so jt can be run without a Jira
// site:
//
//	go run ./tools/fakejira
//	JT_CONFIG_FILE=/tmp/jt-fakejira.yaml go run .
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/fakejira"
)

const token = "fakejira"

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "address to listen on")
	cfgPath := flag.String("config", filepath.Join(os.TempDir(), "jt-fakejira.yaml"), "jt settings file to point at the server")
	flag.Parse()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	server := serverURL(ln.Addr().(*net.TCPAddr))

	if err := writeSettings(*cfgPath, server); err != nil {
		log.Fatalf("writing %s: %v", *cfgPath, err)
	}
	if err := config.SaveToken(fakejira.Login, token); err != nil {
		log.Printf("couldn't save the token to the keyring (%v); add any token for %s under the service \"jt\"", err, fakejira.Login)
	}

	fmt.Printf("Fake Jira listening on %s\n\nRun jt against it with:\n\n  JT_CONFIG_FILE=%s jt\n\n", server, *cfgPath)
	srv := &http.Server{Handler: logRequests(fakejira.New()), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.Serve(ln))
}

func serverURL(a *net.TCPAddr) string {
	host := a.IP.String()
	if a.IP.IsUnspecified() {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, fmt.Sprint(a.Port))
}

// writeSettings points the settings file at the server, keeping any other
// settings such as the theme.
func writeSettings(path, server string) error {
	if err := os.Setenv("JT_CONFIG_FILE", path); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return cfg.SaveJira(config.Jira{Server: server, Login: fakejira.Login, Project: fakejira.DefaultProject})
}

func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.RequestURI())
		h.ServeHTTP(w, r)
	})
}
