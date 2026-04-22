package proxy

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleUsageSummary(w http.ResponseWriter, r *http.Request) {
	since, until := parseTimeRange(r)

	users, err := s.usage.QuerySummary(r.Context(), since, until)
	if err != nil {
		writeErrJSON(w, http.StatusInternalServerError, "cc_proxy_internal", err.Error())
		return
	}

	w.Header().Set(HeaderContentType, "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"since": since.Format(time.RFC3339),
		"until": until.Format(time.RFC3339),
		"users": users,
	})
}

func (s *Server) handleUsageByEmail(w http.ResponseWriter, r *http.Request) {
	email := chi.URLParam(r, "email")
	since, until := parseTimeRange(r)

	rows, err := s.usage.QueryByEmail(r.Context(), email, since, until)
	if err != nil {
		writeErrJSON(w, http.StatusInternalServerError, "cc_proxy_internal", err.Error())
		return
	}

	var totalInput, totalOutput, totalCacheCreate, totalCacheRead uint64
	var totalCost float64
	for _, row := range rows {
		totalInput += uint64(row.InputTokens)
		totalOutput += uint64(row.OutputTokens)
		totalCacheCreate += uint64(row.CacheCreationTokens)
		totalCacheRead += uint64(row.CacheReadTokens)
		totalCost += row.CostUSD
	}

	w.Header().Set(HeaderContentType, "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"email": email,
		"since": since.Format(time.RFC3339),
		"until": until.Format(time.RFC3339),
		"total": map[string]any{
			"request_count":         len(rows),
			"input_tokens":          totalInput,
			"output_tokens":         totalOutput,
			"cache_creation_tokens": totalCacheCreate,
			"cache_read_tokens":     totalCacheRead,
			"total_cost_usd":        totalCost,
		},
		"requests": rows,
	})
}

func parseTimeRange(r *http.Request) (since, until time.Time) {
	now := time.Now()

	if v := r.URL.Query().Get("range"); v != "" {
		switch v {
		case "today":
			y, m, d := now.Date()
			since = time.Date(y, m, d, 0, 0, 0, 0, now.Location())
			until = now
		case "this_week":
			y, m, d := now.Date()
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			since = time.Date(y, m, d-weekday+1, 0, 0, 0, 0, now.Location())
			until = now
		case "last_week":
			y, m, d := now.Date()
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			endOfLastWeek := time.Date(y, m, d-weekday+1, 0, 0, 0, 0, now.Location())
			since = endOfLastWeek.AddDate(0, 0, -7)
			until = endOfLastWeek
		case "this_month":
			y, m, _ := now.Date()
			since = time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
			until = now
		default:
			since = now.AddDate(0, -1, 0)
			until = now
		}
		return since, until
	}

	since = parseTime(r.URL.Query().Get("since"), now.AddDate(0, -1, 0))
	until = parseTime(r.URL.Query().Get("until"), now)
	return since, until
}

func parseTime(v string, fallback time.Time) time.Time {
	if v == "" {
		return fallback
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t
	}
	return fallback
}
