package shared

import (
	"context"
	"fmt"
	"net/http"

	"github.com/janekbaraniewski/openusage/internal/core"
)

// CheckKey validates an API key with an authenticated GET to url, decoding
// the body into out when it is non-nil. It is for providers whose only
// remote signal is "the key works" (usage arrives through telemetry).
//
// The returned snapshot already carries any auth/limit status. When ok is
// false the caller should return the snapshot as-is.
func CheckKey(ctx context.Context, client *http.Client, providerID string, acct core.AccountConfig, url, keyHint string, out any) (snap core.UsageSnapshot, ok bool, err error) {
	apiKey, authSnap := RequireAPIKey(acct, providerID)
	if authSnap != nil {
		return *authSnap, false, nil
	}
	snap = core.NewUsageSnapshot(providerID, acct.ID)
	statusCode, _, err := FetchJSON(ctx, url, apiKey, out, client)
	if err != nil {
		ApplyStatusFromCode(statusCode, &snap, keyHint)
		if snap.Status != "" && snap.Status != core.StatusOK {
			return snap, false, nil
		}
		return snap, false, fmt.Errorf("%s: %w", providerID, err)
	}
	snap.Status = core.StatusOK
	return snap, true, nil
}
