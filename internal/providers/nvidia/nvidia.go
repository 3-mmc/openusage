package nvidia

import (
	"context"
	"fmt"

	"github.com/janekbaraniewski/openusage/internal/core"
	"github.com/janekbaraniewski/openusage/internal/providers/providerbase"
	"github.com/janekbaraniewski/openusage/internal/providers/shared"
)

const defaultBaseURL = "https://integrate.api.nvidia.com/v1"

// Provider covers the NVIDIA API catalog (build.nvidia.com). The trial tier
// has no credit balance and no rate-limit headers, only undisclosed per-model
// rate limits, so the remote probe validates the key and usage comes from
// telemetry (e.g. OpenCode's "nvidia" provider).
type Provider struct {
	providerbase.Base
}

func New() *Provider {
	return &Provider{
		Base: providerbase.New(core.ProviderSpec{
			ID: "nvidia",
			Info: core.ProviderInfo{
				Name:         "NVIDIA NIM",
				Capabilities: []string{"key_check", "telemetry"},
				DocURL:       "https://docs.api.nvidia.com/nim/",
			},
			Auth: core.ProviderAuthSpec{
				Type:             core.ProviderAuthTypeAPIKey,
				APIKeyEnv:        "NVIDIA_API_KEY",
				DefaultAccountID: "nvidia",
			},
			Setup: core.ProviderSetupSpec{
				Quickstart: []string{"Set NVIDIA_API_KEY to a build.nvidia.com API key."},
			},
			Dashboard: providerbase.DefaultDashboard(providerbase.WithColorRole(core.DashboardColorRoleGreen)),
		}),
	}
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (p *Provider) Fetch(ctx context.Context, acct core.AccountConfig) (core.UsageSnapshot, error) {
	var models modelsResponse
	url := shared.ResolveBaseURL(acct, defaultBaseURL) + "/models"
	snap, ok, err := shared.CheckKey(ctx, p.Client(), p.ID(), acct, url, "NVIDIA_API_KEY", &models)
	if !ok {
		return snap, err
	}
	snap.SetAttribute("models_available", fmt.Sprintf("%d", len(models.Data)))
	snap.Message = "Trial · rate-limited, no credit cap"
	return snap, nil
}
