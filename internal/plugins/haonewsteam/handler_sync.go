package haonewsteam

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	corehaonews "hao.news/internal/haonews"
	teamcore "hao.news/internal/haonews/team"
	newsplugin "hao.news/internal/plugins/haonews"
)

func handleTeamSync(app *newsplugin.App, store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	info, err := store.LoadTeamCtx(r.Context(), teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	conflicts, err := corehaonews.LoadTeamSyncConflicts(app.StoreRoot(), teamID, corehaonews.TeamSyncConflictFilter{Limit: 10})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	status, err := loadTeamSyncRuntimeStatus(app.StoreRoot())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := teamSyncPageData{
		Project:         app.ProjectName(),
		Version:         app.VersionString(),
		PageNav:         app.PageNav("/teams"),
		NodeStatus:      app.NodeStatus(index),
		Now:             time.Now(),
		Team:            info,
		SyncNotice:      strings.TrimSpace(r.URL.Query().Get("resolved")),
		SyncStatus:      status.TeamSync,
		RecentConflicts: conflicts,
		ConflictViews:   buildTeamSyncConflictViews(conflicts),
		SummaryStats: []newsplugin.SummaryStat{
			{Label: "已订阅 Team", Value: formatTeamCount(status.TeamSync.SubscribedTeams)},
			{Label: "pending ack", Value: formatTeamCount(status.TeamSync.PendingAcks)},
			{Label: "ack peers", Value: formatTeamCount(status.TeamSync.AckPeers)},
			{Label: "冲突", Value: formatTeamCount(status.TeamSync.Conflicts)},
			{Label: "最近 publish", Value: formatTeamTimePtr(status.TeamSync.LastPublishedAt)},
			{Label: "最近 apply", Value: formatTeamTimePtr(status.TeamSync.LastAppliedAt)},
		},
	}
	if err := app.Templates().ExecuteTemplate(w, "team_sync.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleTeamSyncConflictResolvePage(app *newsplugin.App, store *teamcore.Store, teamID, conflictKey string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	record, err := resolveTeamSyncConflict(app, store, teamID, conflictKey, r.RemoteAddr, r.FormValue("actor_agent_id"), r.FormValue("action"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	redirectURL := "/teams/" + teamID + "/sync"
	if strings.TrimSpace(record.Resolution) != "" {
		redirectURL += "?resolved=" + record.Resolution
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func handleAPITeamSync(app *newsplugin.App, store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	info, err := store.LoadTeamCtx(r.Context(), teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	status, err := loadTeamSyncRuntimeStatus(app.StoreRoot())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	limit := clampTeamListLimit(r.URL.Query().Get("limit"), 10, 100)
	conflicts, err := corehaonews.LoadTeamSyncConflicts(app.StoreRoot(), teamID, corehaonews.TeamSyncConflictFilter{Limit: limit})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":            "team-sync-health",
		"team_id":          info.TeamID,
		"team_sync":        status.TeamSync,
		"conflict_count":   len(conflicts),
		"recent_conflicts": conflicts,
		"conflict_views":   buildTeamSyncConflictViews(conflicts),
	})
}

func handleAPITeamSyncConflicts(app *newsplugin.App, store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		limit := clampTeamListLimit(r.URL.Query().Get("limit"), 50, 200)
		filter := corehaonews.TeamSyncConflictFilter{
			Type:       strings.TrimSpace(r.URL.Query().Get("type")),
			SubjectID:  strings.TrimSpace(r.URL.Query().Get("subject_id")),
			SourceNode: strings.TrimSpace(r.URL.Query().Get("source_node")),
			Limit:      limit,
		}
		conflicts, err := corehaonews.LoadTeamSyncConflicts(app.StoreRoot(), teamID, filter)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
			"scope":          "team-sync-conflicts",
			"team_id":        teamID,
			"conflict_count": len(conflicts),
			"conflicts":      conflicts,
			"applied_filters": map[string]any{
				"type":        filter.Type,
				"subject_id":  filter.SubjectID,
				"source_node": filter.SourceNode,
				"limit":       limit,
			},
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func loadTeamSyncRuntimeStatus(storeRoot string) (corehaonews.SyncRuntimeStatus, error) {
	path := filepath.Join(strings.TrimSpace(storeRoot), "sync", "status.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return corehaonews.SyncRuntimeStatus{}, nil
	}
	if err != nil {
		return corehaonews.SyncRuntimeStatus{}, err
	}
	var status corehaonews.SyncRuntimeStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return corehaonews.SyncRuntimeStatus{}, err
	}
	return status, nil
}

func resolveTeamSyncConflict(app *newsplugin.App, store *teamcore.Store, teamID, conflictKey, remoteAddr, actorAgentID, action string) (corehaonews.TeamSyncConflictRecord, error) {
	request := &http.Request{RemoteAddr: remoteAddr}
	if !teamRequestTrusted(request) {
		return corehaonews.TeamSyncConflictRecord{}, errors.New("team sync conflict resolution is limited to local or LAN requests")
	}
	actorAgentID = strings.TrimSpace(actorAgentID)
	action = strings.TrimSpace(action)
	if err := requireTeamConflictAction(store, teamID, actorAgentID); err != nil {
		return corehaonews.TeamSyncConflictRecord{}, err
	}
	record, err := corehaonews.ResolveTeamSyncConflict(app.StoreRoot(), teamID, conflictKey, action, actorAgentID)
	if err != nil {
		return corehaonews.TeamSyncConflictRecord{}, err
	}
	_ = appendTeamHistory(store, historyActor{AgentID: actorAgentID, Source: "api"}, teamID, "sync-conflict", "resolve", conflictKey, "处理 Team 复制冲突", map[string]any{
		"diff_summary":      "复制冲突已处理",
		"reason_before":     record.Reason,
		"resolution_after":  record.Resolution,
		"subject_id_after":  record.SubjectID,
		"source_node_after": record.SourceNode,
		"sync_type_after":   record.SyncType,
	})
	return record, nil
}

func buildTeamSyncConflictViews(records []corehaonews.TeamSyncConflictRecord) []teamSyncConflictView {
	views := make([]teamSyncConflictView, 0, len(records))
	for _, record := range records {
		syncType := strings.TrimSpace(record.SyncType)
		if syncType == "" {
			syncType = strings.TrimSpace(record.Type)
		}
		views = append(views, teamSyncConflictView{
			Record:            record,
			AllowAcceptRemote: supportsAcceptRemoteConflict(syncType),
			SuggestedAction:   suggestedConflictAction(record, supportsAcceptRemoteConflict(syncType)),
		})
	}
	return views
}

func supportsAcceptRemoteConflict(syncType string) bool {
	switch strings.TrimSpace(syncType) {
	case "task", "artifact", "member", "policy", "channel":
		return true
	default:
		return false
	}
}

func suggestedConflictAction(record corehaonews.TeamSyncConflictRecord, allowAcceptRemote bool) string {
	reason := strings.TrimSpace(record.Reason)
	switch {
	case allowAcceptRemote && reason == "same_version_diverged":
		return "accept_remote"
	case reason == "local_newer":
		return "dismiss"
	case allowAcceptRemote:
		return "review_accept_remote"
	default:
		return "dismiss"
	}
}

func countUnresolvedTeamConflicts(conflicts []corehaonews.TeamSyncConflictRecord) int {
	count := 0
	for _, record := range conflicts {
		if strings.TrimSpace(record.Resolution) == "" {
			count++
		}
	}
	return count
}

func countResolvedTeamConflicts(storeRoot, teamID string) int {
	conflicts, err := corehaonews.LoadTeamSyncConflicts(storeRoot, teamID, corehaonews.TeamSyncConflictFilter{
		IncludeResolved: true,
		Limit:           200,
	})
	if err != nil {
		return 0
	}
	count := 0
	for _, record := range conflicts {
		if strings.TrimSpace(record.Resolution) != "" {
			count++
		}
	}
	return count
}

func handleAPITeamSyncConflictResolve(app *newsplugin.App, store *teamcore.Store, teamID, conflictKey string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		ActorAgentID string `json:"actor_agent_id"`
		Action       string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	record, err := resolveTeamSyncConflict(app, store, teamID, conflictKey, r.RemoteAddr, payload.ActorAgentID, payload.Action)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":    "team-sync-conflict-resolve",
		"team_id":  teamID,
		"conflict": record,
	})
}
