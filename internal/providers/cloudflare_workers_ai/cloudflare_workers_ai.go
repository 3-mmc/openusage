package cloudflare_workers_ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/janekbaraniewski/openusage/internal/core"
	"github.com/janekbaraniewski/openusage/internal/providers/providerbase"
	"github.com/janekbaraniewski/openusage/internal/providers/shared"
)

const (
	defaultBaseURL = "https://api.cloudflare.com/client/v4"
	// Workers AI includes 10,000 neurons per day on both Free and Paid plans;
	// Paid accounts are billed per 1,000 neurons beyond that.
	dailyFreeNeurons = 10000.0
)

// Provider covers Cloudflare Workers AI. Account-scoped API tokens can read
// the GraphQL analytics dataset aiInferenceAdaptiveGroups, which gives daily
// neuron consumption against the free daily allocation.
type Provider struct {
	providerbase.Base
}

func New() *Provider {
	return &Provider{
		Base: providerbase.New(core.ProviderSpec{
			ID: "cloudflare_workers_ai",
			Info: core.ProviderInfo{
				Name:         "Cloudflare Workers AI",
				Capabilities: []string{"daily_neurons", "telemetry"},
				DocURL:       "https://developers.cloudflare.com/workers-ai/platform/pricing/",
			},
			Auth: core.ProviderAuthSpec{
				Type:             core.ProviderAuthTypeAPIKey,
				APIKeyEnv:        "CLOUDFLARE_API_KEY",
				DefaultAccountID: "cloudflare-workers-ai",
			},
			Setup: core.ProviderSetupSpec{
				Quickstart: []string{"Set CLOUDFLARE_API_KEY and CLOUDFLARE_ACCOUNT_ID (an account API token with Workers AI + Analytics read)."},
			},
			Dashboard: providerbase.DefaultDashboard(providerbase.WithColorRole(core.DashboardColorRolePeach)),
		}),
	}
}

func cloudflareAccountID(acct core.AccountConfig) string {
	if id := strings.TrimSpace(acct.Hint("account_id", "")); id != "" {
		return id
	}
	return strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID"))
}

type graphqlResponse struct {
	Data struct {
		Viewer struct {
			Accounts []struct {
				Groups []struct {
					Count int `json:"count"`
					Sum   struct {
						TotalNeurons float64 `json:"totalNeurons"`
					} `json:"sum"`
					Dimensions struct {
						Date string `json:"date"`
					} `json:"dimensions"`
				} `json:"aiInferenceAdaptiveGroups"`
			} `json:"accounts"`
		} `json:"viewer"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

const neuronsQuery = `query($a:String!,$s:Date!){viewer{accounts(filter:{accountTag:$a}){aiInferenceAdaptiveGroups(limit:100,filter:{date_geq:$s}){count sum{totalNeurons} dimensions{date}}}}}`

func (p *Provider) Fetch(ctx context.Context, acct core.AccountConfig) (core.UsageSnapshot, error) {
	accountID := cloudflareAccountID(acct)
	if accountID == "" {
		snap := core.NewAuthSnapshot(p.ID(), acct.ID, "no Cloudflare account id (set CLOUDFLARE_ACCOUNT_ID)")
		return snap, nil
	}
	baseURL := shared.ResolveBaseURL(acct, defaultBaseURL)
	snap, ok, err := shared.CheckKey(ctx, p.Client(), p.ID(), acct, baseURL+"/accounts/"+accountID+"/tokens/verify", "CLOUDFLARE_API_KEY", nil)
	if !ok {
		return snap, err
	}

	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	since := now.AddDate(0, 0, -29).Format("2006-01-02")
	gql, err := p.queryNeurons(ctx, baseURL, acct.ResolveAPIKey(), accountID, since)
	if err != nil {
		snap.Message = "Token valid · analytics unavailable"
		snap.SetDiagnostic("analytics_error", err.Error())
		return snap, nil
	}

	var todayNeurons, monthNeurons float64
	var todayRequests, monthRequests int
	for _, a := range gql.Data.Viewer.Accounts {
		for _, g := range a.Groups {
			monthNeurons += g.Sum.TotalNeurons
			monthRequests += g.Count
			if g.Dimensions.Date == today {
				todayNeurons += g.Sum.TotalNeurons
				todayRequests += g.Count
			}
		}
	}

	limit := dailyFreeNeurons
	remaining := limit - todayNeurons
	if remaining < 0 {
		remaining = 0
	}
	snap.Metrics["quota"] = core.Metric{Used: &todayNeurons, Limit: &limit, Remaining: &remaining, Unit: "neurons", Window: "1d"}
	snap.Resets["quota"] = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	todayReq, monthReq := float64(todayRequests), float64(monthRequests)
	snap.Metrics["requests_today"] = core.Metric{Used: &todayReq, Unit: "requests", Window: "1d"}
	snap.Metrics["neurons_30d"] = core.Metric{Used: &monthNeurons, Unit: "neurons", Window: "30d"}
	snap.Metrics["requests_30d"] = core.Metric{Used: &monthReq, Unit: "requests", Window: "30d"}
	snap.Message = fmt.Sprintf("%.0f / %.0f free neurons today", todayNeurons, limit)
	return snap, nil
}

func (p *Provider) queryNeurons(ctx context.Context, baseURL, apiKey, accountID, since string) (*graphqlResponse, error) {
	body, err := json.Marshal(map[string]any{
		"query":     neuronsQuery,
		"variables": map[string]string{"a": accountID, "s": since},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/graphql", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("graphql HTTP %d", resp.StatusCode)
	}
	var out graphqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("graphql: %s", out.Errors[0].Message)
	}
	return &out, nil
}
