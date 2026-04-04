package haonewsteam

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	teamcore "hao.news/internal/haonews/team"
	newsplugin "hao.news/internal/plugins/haonews"
)

func handleTeamArchiveIndex(app *newsplugin.App, store *teamcore.Store, w http.ResponseWriter, r *http.Request) {
	teams, err := store.ListTeamsCtx(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	archivedTeams := 0
	for _, item := range teams {
		archives, err := store.ListArchivesCtx(r.Context(), item.TeamID)
		if err == nil && len(archives) > 0 {
			archivedTeams++
		}
	}
	data := teamArchiveIndexPageData{
		Project:    app.ProjectName(),
		Version:    app.VersionString(),
		PageNav:    app.PageNav("/archive"),
		NodeStatus: app.NodeStatus(index),
		Now:        time.Now(),
		Teams:      teams,
		SummaryStats: []newsplugin.SummaryStat{
			{Label: "团队", Value: formatTeamCount(len(teams))},
			{Label: "已归档团队", Value: formatTeamCount(archivedTeams)},
			{Label: "最近更新", Value: latestTeamValue(teams)},
		},
	}
	if err := app.Templates().ExecuteTemplate(w, "team_archive_index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleTeamArchive(app *newsplugin.App, store *teamcore.Store, teamID, archiveID string, w http.ResponseWriter, r *http.Request) {
	info, err := store.LoadTeamCtx(r.Context(), teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	archives, err := store.ListArchivesCtx(r.Context(), teamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var selected *teamcore.ArchiveSnapshot
	if archiveID != "" {
		item, err := store.LoadArchiveCtx(r.Context(), teamID, archiveID)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		selected = &item
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := teamArchivePageData{
		Project:    app.ProjectName(),
		Version:    app.VersionString(),
		PageNav:    app.PageNav("/archive"),
		NodeStatus: app.NodeStatus(index),
		Now:        time.Now(),
		Team:       info,
		Archives:   archives,
		Archive:    selected,
		SummaryStats: []newsplugin.SummaryStat{
			{Label: "归档批次", Value: formatTeamCount(len(archives))},
			{Label: "任务", Value: archiveTaskValue(selected)},
			{Label: "产物", Value: archiveArtifactValue(selected)},
		},
	}
	if err := app.Templates().ExecuteTemplate(w, "team_archive.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleTeamArchiveCreate(store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !teamRequestTrusted(r) {
		http.Error(w, "team archive writes are limited to local or LAN requests", http.StatusForbidden)
		return
	}
	if err := requireTeamAction(store, teamID, strings.TrimSpace(r.FormValue("actor_agent_id")), "archive.create"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	record, err := store.CreateManualArchiveCtx(r.Context(), teamID, time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/archive/team/"+teamID+"/"+record.ArchiveID, http.StatusSeeOther)
}

func handleAPITeamArchiveIndex(store *teamcore.Store, w http.ResponseWriter, r *http.Request) {
	teams, err := store.ListTeamsCtx(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	items := make([]map[string]any, 0, len(teams))
	for _, item := range teams {
		archives, err := store.ListArchivesCtx(r.Context(), item.TeamID)
		if err != nil {
			continue
		}
		summary := map[string]any{
			"team_id":       item.TeamID,
			"title":         item.Title,
			"archive_count": len(archives),
		}
		if len(archives) > 0 {
			summary["last_archive_id"] = archives[0].ArchiveID
			summary["last_archived_at"] = archives[0].ArchivedAt
		}
		items = append(items, summary)
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope": "team-archive-index",
		"teams": items,
	})
}

func handleAPITeamArchive(store *teamcore.Store, teamID, archiveID string, w http.ResponseWriter, r *http.Request) {
	if archiveID == "" {
		archives, err := store.ListArchivesCtx(r.Context(), teamID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
			"scope":    "team-archive-list",
			"team_id":  teamID,
			"archives": archives,
		})
		return
	}
	record, err := store.LoadArchiveCtx(r.Context(), teamID, archiveID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":   "team-archive-detail",
		"team_id": teamID,
		"archive": record,
	})
}

func handleAPITeamArchiveCreate(store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !teamRequestTrusted(r) {
		http.Error(w, "team archive writes are limited to local or LAN requests", http.StatusForbidden)
		return
	}
	var payload struct {
		ActorAgentID string `json:"actor_agent_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	if err := requireTeamAction(store, teamID, payload.ActorAgentID, "archive.create"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	record, err := store.CreateManualArchiveCtx(r.Context(), teamID, time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":   "team-archive-create",
		"team_id": teamID,
		"archive": record,
	})
}

func archiveTaskValue(item *teamcore.ArchiveSnapshot) string {
	if item == nil {
		return "0"
	}
	return formatTeamCount(item.TaskCount)
}

func archiveArtifactValue(item *teamcore.ArchiveSnapshot) string {
	if item == nil {
		return "0"
	}
	return formatTeamCount(item.ArtifactCount)
}
