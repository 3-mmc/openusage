package orcarouter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/janekbaraniewski/openusage/internal/core"
)

func TestFetch_SumsPaidFreeAndPromoCredits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/balance" || r.Header.Get("Authorization") != "Bearer test-key" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"object":"balance","unit":"USD","paid_balance":1.5,
			"free_credit":[{"model":"orca/dub","balance":6,"unit":"USD","balance_usd":6}],
			"promo_credits":[{"balance_usd":0.25}]}`))
	}))
	defer srv.Close()

	acct := core.AccountConfig{ID: "orcarouter", Provider: "orcarouter", Token: "test-key", BaseURL: srv.URL}
	snap, err := New().Fetch(context.Background(), acct)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Status != core.StatusOK {
		t.Fatalf("status = %q (%s)", snap.Status, snap.Message)
	}
	if got := *snap.Metrics["credit_balance"].Remaining; got != 7.75 {
		t.Fatalf("credit_balance = %v, want 7.75", got)
	}
	if snap.Message != "$7.75 balance ($1.50 paid)" {
		t.Fatalf("message = %q", snap.Message)
	}
}

func TestFetch_UnauthorizedIsAuthStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	acct := core.AccountConfig{ID: "orcarouter", Provider: "orcarouter", Token: "bad", BaseURL: srv.URL}
	snap, err := New().Fetch(context.Background(), acct)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Status != core.StatusAuth {
		t.Fatalf("status = %q, want AUTH", snap.Status)
	}
}
