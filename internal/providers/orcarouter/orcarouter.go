package orcarouter

import (
	"context"
	"fmt"

	"github.com/janekbaraniewski/openusage/internal/core"
	"github.com/janekbaraniewski/openusage/internal/providers/providerbase"
	"github.com/janekbaraniewski/openusage/internal/providers/shared"
)

const defaultBaseURL = "https://api.orcarouter.ai/v1"

// Provider covers OrcaRouter, whose /v1/balance reports paid balance plus
// per-model free and promo credits.
type Provider struct {
	providerbase.Base
}

func New() *Provider {
	return &Provider{
		Base: providerbase.New(core.ProviderSpec{
			ID: "orcarouter",
			Info: core.ProviderInfo{
				Name:         "OrcaRouter",
				Capabilities: []string{"balance", "telemetry"},
				DocURL:       "https://docs.orcarouter.ai",
			},
			Auth: core.ProviderAuthSpec{
				Type:             core.ProviderAuthTypeAPIKey,
				APIKeyEnv:        "ORCAROUTER_API_KEY",
				DefaultAccountID: "orcarouter",
			},
			Setup: core.ProviderSetupSpec{
				Quickstart: []string{"Set ORCAROUTER_API_KEY to a valid OrcaRouter API key."},
			},
			Dashboard: providerbase.DefaultDashboard(providerbase.WithColorRole(core.DashboardColorRoleSky)),
		}),
	}
}

type creditEntry struct {
	Model      string   `json:"model"`
	Balance    *float64 `json:"balance"`
	BalanceUSD *float64 `json:"balance_usd"`
}

func (c creditEntry) usd() float64 {
	if c.BalanceUSD != nil {
		return *c.BalanceUSD
	}
	if c.Balance != nil {
		return *c.Balance
	}
	return 0
}

type balanceResponse struct {
	Unit         string        `json:"unit"`
	PaidBalance  float64       `json:"paid_balance"`
	FreeCredit   []creditEntry `json:"free_credit"`
	PromoCredits []creditEntry `json:"promo_credits"`
}

func (p *Provider) Fetch(ctx context.Context, acct core.AccountConfig) (core.UsageSnapshot, error) {
	var bal balanceResponse
	url := shared.ResolveBaseURL(acct, defaultBaseURL) + "/balance"
	snap, ok, err := shared.CheckKey(ctx, p.Client(), p.ID(), acct, url, "ORCAROUTER_API_KEY", &bal)
	if !ok {
		return snap, err
	}

	free, promo := 0.0, 0.0
	for _, c := range bal.FreeCredit {
		free += c.usd()
	}
	for _, c := range bal.PromoCredits {
		promo += c.usd()
	}
	paid := bal.PaidBalance
	total := paid + free + promo

	snap.Metrics["credit_balance"] = core.Metric{Remaining: &total, Unit: "USD", Window: "current"}
	snap.Metrics["paid_balance"] = core.Metric{Remaining: &paid, Unit: "USD", Window: "current"}
	snap.Metrics["free_credit"] = core.Metric{Remaining: &free, Unit: "USD", Window: "current"}
	if len(bal.PromoCredits) > 0 {
		snap.Metrics["promo_credit"] = core.Metric{Remaining: &promo, Unit: "USD", Window: "current"}
	}
	for _, c := range bal.FreeCredit {
		if c.Model != "" {
			snap.SetAttribute("free_credit_model", c.Model)
		}
	}
	snap.Message = fmt.Sprintf("$%.2f balance ($%.2f paid)", total, paid)
	return snap, nil
}
