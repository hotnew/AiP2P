package aip2pteam

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	teamcore "aip2p/internal/aip2p/team"
	newsplugin "aip2p/internal/plugins/aip2p"
)

func handleAPITeamWebhookStatus(store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status, err := store.LoadWebhookDeliveryStatusCtx(r.Context(), teamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":               "team-webhook-status",
		"team_id":             teamID,
		"retrying_count":      status.RetryingCount,
		"delivered_count":     status.DeliveredCount,
		"failed_count":        status.FailedCount,
		"dead_letter_count":   status.DeadLetterCount,
		"recent_failures":     status.RecentFailures,
		"recent_dead_letters": status.RecentDead,
		"recent_delivered":    status.RecentDelivered,
	})
}

func handleAPITeamWebhookReplay(store *teamcore.Store, teamID, deliveryID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !teamRequestTrusted(r) {
		http.Error(w, "team webhook replay is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	var payload struct {
		ActorAgentID string `json:"actor_agent_id"`
	}
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") && r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	} else {
		_ = r.ParseForm()
		payload.ActorAgentID = strings.TrimSpace(r.FormValue("actor_agent_id"))
	}
	if err := requireTeamAction(store, teamID, payload.ActorAgentID, "policy.update"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	record, err := store.ReplayWebhookDeliveryCtx(r.Context(), teamID, deliveryID)
	if err != nil {
		status := http.StatusInternalServerError
		if err == context.Canceled || err == context.DeadlineExceeded {
			status = http.StatusRequestTimeout
		} else if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	if acceptsHTML(r) {
		http.Redirect(w, r, "/teams/"+teamID+"/webhooks?replayed="+url.QueryEscape(record.DeliveryID), http.StatusSeeOther)
		return
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":       "team-webhook-replay",
		"team_id":     teamID,
		"delivery_id": record.DeliveryID,
		"status":      record.Status,
		"replayed_at": time.Now().UTC(),
		"record":      record,
	})
}

func acceptsHTML(r *http.Request) bool {
	if r == nil {
		return false
	}
	accept := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept")))
	if strings.Contains(accept, "text/html") {
		return true
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(r.Referer())), "/webhooks")
}
