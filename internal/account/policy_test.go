package account

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestPolicyCacheRefreshExpiryAndDisable(t *testing.T) {
	var status atomic.Int32
	status.Store(http.StatusOK)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != policyPath || r.Header.Get("Authorization") != "" {
			t.Error("cache queried a private endpoint or sent credentials")
		}
		httpStatus := int(status.Load())
		w.WriteHeader(httpStatus)
		if httpStatus == http.StatusOK {
			fmt.Fprintf(w, `{"data":{"version":1,"status":"available","policyRevision":%q,"lang":%q,"signupCredit":{"status":"enabled","automatic":true,"currency":"USD","eligibleChannels":["agent_wallet"],"amountMicroUsd":"100000"}}}`,
				r.URL.Query().Get("lang")+"-revision", r.URL.Query().Get("lang"))
		}
	}))
	defer upstream.Close()
	client, err := NewClient(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	cache := NewPolicyCache(client)
	if cache.Current("en") != nil {
		t.Fatal("empty cache returned benefits")
	}
	cache.Refresh(t.Context())
	if cache.Current("").TrialAmount() != 100000 || cache.Current("ja").PolicyRevision != "ja-revision" {
		t.Fatal("localized policy not loaded")
	}
	status.Store(http.StatusServiceUnavailable)
	cache.Refresh(t.Context())
	if cache.Current("en") == nil {
		t.Fatal("transient failure discarded fresh policy")
	}
	if cache.current("en", time.Now().Add(policyMaxAge)) != nil {
		t.Fatal("expired policy returned")
	}
	status.Store(http.StatusNotFound)
	cache.Refresh(t.Context())
	if cache.Current("en") != nil {
		t.Fatal("disabled public endpoint kept offering cached credit")
	}
}
