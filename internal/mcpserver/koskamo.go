package mcpserver

import (
	"net/http"
	"strconv"
)

const koskamoClaimPath = "/.well-known/koskamo-claim"

const koskamoClaimBody = "koskamo-claim=fBvJozPROrZ9fufMAMhI1e3w\n"

func serveKoskamoClaim(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(koskamoClaimBody)))
	if r.Method == http.MethodGet {
		w.Write([]byte(koskamoClaimBody))
	}
}
