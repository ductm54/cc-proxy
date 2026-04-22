package proxy

import (
	"context"
	"testing"

	"github.com/ductm54/cc-proxy/internal/config"
	"github.com/ductm54/cc-proxy/internal/httputil"
	"github.com/ductm54/cc-proxy/internal/tokens"
)

func TestIntegration_FetchAccountUsage(t *testing.T) {
	t.Skip("integration test: requires valid tokens at default location")

	tok, err := tokens.Load(config.DefaultTokensFile())
	if err != nil {
		t.Fatalf("Load tokens: %v", err)
	}

	client := httputil.NewLoggingClient(t)
	usage, err := FetchAccountUsage(context.Background(), client, "", tok.AccessToken)
	if err != nil {
		t.Fatalf("FetchAccountUsage: %v", err)
	}

	t.Logf("5h utilization:  %.2f%%", usage.FiveHour.Utilization*100)
	if usage.FiveHour.ResetsAt != nil {
		t.Logf("5h resets at:    %v", *usage.FiveHour.ResetsAt)
	}
	t.Logf("7d utilization:  %.2f%%", usage.SevenDay.Utilization*100)
	if usage.SevenDay.ResetsAt != nil {
		t.Logf("7d resets at:    %v", *usage.SevenDay.ResetsAt)
	}
	if usage.ExtraUsage != nil {
		t.Logf("extra usage:     enabled=%v used=%.2f/%.2f %s",
			usage.ExtraUsage.IsEnabled,
			usage.ExtraUsage.UsedCredit,
			usage.ExtraUsage.MonthlyLmt,
			usage.ExtraUsage.Currency)
	}
}
