package haonewsteam

import (
	"time"

	teamcore "hao.news/internal/haonews/team"
	newsplugin "hao.news/internal/plugins/haonews"
)

type teamIndexPageData struct {
	Project      string
	Version      string
	PageNav      []newsplugin.NavItem
	NodeStatus   newsplugin.NodeStatus
	Now          time.Time
	Teams        []teamcore.Summary
	SummaryStats []newsplugin.SummaryStat
}

type teamPageData struct {
	Project      string
	Version      string
	PageNav      []newsplugin.NavItem
	NodeStatus   newsplugin.NodeStatus
	Now          time.Time
	Team         teamcore.Info
	Members      []teamcore.Member
	Messages     []teamcore.Message
	Tasks        []teamcore.Task
	SummaryStats []newsplugin.SummaryStat
}
