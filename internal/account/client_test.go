package account

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAccountClientPathsCredentialsAndRedirect(t *testing.T) {
	var redirectHits atomic.Int32
	redirect := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirectHits.Add(1)
	}))
	defer redirect.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Cookie") != "" || r.Header.Get("X-Other-Secret") != "" {
			t.Errorf("unexpected request method or credential headers")
		}
		switch r.URL.Path {
		case policyPath:
			if r.URL.RawQuery != "lang=ja" || r.Header.Get("Authorization") != "" {
				t.Errorf("public policy received unexpected query or credentials")
			}
			fmt.Fprint(w, `{"data":{"version":1,"status":"available","policyRevision":"revision","lang":"ja"}}`)
		case accountPath:
			if r.URL.RawQuery != "lang=zh" || r.Header.Get("Authorization") != "Bearer example-key" ||
				r.Header.Get("X-Api-Key") != "example-key" || r.Header.Get("Xi-Api-Key") != "example-key" {
				t.Errorf("account received unexpected query or credential headers")
			}
			http.Redirect(w, r, redirect.URL, http.StatusTemporaryRedirect)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer upstream.Close()
	client, err := NewClient(upstream.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.PublicPolicy(t.Context(), "ja"); err != nil {
		t.Fatal(err)
	}
	headers := make(http.Header)
	headers.Set("Authorization", "example-key")
	headers.Set("X-Api-Key", "example-key")
	headers.Set("Xi-Api-Key", "example-key")
	headers.Set("Cookie", "web-session=private")
	headers.Set("X-Other-Secret", "private")
	_, err = client.Summary(t.Context(), "zh", headers)
	var reply *ResponseError
	if !errors.As(err, &reply) || reply.Status != http.StatusTemporaryRedirect || redirectHits.Load() != 0 {
		t.Fatalf("redirect followed or misclassified: %v, %d hits", err, redirectHits.Load())
	}
	headers.Set("Authorization", "Bearer ${OPENANY_API_KEY}")
	if _, err := client.Summary(t.Context(), "zh", headers); err == nil {
		t.Fatal("placeholder accepted")
	}
	if _, err := client.PublicPolicy(t.Context(), "en&extra=anything"); err == nil {
		t.Fatal("invalid language accepted")
	}
}

func TestAccountClientResponseLimits(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("lang") == "zh" {
			<-r.Context().Done()
			return
		}
		fmt.Fprint(w, strings.Repeat("x", maxBody+1))
	}))
	defer upstream.Close()
	client, err := NewClient(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.PublicPolicy(t.Context(), "en"); err == nil {
		t.Fatal("oversized body accepted")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if _, err := client.PublicPolicy(ctx, "zh"); err == nil || ctx.Err() == nil {
		t.Fatal("query did not honor cancellation")
	}
	if client.http.Timeout != 2*time.Second {
		t.Fatal("unexpected total request budget")
	}
}

func TestAccountOriginRejectsPathsCredentialsAndQueries(t *testing.T) {
	for _, raw := range []string{"", "/local", "file:///local", "https://@example.org", "https://example.org/v1", "https://example.org?#", "https://example.org/#", "https://example.org/%2f", "https://example.org/?lang=en"} {
		if origin, err := ParseOrigin(raw); err == nil {
			t.Errorf("invalid origin %q accepted as %q", raw, origin)
		}
	}
	if origin, err := ParseOrigin("https://openany.si/"); err != nil || origin != "https://openany.si" {
		t.Fatal(origin, err)
	}
}
