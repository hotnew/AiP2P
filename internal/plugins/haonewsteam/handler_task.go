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

func handleTeamTasks(app *newsplugin.App, store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	info, err := store.LoadTeamCtx(r.Context(), teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	tasks, err := store.LoadTasksCtx(r.Context(), teamID, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	artifacts, err := store.LoadArtifactsCtx(r.Context(), teamID, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	channels, err := store.ListChannelsCtx(r.Context(), teamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	history, err := store.LoadHistoryCtx(r.Context(), teamID, 200)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filterStatus := strings.TrimSpace(r.URL.Query().Get("status"))
	filterAssignee := strings.TrimSpace(r.URL.Query().Get("assignee"))
	filterLabel := strings.TrimSpace(r.URL.Query().Get("label"))
	filterChannel := normalizeTeamChannel(r.URL.Query().Get("channel"))
	statuses := taskStatuses(tasks)
	assignees := taskAssignees(tasks)
	labels := taskLabels(tasks)
	tasks = filterTasks(tasks, filterStatus, filterAssignee, filterLabel, filterChannel)
	data := teamTasksPageData{
		Project:        app.ProjectName(),
		Version:        app.VersionString(),
		PageNav:        app.PageNav("/teams"),
		NodeStatus:     app.NodeStatus(index),
		Now:            time.Now(),
		Team:           info,
		Tasks:          tasks,
		ArtifactCounts: artifactCountsByTask(artifacts),
		HistoryCounts:  historyCountsByTask(history),
		FilterStatus:   filterStatus,
		FilterAssignee: filterAssignee,
		FilterLabel:    filterLabel,
		FilterChannel:  filterChannel,
		AppliedFilters: appliedTeamFilters(
			labeledTeamFilter("状态", filterStatus),
			labeledTeamFilter("负责者", filterAssignee),
			labeledTeamFilter("标签", filterLabel),
			labeledTeamFilter("频道", filterChannel),
		),
		Statuses:  statuses,
		Assignees: assignees,
		Labels:    labels,
		Channels:  channels,
		SummaryStats: []newsplugin.SummaryStat{
			{Label: "任务", Value: formatTeamCount(len(tasks))},
			{Label: "进行中", Value: formatTeamCount(countTasksByStatus(tasks, "doing"))},
			{Label: "已完成", Value: formatTeamCount(countTasksByStatus(tasks, "done"))},
		},
	}
	if err := app.Templates().ExecuteTemplate(w, "team_tasks.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleTeamTask(app *newsplugin.App, store *teamcore.Store, teamID, taskID string, w http.ResponseWriter, r *http.Request) {
	info, err := store.LoadTeamCtx(r.Context(), teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	task, err := store.LoadTaskCtx(r.Context(), teamID, taskID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	tasks, err := store.LoadTasksCtx(r.Context(), teamID, 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	messages, err := store.LoadTaskMessagesCtx(r.Context(), teamID, taskID, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	artifacts, err := store.LoadArtifactsCtx(r.Context(), teamID, 100)
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
	var relatedChannel *teamcore.ChannelSummary
	if strings.TrimSpace(task.ChannelID) != "" {
		for _, channel := range channels {
			if normalizeTeamChannel(channel.ChannelID) == normalizeTeamChannel(task.ChannelID) {
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
	data := teamTaskPageData{
		Project:            app.ProjectName(),
		Version:            app.VersionString(),
		PageNav:            app.PageNav("/teams"),
		NodeStatus:         app.NodeStatus(index),
		Now:                time.Now(),
		Team:               info,
		Task:               task,
		Tasks:              tasks,
		Channels:           channels,
		Messages:           messages,
		Artifacts:          relatedArtifacts(artifacts, taskID, 20),
		RelatedChannel:     relatedChannel,
		RelatedHistory:     taskHistory(history, taskID, 10),
		DefaultCommentType: "comment",
		DefaultChannelID:   preferredTaskCommentChannel(task, channels),
		SummaryStats: []newsplugin.SummaryStat{
			{Label: "状态", Value: task.Status},
			{Label: "优先级", Value: blankDash(task.Priority)},
			{Label: "评论", Value: formatTeamCount(len(messages))},
			{Label: "产物", Value: formatTeamCount(countArtifactsByTask(artifacts, taskID))},
		},
	}
	if err := app.Templates().ExecuteTemplate(w, "team_task.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleTeamTaskCreate(store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	if !teamRequestTrusted(r) {
		http.Error(w, "team task update is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	payload := teamcore.Task{
		TaskID:          strings.TrimSpace(r.FormValue("task_id")),
		ChannelID:       normalizeTeamChannel(r.FormValue("channel_id")),
		Title:           strings.TrimSpace(r.FormValue("title")),
		Description:     strings.TrimSpace(r.FormValue("description")),
		CreatedBy:       strings.TrimSpace(r.FormValue("created_by")),
		Assignees:       parseCSVStrings(r.FormValue("assignees")),
		Status:          strings.TrimSpace(r.FormValue("status")),
		Priority:        strings.TrimSpace(r.FormValue("priority")),
		Labels:          parseCSVStrings(r.FormValue("labels")),
		OriginPublicKey: strings.TrimSpace(r.FormValue("origin_public_key")),
		ParentPublicKey: strings.TrimSpace(r.FormValue("parent_public_key")),
		CreatedAt:       time.Now().UTC(),
	}
	payload.UpdatedAt = payload.CreatedAt
	if err := requireTeamAction(store, teamID, payload.CreatedBy, "task.create"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := store.AppendTaskCtx(r.Context(), teamID, payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	targetID := payload.TaskID
	if targetID == "" {
		tasks, err := store.LoadTasksCtx(r.Context(), teamID, 1)
		if err == nil && len(tasks) > 0 {
			targetID = tasks[0].TaskID
		}
	}
	_ = appendTeamHistoryCtx(r.Context(), store, historyActor{
		AgentID:         payload.CreatedBy,
		OriginPublicKey: payload.OriginPublicKey,
		ParentPublicKey: payload.ParentPublicKey,
		Source:          "page",
	}, teamID, "task", "create", targetID, "创建 Team Task", taskHistoryMetadata(teamcore.Task{}, payload))
	http.Redirect(w, r, "/teams/"+teamID+"/tasks/"+targetID, http.StatusSeeOther)
}

func handleTeamTaskStatus(store *teamcore.Store, teamID, taskID string, w http.ResponseWriter, r *http.Request) {
	if !teamRequestTrusted(r) {
		http.Error(w, "team task update is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	existing, err := store.LoadTaskCtx(r.Context(), teamID, taskID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	updated := existing
	updated.Status = strings.TrimSpace(r.FormValue("status"))
	updated.UpdatedAt = time.Now().UTC()
	if err := requireTeamAction(store, teamID, strings.TrimSpace(r.FormValue("actor_agent_id")), "task.transition"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := store.SaveTaskCtx(r.Context(), teamID, updated); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	after, err := store.LoadTaskCtx(r.Context(), teamID, taskID)
	if err != nil {
		after = updated
	}
	_ = appendTeamHistoryCtx(r.Context(), store, historyActor{
		AgentID:         after.CreatedBy,
		OriginPublicKey: after.OriginPublicKey,
		ParentPublicKey: after.ParentPublicKey,
		Source:          "page",
	}, teamID, "task", "status", taskID, "更新 Team Task 状态", taskHistoryMetadata(existing, after))
	http.Redirect(w, r, "/teams/"+teamID+"/tasks/"+taskID, http.StatusSeeOther)
}

func handleTeamTaskUpdate(store *teamcore.Store, teamID, taskID string, w http.ResponseWriter, r *http.Request) {
	if !teamRequestTrusted(r) {
		http.Error(w, "team task update is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	existing, err := store.LoadTaskCtx(r.Context(), teamID, taskID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	updated := existing
	updated.Title = strings.TrimSpace(r.FormValue("title"))
	updated.ChannelID = normalizeTeamChannel(r.FormValue("channel_id"))
	updated.Description = strings.TrimSpace(r.FormValue("description"))
	updated.Assignees = parseCSVStrings(r.FormValue("assignees"))
	updated.Status = strings.TrimSpace(r.FormValue("status"))
	updated.Priority = strings.TrimSpace(r.FormValue("priority"))
	updated.Labels = parseCSVStrings(r.FormValue("labels"))
	updated.UpdatedAt = time.Now().UTC()
	if err := requireTeamAction(store, teamID, strings.TrimSpace(r.FormValue("actor_agent_id")), "task.update"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := store.SaveTaskCtx(r.Context(), teamID, updated); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = appendTeamHistoryCtx(r.Context(), store, historyActor{
		AgentID:         updated.CreatedBy,
		OriginPublicKey: updated.OriginPublicKey,
		ParentPublicKey: updated.ParentPublicKey,
		Source:          "page",
	}, teamID, "task", "update", taskID, "更新 Team Task", taskHistoryMetadata(existing, updated))
	http.Redirect(w, r, "/teams/"+teamID+"/tasks/"+taskID, http.StatusSeeOther)
}

func handleTeamTaskDelete(store *teamcore.Store, teamID, taskID string, w http.ResponseWriter, r *http.Request) {
	if !teamRequestTrusted(r) {
		http.Error(w, "team task update is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	existing, err := store.LoadTaskCtx(r.Context(), teamID, taskID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := requireTeamAction(store, teamID, strings.TrimSpace(r.FormValue("actor_agent_id")), "task.delete"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := store.DeleteTaskCtx(r.Context(), teamID, taskID); err != nil {
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
	}, teamID, "task", "delete", taskID, "删除 Team Task", map[string]any{
		"diff_summary": "删除任务",
	})
	http.Redirect(w, r, "/teams/"+teamID+"/tasks", http.StatusSeeOther)
}

func handleTeamTaskCommentCreate(store *teamcore.Store, teamID, taskID string, w http.ResponseWriter, r *http.Request) {
	if !teamRequestTrusted(r) {
		http.Error(w, "team task comment is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	task, err := store.LoadTaskCtx(r.Context(), teamID, taskID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	channelID := normalizeTeamChannel(r.FormValue("channel_id"))
	if channelID == "" {
		channelID = preferredTaskCommentChannel(task, nil)
	}
	structuredData, err := parseOptionalStructuredData(r.FormValue("structured_data"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if structuredData == nil {
		structuredData = make(map[string]any, 2)
	}
	structuredData["task_id"] = taskID
	if strings.TrimSpace(task.ContextID) != "" {
		structuredData["context_id"] = task.ContextID
	}
	msg := teamcore.Message{
		TeamID:          teamID,
		ChannelID:       channelID,
		ContextID:       strings.TrimSpace(task.ContextID),
		AuthorAgentID:   strings.TrimSpace(r.FormValue("author_agent_id")),
		OriginPublicKey: strings.TrimSpace(r.FormValue("origin_public_key")),
		ParentPublicKey: strings.TrimSpace(r.FormValue("parent_public_key")),
		MessageType:     strings.TrimSpace(r.FormValue("message_type")),
		Content:         strings.TrimSpace(r.FormValue("content")),
		StructuredData:  structuredData,
		CreatedAt:       time.Now().UTC(),
	}
	if err := requireTeamAction(store, teamID, msg.AuthorAgentID, "message.send"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := store.AppendMessageCtx(r.Context(), teamID, msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = appendTeamHistoryCtx(r.Context(), store, historyActor{
		AgentID:         msg.AuthorAgentID,
		OriginPublicKey: msg.OriginPublicKey,
		ParentPublicKey: msg.ParentPublicKey,
		Source:          "page",
	}, teamID, "task", "comment", taskID, "追加 Team Task 评论", map[string]any{
		"task_id":       taskID,
		"channel_id":    channelID,
		"message_type":  blankDash(msg.MessageType),
		"author_agent":  msg.AuthorAgentID,
		"diff_summary":  "任务评论已追加到 Team Channel",
		"message_scope": "team-message",
	})
	http.Redirect(w, r, "/teams/"+teamID+"/tasks/"+taskID, http.StatusSeeOther)
}

func handleAPITeamTasks(store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		handleAPITeamTaskCreate(store, teamID, w, r)
		return
	}
	info, err := store.LoadTeamCtx(r.Context(), teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	limit := clampTeamListLimit(r.URL.Query().Get("limit"), 100, 200)
	tasks, err := store.LoadTasksCtx(r.Context(), teamID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filterStatus := strings.TrimSpace(r.URL.Query().Get("status"))
	filterAssignee := strings.TrimSpace(r.URL.Query().Get("assignee"))
	filterLabel := strings.TrimSpace(r.URL.Query().Get("label"))
	filterChannel := normalizeTeamChannel(r.URL.Query().Get("channel"))
	tasks = filterTasks(tasks, filterStatus, filterAssignee, filterLabel, filterChannel)
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":      "team-tasks",
		"team_id":    info.TeamID,
		"limit":      limit,
		"task_count": len(tasks),
		"tasks":      tasks,
		"applied_filters": map[string]string{
			"status":   filterStatus,
			"assignee": filterAssignee,
			"label":    filterLabel,
			"channel":  filterChannel,
		},
	})
}

func handleAPITeamTask(store *teamcore.Store, teamID, taskID string, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && strings.TrimSpace(r.URL.Query().Get("action")) == "comment" {
		handleAPITeamTaskCommentCreate(store, teamID, taskID, w, r)
		return
	}
	if r.Method == http.MethodPut {
		handleAPITeamTaskUpdate(store, teamID, taskID, w, r)
		return
	}
	if r.Method == http.MethodDelete {
		handleAPITeamTaskDelete(store, teamID, taskID, w, r)
		return
	}
	info, err := store.LoadTeamCtx(r.Context(), teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	task, err := store.LoadTaskCtx(r.Context(), teamID, taskID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	messages, err := store.LoadTaskMessagesCtx(r.Context(), teamID, taskID, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":         "team-task",
		"team_id":       info.TeamID,
		"task_id":       task.TaskID,
		"task":          task,
		"message_count": len(messages),
		"messages":      messages,
	})
}

func handleAPITeamTaskCommentCreate(store *teamcore.Store, teamID, taskID string, w http.ResponseWriter, r *http.Request) {
	if !teamRequestTrusted(r) {
		http.Error(w, "team task comment is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	task, err := store.LoadTaskCtx(r.Context(), teamID, taskID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var payload teamcore.Message
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	payload.TeamID = teamID
	payload.ChannelID = normalizeTeamChannel(payload.ChannelID)
	if payload.ChannelID == "" {
		payload.ChannelID = "main"
	}
	if payload.StructuredData == nil {
		payload.StructuredData = make(map[string]any, 2)
	}
	payload.StructuredData["task_id"] = taskID
	if strings.TrimSpace(task.ContextID) != "" {
		payload.ContextID = task.ContextID
		payload.StructuredData["context_id"] = task.ContextID
	}
	payload.CreatedAt = time.Now().UTC()
	if err := requireTeamAction(store, teamID, payload.AuthorAgentID, "message.send"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := store.AppendMessageCtx(r.Context(), teamID, payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = appendTeamHistoryCtx(r.Context(), store, historyActor{
		AgentID:         payload.AuthorAgentID,
		OriginPublicKey: payload.OriginPublicKey,
		ParentPublicKey: payload.ParentPublicKey,
		Source:          "api",
	}, teamID, "task", "comment", taskID, "追加 Team Task 评论", map[string]any{
		"task_id":       taskID,
		"channel_id":    payload.ChannelID,
		"message_type":  blankDash(payload.MessageType),
		"author_agent":  payload.AuthorAgentID,
		"diff_summary":  "任务评论已追加到 Team Channel",
		"message_scope": "team-message",
	})
	newsplugin.WriteJSON(w, http.StatusCreated, map[string]any{
		"scope":   "team-task-comment",
		"team_id": teamID,
		"task_id": taskID,
		"message": payload,
	})
}

func handleAPITeamContext(store *teamcore.Store, teamID, contextID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := store.LoadTeamCtx(r.Context(), teamID); err != nil {
		http.NotFound(w, r)
		return
	}
	limit := clampTeamListLimit(r.URL.Query().Get("limit"), 100, 200)
	tasks, err := store.LoadTasksByContextCtx(r.Context(), teamID, contextID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	messages, err := store.LoadMessagesByContextCtx(r.Context(), teamID, contextID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":         "team-context",
		"team_id":       teamID,
		"context_id":    strings.TrimSpace(contextID),
		"limit":         limit,
		"task_count":    len(tasks),
		"message_count": len(messages),
		"tasks":         tasks,
		"messages":      messages,
	})
}

func handleAPITeamTaskCreate(store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	if !teamRequestTrusted(r) {
		http.Error(w, "team task update is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	var payload teamcore.Task
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	payload.ChannelID = normalizeTeamChannel(payload.ChannelID)
	payload.CreatedAt = time.Now().UTC()
	payload.UpdatedAt = payload.CreatedAt
	if err := requireTeamAction(store, teamID, payload.CreatedBy, "task.create"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := store.AppendTaskCtx(r.Context(), teamID, payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	task, err := store.LoadTaskCtx(r.Context(), teamID, payload.TaskID)
	if err != nil {
		task = payload
	}
	_ = appendTeamHistoryCtx(r.Context(), store, historyActor{
		AgentID:         task.CreatedBy,
		OriginPublicKey: task.OriginPublicKey,
		ParentPublicKey: task.ParentPublicKey,
		Source:          "api",
	}, teamID, "task", "create", task.TaskID, "创建 Team Task", taskHistoryMetadata(teamcore.Task{}, task))
	newsplugin.WriteJSON(w, http.StatusCreated, map[string]any{
		"scope":   "team-task",
		"team_id": teamID,
		"task":    task,
	})
}

func handleAPITeamTaskUpdate(store *teamcore.Store, teamID, taskID string, w http.ResponseWriter, r *http.Request) {
	if !teamRequestTrusted(r) {
		http.Error(w, "team task update is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	existing, err := store.LoadTaskCtx(r.Context(), teamID, taskID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var payload teamcore.Task
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	payload.TeamID = teamID
	payload.TaskID = taskID
	if strings.TrimSpace(payload.ChannelID) == "" {
		payload.ChannelID = existing.ChannelID
	}
	if payload.Title == "" {
		payload.Title = existing.Title
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
	if err := requireTeamAction(store, teamID, payload.CreatedBy, "task.update"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := store.SaveTaskCtx(r.Context(), teamID, payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	task, err := store.LoadTaskCtx(r.Context(), teamID, taskID)
	if err != nil {
		task = payload
	}
	_ = appendTeamHistoryCtx(r.Context(), store, historyActor{
		AgentID:         task.CreatedBy,
		OriginPublicKey: task.OriginPublicKey,
		ParentPublicKey: task.ParentPublicKey,
		Source:          "api",
	}, teamID, "task", "update", taskID, "更新 Team Task", taskHistoryMetadata(existing, task))
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":   "team-task",
		"team_id": teamID,
		"task":    task,
	})
}

func handleAPITeamTaskStatus(store *teamcore.Store, teamID, taskID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !teamRequestTrusted(r) {
		http.Error(w, "team task update is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	existing, err := store.LoadTaskCtx(r.Context(), teamID, taskID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var payload struct {
		Status       string `json:"status"`
		ActorAgentID string `json:"actor_agent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	updated := existing
	updated.Status = payload.Status
	updated.UpdatedAt = time.Now().UTC()
	if err := requireTeamAction(store, teamID, payload.ActorAgentID, "task.transition"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := store.SaveTaskCtx(r.Context(), teamID, updated); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	task, err := store.LoadTaskCtx(r.Context(), teamID, taskID)
	if err != nil {
		task = updated
	}
	_ = appendTeamHistoryCtx(r.Context(), store, historyActor{
		AgentID:         task.CreatedBy,
		OriginPublicKey: task.OriginPublicKey,
		ParentPublicKey: task.ParentPublicKey,
		Source:          "api",
	}, teamID, "task", "status", taskID, "更新 Team Task 状态", taskHistoryMetadata(existing, task))
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":   "team-task",
		"team_id": teamID,
		"task":    task,
	})
}

func handleAPITeamTaskDelete(store *teamcore.Store, teamID, taskID string, w http.ResponseWriter, r *http.Request) {
	if !teamRequestTrusted(r) {
		http.Error(w, "team task update is limited to local or LAN requests", http.StatusForbidden)
		return
	}
	existing, err := store.LoadTaskCtx(r.Context(), teamID, taskID)
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
	if err := requireTeamAction(store, teamID, payload.ActorAgentID, "task.delete"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := store.DeleteTaskCtx(r.Context(), teamID, taskID); err != nil {
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
	}, teamID, "task", "delete", taskID, "删除 Team Task", map[string]any{
		"diff_summary": "删除任务",
	})
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":   "team-task",
		"team_id": teamID,
		"task_id": taskID,
		"deleted": true,
	})
}
