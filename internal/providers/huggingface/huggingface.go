package huggingface

import (
	"context"

	"github.com/janekbaraniewski/openusage/internal/core"
	"github.com/janekbaraniewski/openusage/internal/providers/providerbase"
	"github.com/janekbaraniewski/openusage/internal/providers/shared"
)

const defaultWhoamiURL = "https://huggingface.co/api/whoami-v2"

// Provider covers Hugging Face Inference Providers. The Hub exposes no
// billing or credit-balance API (only the settings web page), so the probe
// reports the plan from whoami-v2 and usage comes from telemetry.
type Provider struct {
	providerbase.Base
}

func New() *Provider {
	return &Provider{
		Base: providerbase.New(core.ProviderSpec{
			ID: "huggingface",
			Info: core.ProviderInfo{
				Name:         "Hugging Face",
				Capabilities: []string{"key_check", "plan", "telemetry"},
				DocURL:       "https://huggingface.co/docs/inference-providers/pricing",
			},
			Auth: core.ProviderAuthSpec{
				Type:             core.ProviderAuthTypeAPIKey,
				APIKeyEnv:        "HF_TOKEN",
				DefaultAccountID: "huggingface",
			},
			Setup: core.ProviderSetupSpec{
				Quickstart: []string{"Set HF_TOKEN to a Hugging Face access token."},
			},
			Dashboard: providerbase.DefaultDashboard(providerbase.WithColorRole(core.DashboardColorRoleYellow)),
		}),
	}
}

type whoamiResponse struct {
	Name  string `json:"name"`
	IsPro bool   `json:"isPro"`
}

func (p *Provider) Fetch(ctx context.Context, acct core.AccountConfig) (core.UsageSnapshot, error) {
	var who whoamiResponse
	url := acct.BaseURL
	if url == "" {
		url = defaultWhoamiURL
	}
	snap, ok, err := shared.CheckKey(ctx, p.Client(), p.ID(), acct, url, "HF_TOKEN", &who)
	if !ok {
		return snap, err
	}
	if who.Name != "" {
		snap.SetAttribute("account", who.Name)
	}
	// Monthly included Inference Providers credits per the pricing docs.
	if who.IsPro {
		snap.SetAttribute("plan", "PRO")
		snap.Message = "PRO · $2.00/mo included credits"
	} else {
		snap.SetAttribute("plan", "Free")
		snap.Message = "Free · $0.10/mo included credits"
	}
	return snap, nil
}
