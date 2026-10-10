package cerebras

import (
	"context"
	"fmt"

	"github.com/janekbaraniewski/openusage/internal/core"
	"github.com/janekbaraniewski/openusage/internal/providers/providerbase"
	"github.com/janekbaraniewski/openusage/internal/providers/shared"
)

const defaultBaseURL = "https://api.cerebras.ai/v1"

// Provider covers Cerebras Inference. Rate-limit headers are only sent on
// completions, so the remote probe validates the key and usage comes from
// telemetry.
type Provider struct {
	providerbase.Base
}

func New() *Provider {
	return &Provider{
		Base: providerbase.New(core.ProviderSpec{
			ID: "cerebras",
			Info: core.ProviderInfo{
				Name:         "Cerebras",
				Capabilities: []string{"key_check", "telemetry"},
				DocURL:       "https://inference-docs.cerebras.ai/support/rate-limits",
			},
			Auth: core.ProviderAuthSpec{
				Type:             core.ProviderAuthTypeAPIKey,
				APIKeyEnv:        "CEREBRAS_API_KEY",
				DefaultAccountID: "cerebras",
			},
			Setup: core.ProviderSetupSpec{
				Quickstart: []string{"Set CEREBRAS_API_KEY to a valid Cerebras API key."},
			},
			Dashboard: providerbase.DefaultDashboard(providerbase.WithColorRole(core.DashboardColorRolePeach)),
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
	snap, ok, err := shared.CheckKey(ctx, p.Client(), p.ID(), acct, url, "CEREBRAS_API_KEY", &models)
	if !ok {
		return snap, err
	}
	snap.SetAttribute("models_available", fmt.Sprintf("%d", len(models.Data)))
	snap.Message = fmt.Sprintf("Key valid · %d models", len(models.Data))
	return snap, nil
}
