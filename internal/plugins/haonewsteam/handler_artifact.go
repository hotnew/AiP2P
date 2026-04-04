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

func handleTeamArtifacts(app *newsplugin.App, store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	info, err := store.LoadTeamCtx(r.Context(), teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	artifacts, err := store.LoadArtifactsCtx(r.Context(), teamID, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tasks, err := store.LoadTasksCtx(r.Context(), teamID, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	channels, err := store.ListChannelsCtx(r.Context(), teamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filterKind := strings.TrimSpace(r.URL.Query().Get("kind"))
	filterChannel := normalizeTeamChannel(r.URL.Query().Get("channel"))
	filterTask := strings.TrimSpace(r.URL.Query().Get("task"))
	kinds := artifactKinds(artifacts)
	filtered := filterArtifacts(artifacts, filterKind, filterChannel, filterTask)
	data := teamArtifactsPageData{
		Project:       app.ProjectName(),
		Version:       app.VersionString(),
		PageNav:       app.PageNav("/teams"),
		NodeStatus:    app.NodeStatus(index),
		Now:           time.Now(),
		Team:          info,
		Artifacts:     filtered,
		FilterKind:    filterKind,
		FilterChannel: filterChannel,
		FilterTask:    filterTask,
		AppliedFilters: appliedTeamFilters(
			labeledTeamFilter("类型", filterKind),
			labeledTeamFilter("频道", filterChannel),
			labeledTeamFilter("任务", filterTask),
		),
		Kinds:    kinds,
		Channels: channels,
		Tasks:    artifactFilterTasks(tasks, artifacts),
		SummaryStats: []newsplugin.SummaryStat{
			{Label: "产物", Value: formatTeamCount(len(filtered))},
			{Label: "Markdown", Value: formatTeamCount(countArtifactsByKind(filtered, "markdown"))},
			{Label: "链接", Value: formatTeamCount(countArtifactsByKind(filtered, "link"))},
		},
	}
	if err := app.Templates().ExecuteTemplate(w, "team_artifacts.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleTeamArtifact(app *newsplugin.App, store *teamcore.Store, teamID, artifactID string, w http.ResponseWriter, r *http.Request) {
	info, err := store.LoadTeamCtx(r.Context(), teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	artifact, err := store.LoadArtifactCtx(r.Context(), teamID, artifactID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	artifacts, err := store.LoadArtifactsCtx(r.Context(), teamID, 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	channels, err := store.ListChannelsCtx(r.Context(), teamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	history, err := store.LoadHistoryCtx(r.Context(), teamID, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var relatedTask *teamcore.Task
	if strings.TrimSpace(artifact.TaskID) != "" {
		task, err := store.LoadTaskCtx(r.Context(), teamID, artifact.TaskID)
		if err == nil {
			relatedTask = &task
		}
	}
	var relatedChannel *teamcore.ChannelSummary
	if strings.TrimSpace(artifact.ChannelID) != "" {
		for _, channel := range channels {
			if normalizeTeamChannel(channel.ChannelID) == normalizeTeamChannel(artifact.ChannelID) {
				channelCopy := channel
				relatedChannel = &channelCopy
				break
			}
		}
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := teamArtifactPageData{
		Project:        app.ProjectName(),
		Version:        app.VersionString(),
		PageNav:        app.PageNav("/teams"),
		NodeStatus:     app.NodeStatus(index),
		Now:            time.Now(),
		Team:           info,
		Artifact:       artifact,
		Artifacts:      artifacts,
		RelatedTask:    relatedTask,
		RelatedChannel: relatedChannel,
		RelatedHistory: artifactHistory(history, artifactID, 8),
		SummaryStats: []newsplugin.SummaryStat{
			{Label: "类型", Value: artifact.Kind},
			{Label: "频道", Value: artifact.ChannelID},
			{Label: "标签", Value: formatTeamCount(len(artifact.Labels))},
		},
	}
	if err := app.Templates().ExecuteTemplate(w, "team_artifact.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleTeamArtifactCreate(store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	if !teamRequestTrusted(r) {
		http.Error(w, "team artifact update is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	payload := teamcore.Artifact{
		ArtifactID:      strings.TrimSpace(r.FormValue("artifact_id")),
		ChannelID:       strings.TrimSpace(r.FormValue("channel_id")),
		TaskID:          strings.TrimSpace(r.FormValue("task_id")),
		Title:           strings.TrimSpace(r.FormValue("title")),
		Kind:            strings.TrimSpace(r.FormValue("kind")),
		Summary:         strings.TrimSpace(r.FormValue("summary")),
		Content:         strings.TrimSpace(r.FormValue("content")),
		LinkURL:         strings.TrimSpace(r.FormValue("link_url")),
		CreatedBy:       strings.TrimSpace(r.FormValue("created_by")),
		OriginPublicKey: strings.TrimSpace(r.FormValue("origin_public_key")),
		ParentPublicKey: strings.TrimSpace(r.FormValue("parent_public_key")),
		Labels:          parseCSVStrings(r.FormValue("labels")),
		CreatedAt:       time.Now().UTC(),
	}
	payload.UpdatedAt = payload.CreatedAt
	if err := requireTeamAction(store, teamID, payload.CreatedBy, "artifact.create"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := store.AppendArtifactCtx(r.Context(), teamID, payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	targetID := payload.ArtifactID
	if targetID == "" {
		artifact, err := store.LoadArtifactsCtx(r.Context(), teamID, 1)
		if err == nil && len(artifact) > 0 {
			targetID = artifact[0].ArtifactID
		}
	}
	_ = appendTeamHistoryCtx(r.Context(), store, historyActor{
		AgentID:         payload.CreatedBy,
		OriginPublicKey: payload.OriginPublicKey,
		ParentPublicKey: payload.ParentPublicKey,
		Source:          "page",
	}, teamID, "artifact", "create", targetID, "创建 Team Artifact", artifactHistoryMetadata(teamcore.Artifact{}, payload))
	http.Redirect(w, r, "/teams/"+teamID+"/artifacts/"+targetID, http.StatusSeeOther)
}

func handleTeamArtifactUpdate(store *teamcore.Store, teamID, artifactID string, w http.ResponseWriter, r *http.Request) {
	if !teamRequestTrusted(r) {
		http.Error(w, "team artifact update is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	existing, err := store.LoadArtifactCtx(r.Context(), teamID, artifactID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	updated := existing
	updated.ChannelID = strings.TrimSpace(r.FormValue("channel_id"))
	updated.TaskID = strings.TrimSpace(r.FormValue("task_id"))
	updated.Title = strings.TrimSpace(r.FormValue("title"))
	updated.Kind = strings.TrimSpace(r.FormValue("kind"))
	updated.Summary = strings.TrimSpace(r.FormValue("summary"))
	updated.Content = strings.TrimSpace(r.FormValue("content"))
	updated.LinkURL = strings.TrimSpace(r.FormValue("link_url"))
	updated.Labels = parseCSVStrings(r.FormValue("labels"))
	updated.UpdatedAt = time.Now().UTC()
	if err := requireTeamAction(store, teamID, strings.TrimSpace(r.FormValue("actor_agent_id")), "artifact.update"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := store.SaveArtifactCtx(r.Context(), teamID, updated); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = appendTeamHistoryCtx(r.Context(), store, historyActor{
		AgentID:         updated.CreatedBy,
		OriginPublicKey: updated.OriginPublicKey,
		ParentPublicKey: updated.ParentPublicKey,
		Source:          "page",
	}, teamID, "artifact", "update", artifactID, "更新 Team Artifact", artifactHistoryMetadata(existing, updated))
	http.Redirect(w, r, "/teams/"+teamID+"/artifacts/"+artifactID, http.StatusSeeOther)
}

func handleTeamArtifactDelete(store *teamcore.Store, teamID, artifactID string, w http.ResponseWriter, r *http.Request) {
	if !teamRequestTrusted(r) {
		http.Error(w, "team artifact update is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	existing, err := store.LoadArtifactCtx(r.Context(), teamID, artifactID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := requireTeamAction(store, teamID, strings.TrimSpace(r.FormValue("actor_agent_id")), "artifact.delete"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := store.DeleteArtifactCtx(r.Context(), teamID, artifactID); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = appendTeamHistoryCtx(r.Context(), store, historyActor{
		AgentID:         existing.CreatedBy,
		OriginPublicKey: existing.OriginPublicKey,
		ParentPublicKey: existing.ParentPublicKey,
		Source:          "page",
	}, teamID, "artifact", "delete", artifactID, "删除 Team Artifact", map[string]any{
		"diff_summary": "删除产物",
	})
	http.Redirect(w, r, "/teams/"+teamID+"/artifacts", http.StatusSeeOther)
}

func handleAPITeamArtifacts(store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		handleAPITeamArtifactCreate(store, teamID, w, r)
		return
	}
	info, err := store.LoadTeamCtx(r.Context(), teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	limit := clampTeamListLimit(r.URL.Query().Get("limit"), 100, 200)
	artifacts, err := store.LoadArtifactsCtx(r.Context(), teamID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filterKind := strings.TrimSpace(r.URL.Query().Get("kind"))
	filterChannel := normalizeTeamChannel(r.URL.Query().Get("channel"))
	filterTask := strings.TrimSpace(r.URL.Query().Get("task"))
	artifacts = filterArtifacts(artifacts, filterKind, filterChannel, filterTask)
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":          "team-artifacts",
		"team_id":        info.TeamID,
		"limit":          limit,
		"artifact_count": len(artifacts),
		"artifacts":      artifacts,
		"applied_filters": map[string]string{
			"kind":    filterKind,
			"channel": filterChannel,
			"task":    filterTask,
		},
	})
}

func handleAPITeamArtifact(store *teamcore.Store, teamID, artifactID string, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		handleAPITeamArtifactUpdate(store, teamID, artifactID, w, r)
		return
	}
	if r.Method == http.MethodDelete {
		handleAPITeamArtifactDelete(store, teamID, artifactID, w, r)
		return
	}
	artifact, err := store.LoadArtifactCtx(r.Context(), teamID, artifactID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":    "team-artifact",
		"team_id":  teamID,
		"artifact": artifact,
	})
}

func handleAPITeamArtifactCreate(store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	if !teamRequestTrusted(r) {
		http.Error(w, "team artifact update is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	var payload teamcore.Artifact
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	payload.CreatedAt = time.Now().UTC()
	payload.UpdatedAt = payload.CreatedAt
	if err := requireTeamAction(store, teamID, payload.CreatedBy, "artifact.create"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := store.AppendArtifactCtx(r.Context(), teamID, payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	artifact, err := store.LoadArtifactCtx(r.Context(), teamID, payload.ArtifactID)
	if err != nil {
		artifact = payload
	}
	_ = appendTeamHistoryCtx(r.Context(), store, historyActor{
		AgentID:         artifact.CreatedBy,
		OriginPublicKey: artifact.OriginPublicKey,
		ParentPublicKey: artifact.ParentPublicKey,
		Source:          "api",
	}, teamID, "artifact", "create", artifact.ArtifactID, "创建 Team Artifact", artifactHistoryMetadata(teamcore.Artifact{}, artifact))
	newsplugin.WriteJSON(w, http.StatusCreated, map[string]any{
		"scope":    "team-artifact",
		"team_id":  teamID,
		"artifact": artifact,
	})
}

func handleAPITeamArtifactUpdate(store *teamcore.Store, teamID, artifactID string, w http.ResponseWriter, r *http.Request) {
	if !teamRequestTrusted(r) {
		http.Error(w, "team artifact update is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	existing, err := store.LoadArtifactCtx(r.Context(), teamID, artifactID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var payload teamcore.Artifact
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	payload.TeamID = teamID
	payload.ArtifactID = artifactID
	if payload.Title == "" {
		payload.Title = existing.Title
	}
	if payload.ChannelID == "" {
		payload.ChannelID = existing.ChannelID
	}
	if payload.TaskID == "" {
		payload.TaskID = existing.TaskID
	}
	if payload.CreatedBy == "" {
		payload.CreatedBy = existing.CreatedBy
	}
	if payload.OriginPublicKey == "" {
		payload.OriginPublicKey = existing.OriginPublicKey
	}
	if payload.ParentPublicKey == "" {
		payload.ParentPublicKey = existing.ParentPublicKey
	}
	if payload.CreatedAt.IsZero() {
		payload.CreatedAt = existing.CreatedAt
	}
	payload.UpdatedAt = time.Now().UTC()
	if err := requireTeamAction(store, teamID, payload.CreatedBy, "artifact.update"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := store.SaveArtifactCtx(r.Context(), teamID, payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	artifact, err := store.LoadArtifactCtx(r.Context(), teamID, artifactID)
	if err != nil {
		artifact = payload
	}
	_ = appendTeamHistoryCtx(r.Context(), store, historyActor{
		AgentID:         artifact.CreatedBy,
		OriginPublicKey: artifact.OriginPublicKey,
		ParentPublicKey: artifact.ParentPublicKey,
		Source:          "api",
	}, teamID, "artifact", "update", artifactID, "更新 Team Artifact", artifactHistoryMetadata(existing, artifact))
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":    "team-artifact",
		"team_id":  teamID,
		"artifact": artifact,
	})
}

func handleAPITeamArtifactDelete(store *teamcore.Store, teamID, artifactID string, w http.ResponseWriter, r *http.Request) {
	if !teamRequestTrusted(r) {
		http.Error(w, "team artifact update is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	existing, err := store.LoadArtifactCtx(r.Context(), teamID, artifactID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var payload struct {
		ActorAgentID string `json:"actor_agent_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	if err := requireTeamAction(store, teamID, payload.ActorAgentID, "artifact.delete"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := store.DeleteArtifactCtx(r.Context(), teamID, artifactID); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = appendTeamHistoryCtx(r.Context(), store, historyActor{
		AgentID:         existing.CreatedBy,
		OriginPublicKey: existing.OriginPublicKey,
		ParentPublicKey: existing.ParentPublicKey,
		Source:          "api",
	}, teamID, "artifact", "delete", artifactID, "删除 Team Artifact", map[string]any{
		"diff_summary": "删除产物",
	})
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":       "team-artifact",
		"team_id":     teamID,
		"artifact_id": artifactID,
		"deleted":     true,
	})
}
