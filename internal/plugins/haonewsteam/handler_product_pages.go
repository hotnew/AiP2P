package haonewsteam

import (
	"net/http"
	"strings"
	"time"

	teamcore "hao.news/internal/haonews/team"
	newsplugin "hao.news/internal/plugins/haonews"
)

func handleTeamWebhookPage(app *newsplugin.App, store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	info, err := store.LoadTeamCtx(r.Context(), teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	webhooks, err := store.LoadWebhookConfigsCtx(r.Context(), teamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	status, err := store.LoadWebhookDeliveryStatusCtx(r.Context(), teamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	deliveries, err := store.LoadRecentDeliveriesCtx(r.Context(), teamID, 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := teamWebhookPageData{
		Project:          app.ProjectName(),
		Version:          app.VersionString(),
		PageNav:          app.PageNav("/teams"),
		NodeStatus:       app.NodeStatus(index),
		Now:              time.Now(),
		Team:             info,
		Webhooks:         webhooks,
		WebhookStatus:    status,
		RecentDeliveries: deliveries,
		ReplayNotice:     stringsTrimSpace(r.URL.Query().Get("replayed")),
	}
	if err := app.Templates().ExecuteTemplate(w, "team_webhook.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleTeamA2APage(app *newsplugin.App, store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	info, err := store.LoadTeamCtx(r.Context(), teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	agents, err := store.ListAgentCardsCtx(r.Context(), teamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tasks, err := store.LoadTasksCtx(r.Context(), teamID, 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := teamA2APageData{
		Project:    app.ProjectName(),
		Version:    app.VersionString(),
		PageNav:    app.PageNav("/teams"),
		NodeStatus: app.NodeStatus(index),
		Now:        time.Now(),
		Team:       info,
		Agents:     agents,
		Tasks:      tasks,
		Endpoints: []teamA2AEndpointInfo{
			{Method: "GET", Path: "/.well-known/agent.json", Description: "Agent 公告卡片"},
			{Method: "GET", Path: "/a2a/teams/" + teamID + "/tasks", Description: "A2A 任务列表"},
			{Method: "GET", Path: "/a2a/teams/" + teamID + "/tasks/{taskID}", Description: "A2A 任务详情"},
			{Method: "POST", Path: "/a2a/teams/" + teamID + "/message:send", Description: "A2A 发送消息"},
			{Method: "GET", Path: "/a2a/teams/" + teamID + "/message:stream", Description: "A2A 流式事件"},
			{Method: "POST", Path: "/a2a/teams/" + teamID + "/tasks/{taskID}:cancel", Description: "A2A 取消任务"},
		},
		SummaryStats: []newsplugin.SummaryStat{
			{Label: "Agent Cards", Value: formatTeamCount(len(agents))},
			{Label: "近期任务", Value: formatTeamCount(len(tasks))},
			{Label: "A2A 端点", Value: formatTeamCount(6)},
		},
	}
	if err := app.Templates().ExecuteTemplate(w, "team_a2a.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func stringsTrimSpace(value string) string { return strings.TrimSpace(value) }
