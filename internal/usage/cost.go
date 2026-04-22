package usage

import "strings"

type modelPricing struct {
	InputPerMillion      float64
	OutputPerMillion     float64
	CacheWritePerMillion float64
	CacheReadPerMillion  float64
}

// Keyed by model prefix — longest prefix match wins.
var pricing = []struct {
	prefix  string
	pricing modelPricing
}{
	{"claude-opus-4-7", modelPricing{5.00, 25.00, 6.25, 0.50}},
	{"claude-opus-4-6", modelPricing{5.00, 25.00, 6.25, 0.50}},
	{"claude-opus-4-5", modelPricing{5.00, 25.00, 6.25, 0.50}},
	{"claude-opus-4-1", modelPricing{15.00, 75.00, 18.75, 1.50}},
	{"claude-opus-4-0", modelPricing{15.00, 75.00, 18.75, 1.50}},
	{"claude-sonnet-4", modelPricing{3.00, 15.00, 3.75, 0.30}},
	{"claude-haiku-4-5", modelPricing{1.00, 5.00, 1.25, 0.10}},
	{"claude-3-5-sonnet", modelPricing{3.00, 15.00, 3.75, 0.30}},
	{"claude-3-5-haiku", modelPricing{0.80, 4.00, 1.00, 0.08}},
	{"claude-3-opus", modelPricing{15.00, 75.00, 18.75, 1.50}},
	{"claude-3-haiku", modelPricing{0.25, 1.25, 0.30, 0.03}},
}

var defaultPricing = modelPricing{3.00, 15.00, 3.75, 0.30}

type TokenUsage struct {
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
}

func lookupPricing(model string) modelPricing {
	for _, e := range pricing {
		if strings.HasPrefix(model, e.prefix) {
			return e.pricing
		}
	}
	return defaultPricing
}

func ComputeCost(model string, u TokenUsage) float64 {
	p := lookupPricing(model)
	return (float64(u.InputTokens)*p.InputPerMillion +
		float64(u.OutputTokens)*p.OutputPerMillion +
		float64(u.CacheCreationTokens)*p.CacheWritePerMillion +
		float64(u.CacheReadTokens)*p.CacheReadPerMillion) / 1_000_000
}
