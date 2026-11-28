// Preview the embedded guide without credentials, a database or MCP execution.
package main

import (
	"flag"
	"log"
	"net/http"

	publicdocs "github.com/openany/openany-mcp/docs"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:5290", "local preview address")
	flag.Parse()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/docs/mcp" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		lang, body := "en", publicdocs.MCPEnglish
		if r.URL.Query().Get("lang") == "zh" {
			lang, body = "zh", publicdocs.MCP
		}
		w.Header().Set("X-Robots-Tag", "noindex")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodGet {
			w.Write([]byte(publicdocs.HTML(body, lang)))
		}
	})
	log.Printf("Guide-only preview: http://%s", *listen)
	log.Fatal(http.ListenAndServe(*listen, handler))
}
