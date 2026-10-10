package cloudflare_workers_ai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/janekbaraniewski/openusage/internal/core"
)

func TestFetch_ProjectsTodaysNeuronsAgainstDailyAllowance(t *testing.T) {
	today := time.Now().UTC().Format("2006-01-02")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/accounts/acc123/tokens/verify":
			w.Write([]byte(`{"success":true,"result":{"status":"active"}}`))
		case "/graphql":
			fmt.Fprintf(w, `{"data":{"viewer":{"accounts":[{"aiInferenceAdaptiveGroups":[
				{"count":3,"sum":{"totalNeurons":2500},"dimensions":{"date":%q}},
				{"count":7,"sum":{"totalNeurons":100},"dimensions":{"date":"2000-01-01"}}]}]}},"errors":null}`, today)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	acct := core.AccountConfig{ID: "cloudflare-workers-ai", Provider: "cloudflare_workers_ai", Token: "tok", BaseURL: srv.URL}
	acct.SetHint("account_id", "acc123")
	snap, err := New().Fetch(context.Background(), acct)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	q := snap.Metrics["quota"]
	if q.Used == nil || *q.Used != 2500 || *q.Remaining != 7500 || *q.Limit != dailyFreeNeurons {
		t.Fatalf("quota = %+v", q)
	}
	if got := core.MetricUsedPercent("quota", q); got != 25 {
		t.Fatalf("used percent = %v, want 25", got)
	}
	if got := *snap.Metrics["requests_30d"].Used; got != 10 {
		t.Fatalf("requests_30d = %v, want 10", got)
	}
}

func TestFetch_MissingAccountIDIsAuthStatus(t *testing.T) {
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "")
	acct := core.AccountConfig{ID: "cloudflare-workers-ai", Provider: "cloudflare_workers_ai", Token: "tok"}
	snap, _ := New().Fetch(context.Background(), acct)
	if snap.Status != core.StatusAuth {
		t.Fatalf("status = %q, want AUTH", snap.Status)
	}
}
