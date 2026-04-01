package haonewsteam

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	teamcore "hao.news/internal/haonews/team"
	newsplugin "hao.news/internal/plugins/haonews"
)

func handleTeamIndex(app *newsplugin.App, store *teamcore.Store, w http.ResponseWriter, r *http.Request) {
	teams, err := store.ListTeams()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	memberCount := 0
	for _, team := range teams {
		memberCount += team.MemberCount
	}
	data := teamIndexPageData{
		Project:    app.ProjectName(),
		Version:    app.VersionString(),
		PageNav:    app.PageNav("/teams"),
		NodeStatus: app.NodeStatus(index),
		Now:        time.Now(),
		Teams:      teams,
		SummaryStats: []newsplugin.SummaryStat{
			{Label: "团队数", Value: formatTeamCount(len(teams))},
			{Label: "成员总数", Value: formatTeamCount(memberCount)},
			{Label: "最近更新", Value: latestTeamValue(teams)},
		},
	}
	if err := app.Templates().ExecuteTemplate(w, "team.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleTeam(app *newsplugin.App, store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	info, err := store.LoadTeam(teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	members, err := store.LoadMembers(teamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	messages, err := store.LoadMessages(teamID, "main", 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tasks, err := store.LoadTasks(teamID, 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := teamPageData{
		Project:    app.ProjectName(),
		Version:    app.VersionString(),
		PageNav:    app.PageNav("/teams"),
		NodeStatus: app.NodeStatus(index),
		Now:        time.Now(),
		Team:       info,
		Members:    members,
		Messages:   messages,
		Tasks:      tasks,
		SummaryStats: []newsplugin.SummaryStat{
			{Label: "成员", Value: formatTeamCount(len(members))},
			{Label: "任务", Value: formatTeamCount(len(tasks))},
			{Label: "消息", Value: formatTeamCount(len(messages))},
		},
	}
	if err := app.Templates().ExecuteTemplate(w, "team_detail.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleAPITeamIndex(store *teamcore.Store, w http.ResponseWriter, _ *http.Request) {
	teams, err := store.ListTeams()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope": "team-index",
		"count": len(teams),
		"teams": teams,
	})
}

func handleAPITeam(store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	info, err := store.LoadTeam(teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	members, err := store.LoadMembers(teamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":        "team-detail",
		"team":         info,
		"member_count": len(members),
		"members":      members,
	})
}

func handleAPITeamMembers(store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	info, err := store.LoadTeam(teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	members, err := store.LoadMembers(teamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":        "team-members",
		"team_id":      info.TeamID,
		"member_count": len(members),
		"members":      members,
	})
}

func handleAPITeamMessages(store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	info, err := store.LoadTeam(teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	channelID := strings.TrimSpace(r.URL.Query().Get("channel"))
	messages, err := store.LoadMessages(teamID, channelID, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if channelID == "" {
		channelID = "main"
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":         "team-messages",
		"team_id":       info.TeamID,
		"channel_id":    channelID,
		"message_count": len(messages),
		"messages":      messages,
	})
}

func handleAPITeamTasks(store *teamcore.Store, teamID string, w http.ResponseWriter, r *http.Request) {
	info, err := store.LoadTeam(teamID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	tasks, err := store.LoadTasks(teamID, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":      "team-tasks",
		"team_id":    info.TeamID,
		"task_count": len(tasks),
		"tasks":      tasks,
	})
}

func formatTeamCount(value int) string {
	return strconv.Itoa(value)
}

func latestTeamValue(teams []teamcore.Summary) string {
	for _, team := range teams {
		if !team.UpdatedAt.IsZero() {
			return formatTeamTime(team.UpdatedAt)
		}
		if !team.CreatedAt.IsZero() {
			return formatTeamTime(team.CreatedAt)
		}
	}
	return "暂无"
}

func formatTeamTime(value time.Time) string {
	if value.IsZero() {
		return "暂无"
	}
	return value.In(time.Local).Format("2006-01-02 15:04")
}
