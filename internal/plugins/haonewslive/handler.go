package haonewslive

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"hao.news/internal/haonews/live"
	newsplugin "hao.news/internal/plugins/haonews"
)

func handleLiveIndex(app *newsplugin.App, store *live.LocalStore, w http.ResponseWriter, r *http.Request) {
	rooms, err := store.ListRooms()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rules, err := app.SubscriptionRules()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pendingRooms, err := buildLivePendingRooms(store, rules)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rooms = filterLiveRoomsByRules(rooms, rules)
	applyPendingCountsToLiveRooms(rooms, pendingRooms)
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := liveIndexPageData{
		Project:    app.ProjectName(),
		Version:    app.VersionString(),
		PageNav:    app.PageNav("/live"),
		NodeStatus: app.NodeStatus(index),
		Now:        time.Now(),
		Rooms:      rooms,
		PendingCount: len(pendingRooms),
		SummaryStats: []newsplugin.SummaryStat{
			{Label: "房间数", Value: formatCount(len(rooms))},
			{Label: "在线房间", Value: formatCount(countActiveRooms(rooms))},
			{Label: "已归档", Value: formatCount(countArchivedRooms(rooms))},
			{Label: "最近更新", Value: latestRoomValue(rooms)},
		},
	}
	if err := app.Templates().ExecuteTemplate(w, "live.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleLivePendingIndex(app *newsplugin.App, store *live.LocalStore, w http.ResponseWriter, r *http.Request) {
	rules, err := app.SubscriptionRules()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rooms, err := buildLivePendingRooms(store, rules)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := livePendingIndexPageData{
		Project:    app.ProjectName(),
		Version:    app.VersionString(),
		PageNav:    app.PageNav("/live"),
		NodeStatus: app.NodeStatus(index),
		Now:        time.Now(),
		Rooms:      rooms,
		SummaryStats: []newsplugin.SummaryStat{
			{Label: "待处理房间", Value: formatCount(len(rooms))},
			{Label: "整房拦截", Value: formatCount(countPendingBlockedRooms(rooms))},
			{Label: "待处理事件", Value: formatCount(countPendingBlockedEvents(rooms))},
			{Label: "最近拦截", Value: latestPendingRoomValue(rooms)},
		},
	}
	if err := app.Templates().ExecuteTemplate(w, "live_pending.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleLiveRoom(app *newsplugin.App, store *live.LocalStore, roomID string, w http.ResponseWriter, r *http.Request) {
	room, err := store.LoadRoom(roomID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such file") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	events, err := store.ReadEvents(roomID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rules, err := app.SubscriptionRules()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !liveRoomInfoAllowed(room, rules) {
		http.NotFound(w, r)
		return
	}
	archive, err := store.LoadArchiveResult(roomID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	showHeartbeats := queryBool(r, "show_heartbeats", false)
	autoRefresh := queryBool(r, "refresh", true)
	filteredEvents := filterLiveEvents(events, showHeartbeats, rules)
	blockedEvents := blockedLiveEvents(events, true, rules)
	taskSummaries := buildTaskSummaries(filteredEvents)
	roomVisibility, _ := classifyLivePublicKeyVisibility(strings.TrimSpace(room.CreatorPubKey), strings.TrimSpace(room.ParentPublicKey), rules)
	data := liveRoomPageData{
		Project:        app.ProjectName(),
		Version:        app.VersionString(),
		PageNav:        app.PageNav("/live"),
		NodeStatus:     app.NodeStatus(index),
		Now:            time.Now(),
		Room:           room,
		RoomVisibility: roomVisibility,
		PendingBlockedEvents: len(blockedEvents),
		Events:         filteredEvents,
		EventViews:     buildEventViews(filteredEvents, rules),
		TaskSummaries:  taskSummaries,
		TaskByStatus:   groupTasksByStatus(taskSummaries),
		TaskByAssignee: groupTasksByAssignee(taskSummaries),
		Roster:         live.BuildRoster(filteredEvents, time.Now().UTC(), 30*time.Second),
		Archive:        archive,
		ShowHeartbeats: showHeartbeats,
		AutoRefresh:    autoRefresh,
	}
	if err := app.Templates().ExecuteTemplate(w, "live_room.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleLivePendingRoom(app *newsplugin.App, store *live.LocalStore, roomID string, w http.ResponseWriter, r *http.Request) {
	room, err := store.LoadRoom(roomID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such file") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	events, err := store.ReadEvents(roomID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rules, err := app.SubscriptionRules()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	showHeartbeats := queryBool(r, "show_heartbeats", false)
	blockedEvents := blockedLiveEvents(events, showHeartbeats, rules)
	roomVisibility, roomAllowed := classifyLivePublicKeyVisibility(strings.TrimSpace(room.CreatorPubKey), strings.TrimSpace(room.ParentPublicKey), rules)
	if roomAllowed && len(blockedEvents) == 0 {
		http.NotFound(w, r)
		return
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := livePendingRoomPageData{
		Project:           app.ProjectName(),
		Version:           app.VersionString(),
		PageNav:           app.PageNav("/live"),
		NodeStatus:        app.NodeStatus(index),
		Now:               time.Now(),
		Room:              room,
		RoomVisibility:    roomVisibility,
		BlockedEvents:     blockedEvents,
		EventViews:        buildEventViews(blockedEvents, rules),
		BlockedEventCount: len(blockedEvents),
		ShowHeartbeats:    showHeartbeats,
	}
	if err := app.Templates().ExecuteTemplate(w, "live_pending_room.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleAPILiveRooms(app *newsplugin.App, store *live.LocalStore, w http.ResponseWriter, r *http.Request) {
	rooms, err := store.ListRooms()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if app != nil {
		if rules, err := app.SubscriptionRules(); err == nil {
			pendingRooms, pendingErr := buildLivePendingRooms(store, rules)
			if pendingErr == nil {
				applyPendingCountsToLiveRooms(rooms, pendingRooms)
			}
			rooms = filterLiveRoomsByRules(rooms, rules)
		}
	}
	if rooms == nil {
		rooms = []live.RoomSummary{}
	}
	newsplugin.WriteJSON(w, http.StatusOK, rooms)
}

func handleAPILivePendingRooms(app *newsplugin.App, store *live.LocalStore, w http.ResponseWriter, r *http.Request) {
	rules := newsplugin.SubscriptionRules{}
	if app != nil {
		rules, _ = app.SubscriptionRules()
	}
	rooms, err := buildLivePendingRooms(store, rules)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope": "live-pending",
		"rooms": rooms,
	})
}

func handleAPILiveRoom(app *newsplugin.App, store *live.LocalStore, roomID string, w http.ResponseWriter, r *http.Request) {
	room, err := store.LoadRoom(roomID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	events, err := store.ReadEvents(roomID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rules := newsplugin.SubscriptionRules{}
	if app != nil {
		rules, _ = app.SubscriptionRules()
	}
	if !liveRoomInfoAllowed(room, rules) {
		http.NotFound(w, r)
		return
	}
	archive, err := store.LoadArchiveResult(roomID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	showHeartbeats := queryBool(r, "show_heartbeats", false)
	filteredEvents := filterLiveEvents(events, showHeartbeats, rules)
	blockedEvents := blockedLiveEvents(events, true, rules)
	taskSummaries := buildTaskSummaries(filteredEvents)
	roomVisibility, _ := classifyLivePublicKeyVisibility(strings.TrimSpace(room.CreatorPubKey), strings.TrimSpace(room.ParentPublicKey), rules)
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"room":             room,
		"room_visibility":  roomVisibility,
		"pending_blocked_events": len(blockedEvents),
		"events":           filteredEvents,
		"event_views":      buildEventViews(filteredEvents, rules),
		"task_summaries":   taskSummaries,
		"task_by_status":   groupTasksByStatus(taskSummaries),
		"task_by_assignee": groupTasksByAssignee(taskSummaries),
		"roster":           live.BuildRoster(filteredEvents, time.Now().UTC(), 30*time.Second),
		"archive":          archive,
		"show_heartbeats":  showHeartbeats,
	})
}

func handleAPILivePendingRoom(app *newsplugin.App, store *live.LocalStore, roomID string, w http.ResponseWriter, r *http.Request) {
	room, err := store.LoadRoom(roomID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	events, err := store.ReadEvents(roomID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rules := newsplugin.SubscriptionRules{}
	if app != nil {
		rules, _ = app.SubscriptionRules()
	}
	showHeartbeats := queryBool(r, "show_heartbeats", false)
	blockedEvents := blockedLiveEvents(events, showHeartbeats, rules)
	roomVisibility, roomAllowed := classifyLivePublicKeyVisibility(strings.TrimSpace(room.CreatorPubKey), strings.TrimSpace(room.ParentPublicKey), rules)
	if roomAllowed && len(blockedEvents) == 0 {
		http.NotFound(w, r)
		return
	}
	newsplugin.WriteJSON(w, http.StatusOK, map[string]any{
		"scope":               "live-pending-room",
		"room":                room,
		"room_visibility":     roomVisibility,
		"blocked_event_count": len(blockedEvents),
		"events":              blockedEvents,
		"event_views":         buildEventViews(blockedEvents, rules),
		"show_heartbeats":     showHeartbeats,
	})
}

func filterLiveEvents(events []live.LiveMessage, showHeartbeats bool, rules newsplugin.SubscriptionRules) []live.LiveMessage {
	filtered := make([]live.LiveMessage, 0, len(events))
	for _, event := range events {
		if !liveEventAllowed(event, rules) {
			continue
		}
		if !showHeartbeats && hidesByDefault(event) {
			continue
		}
		if isMetadataOnlyControlEvent(event) {
			continue
		}
		filtered = append(filtered, event)
	}
	return filtered
}

func blockedLiveEvents(events []live.LiveMessage, showHeartbeats bool, rules newsplugin.SubscriptionRules) []live.LiveMessage {
	filtered := make([]live.LiveMessage, 0, len(events))
	for _, event := range events {
		visibility, allowed := classifyLivePublicKeyVisibility(strings.TrimSpace(event.SenderPubKey), metadataString(event.Payload.Metadata, "parent_public_key"), rules)
		if allowed || visibility == "default" {
			continue
		}
		if !showHeartbeats && hidesByDefault(event) {
			continue
		}
		if isMetadataOnlyControlEvent(event) {
			continue
		}
		filtered = append(filtered, event)
	}
	return filtered
}

func filterLiveRoomsByRules(rooms []live.RoomSummary, rules newsplugin.SubscriptionRules) []live.RoomSummary {
	if len(rooms) == 0 {
		return rooms
	}
	filtered := make([]live.RoomSummary, 0, len(rooms))
	for _, room := range rooms {
		visibility, allowed := classifyLivePublicKeyVisibility(strings.TrimSpace(room.CreatorPubKey), strings.TrimSpace(room.ParentPublicKey), rules)
		if allowed {
			room.LiveVisibility = visibility
			filtered = append(filtered, room)
		}
	}
	return filtered
}

func buildLivePendingRooms(store *live.LocalStore, rules newsplugin.SubscriptionRules) ([]livePendingRoomSummary, error) {
	rooms, err := store.ListRooms()
	if err != nil {
		return nil, err
	}
	pending := make([]livePendingRoomSummary, 0, len(rooms))
	for _, room := range rooms {
		roomVisibility, roomAllowed := classifyLivePublicKeyVisibility(strings.TrimSpace(room.CreatorPubKey), strings.TrimSpace(room.ParentPublicKey), rules)
		events, err := store.ReadEvents(room.RoomID)
		if err != nil {
			return nil, err
		}
		blockedEvents := blockedLiveEvents(events, true, rules)
		if roomAllowed && len(blockedEvents) == 0 {
			continue
		}
		lastBlockedAt := room.LastEventAt
		if len(blockedEvents) > 0 {
			lastBlockedAt = parseLatestBlockedEventTime(blockedEvents, lastBlockedAt)
		}
		reason := roomVisibility
		if roomAllowed {
			reason = "blocked_events"
		}
		pending = append(pending, livePendingRoomSummary{
			RoomID:            room.RoomID,
			Title:             room.Title,
			Creator:           room.Creator,
			CreatedAt:         room.CreatedAt,
			LastEventAt:       lastBlockedAt,
			Channel:           room.Channel,
			Archive:           room.Archive,
			RoomVisibility:    roomVisibility,
			BlockedEventCount: len(blockedEvents),
			BlockedReason:     reason,
			PendingURL:        "/live/pending/" + room.RoomID,
			APIURL:            "/api/live/pending/" + room.RoomID,
		})
	}
	sort.SliceStable(pending, func(i, j int) bool {
		if pending[i].LastEventAt.Equal(pending[j].LastEventAt) {
			return pending[i].RoomID < pending[j].RoomID
		}
		return pending[i].LastEventAt.After(pending[j].LastEventAt)
	})
	return pending, nil
}

func applyPendingCountsToLiveRooms(rooms []live.RoomSummary, pending []livePendingRoomSummary) {
	if len(rooms) == 0 || len(pending) == 0 {
		return
	}
	counts := make(map[string]int, len(pending))
	for _, item := range pending {
		if item.BlockedEventCount <= 0 {
			continue
		}
		counts[item.RoomID] = item.BlockedEventCount
	}
	for idx := range rooms {
		rooms[idx].PendingBlockedEvents = counts[rooms[idx].RoomID]
	}
}

func parseLatestBlockedEventTime(events []live.LiveMessage, fallback time.Time) time.Time {
	latest := fallback
	for _, event := range events {
		ts, err := time.Parse(time.RFC3339, strings.TrimSpace(event.Timestamp))
		if err != nil {
			continue
		}
		if latest.IsZero() || ts.After(latest) {
			latest = ts
		}
	}
	return latest
}

func countPendingBlockedRooms(items []livePendingRoomSummary) int {
	count := 0
	for _, item := range items {
		if item.RoomVisibility != "default" && item.RoomVisibility != "" {
			count++
		}
	}
	return count
}

func countPendingBlockedEvents(items []livePendingRoomSummary) int {
	count := 0
	for _, item := range items {
		count += item.BlockedEventCount
	}
	return count
}

func latestPendingRoomValue(items []livePendingRoomSummary) string {
	if len(items) == 0 {
		return "暂无"
	}
	if !items[0].LastEventAt.IsZero() {
		return items[0].LastEventAt.Local().Format("2006-01-02 15:04 MST")
	}
	if !items[0].CreatedAt.IsZero() {
		return items[0].CreatedAt.Local().Format("2006-01-02 15:04 MST")
	}
	return "暂无"
}

func liveRoomAllowed(room live.RoomSummary, rules newsplugin.SubscriptionRules) bool {
	_, allowed := classifyLivePublicKeyVisibility(strings.TrimSpace(room.CreatorPubKey), strings.TrimSpace(room.ParentPublicKey), rules)
	return allowed
}

func liveRoomInfoAllowed(room live.RoomInfo, rules newsplugin.SubscriptionRules) bool {
	_, allowed := classifyLivePublicKeyVisibility(strings.TrimSpace(room.CreatorPubKey), strings.TrimSpace(room.ParentPublicKey), rules)
	return allowed
}

func liveEventAllowed(event live.LiveMessage, rules newsplugin.SubscriptionRules) bool {
	parentKey := metadataString(event.Payload.Metadata, "parent_public_key")
	_, allowed := classifyLivePublicKeyVisibility(strings.TrimSpace(event.SenderPubKey), parentKey, rules)
	return allowed
}

func normalizedLiveRules(rules newsplugin.SubscriptionRules) newsplugin.SubscriptionRules {
	rules.LiveAllowedOriginKeys = uniqueLiveKeys(rules.LiveAllowedOriginKeys)
	rules.LiveBlockedOriginKeys = uniqueLiveKeys(rules.LiveBlockedOriginKeys)
	rules.LiveAllowedParentKeys = uniqueLiveKeys(rules.LiveAllowedParentKeys)
	rules.LiveBlockedParentKeys = uniqueLiveKeys(rules.LiveBlockedParentKeys)
	return rules
}

func liveRulesEmpty(rules newsplugin.SubscriptionRules) bool {
	return len(rules.LiveAllowedOriginKeys) == 0 &&
		len(rules.LiveBlockedOriginKeys) == 0 &&
		len(rules.LiveAllowedParentKeys) == 0 &&
		len(rules.LiveBlockedParentKeys) == 0
}

func hasLiveAllowRules(rules newsplugin.SubscriptionRules) bool {
	return len(rules.LiveAllowedOriginKeys) > 0 || len(rules.LiveAllowedParentKeys) > 0
}

func matchLivePublicKeyFilters(originKey, parentKey string, rules newsplugin.SubscriptionRules) (blocked bool, allowed bool) {
	if containsFold(rules.LiveBlockedOriginKeys, originKey) {
		return true, false
	}
	if containsFold(rules.LiveBlockedParentKeys, parentKey) {
		return true, false
	}
	if containsFold(rules.LiveAllowedOriginKeys, originKey) {
		return false, true
	}
	if containsFold(rules.LiveAllowedParentKeys, parentKey) {
		return false, true
	}
	return false, false
}

func classifyLivePublicKeyVisibility(originKey, parentKey string, rules newsplugin.SubscriptionRules) (string, bool) {
	rules = normalizedLiveRules(rules)
	if liveRulesEmpty(rules) {
		return "default", true
	}
	if containsFold(rules.LiveBlockedOriginKeys, originKey) {
		return "blocked_origin", false
	}
	if containsFold(rules.LiveBlockedParentKeys, parentKey) {
		return "blocked_parent", false
	}
	if containsFold(rules.LiveAllowedOriginKeys, originKey) {
		return "allowed_origin", true
	}
	if containsFold(rules.LiveAllowedParentKeys, parentKey) {
		return "allowed_parent", true
	}
	if hasLiveAllowRules(rules) {
		return "blocked_default", false
	}
	return "default", true
}

func uniqueLiveKeys(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.ToLower(strings.TrimSpace(item))
		if len(item) != 64 {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func containsFold(items []string, needle string) bool {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return false
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), needle) {
			return true
		}
	}
	return false
}

func metadataString(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func hidesByDefault(event live.LiveMessage) bool {
	switch strings.TrimSpace(event.Type) {
	case live.TypeHeartbeat, live.TypeArchiveNotice:
		return true
	default:
		return false
	}
}

func queryBool(r *http.Request, key string, defaultValue bool) bool {
	if r == nil {
		return defaultValue
	}
	raw := strings.TrimSpace(strings.ToLower(r.URL.Query().Get(key)))
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultValue
	}
}

func formatCount(value int) string {
	return strconv.Itoa(value)
}

func latestRoomValue(rooms []live.RoomSummary) string {
	if len(rooms) == 0 {
		return "暂无"
	}
	if !rooms[0].LastEventAt.IsZero() {
		return rooms[0].LastEventAt.Local().Format("2006-01-02 15:04 MST")
	}
	if !rooms[0].CreatedAt.IsZero() {
		return rooms[0].CreatedAt.Local().Format("2006-01-02 15:04 MST")
	}
	return "暂无"
}

func countArchivedRooms(rooms []live.RoomSummary) int {
	count := 0
	for _, room := range rooms {
		if room.Archive != nil {
			count++
		}
	}
	return count
}

func countActiveRooms(rooms []live.RoomSummary) int {
	count := 0
	for _, room := range rooms {
		if room.Active {
			count++
		}
	}
	return count
}

func buildEventViews(events []live.LiveMessage, rules newsplugin.SubscriptionRules) []liveEventView {
	views := make([]liveEventView, 0, len(events))
	for idx := len(events) - 1; idx >= 0; idx-- {
		event := events[idx]
		visibility, _ := classifyLivePublicKeyVisibility(strings.TrimSpace(event.SenderPubKey), metadataString(event.Payload.Metadata, "parent_public_key"), rules)
		view := liveEventView{
			Type:       event.Type,
			Timestamp:  event.Timestamp,
			Sender:     event.Sender,
			Visibility: visibility,
			Heading:    eventHeading(event),
			Fields:     metadataFields(event.Payload.Metadata),
		}
		if task := buildTaskUpdateView(event.Payload.Metadata); task != nil {
			view.Task = task
			view.Note = "任务更新"
		} else if len(view.Fields) > 0 {
			view.Note = "附带结构化元数据"
		}
		views = append(views, view)
	}
	return views
}

func isMetadataOnlyControlEvent(event live.LiveMessage) bool {
	if strings.TrimSpace(event.Payload.Content) != "" {
		return false
	}
	if buildTaskUpdateView(event.Payload.Metadata) != nil {
		return false
	}
	return len(metadataFields(event.Payload.Metadata)) > 0
}

func buildTaskSummaries(events []live.LiveMessage) []liveTaskSummaryView {
	index := make(map[string]*liveTaskSummaryView)
	order := make([]string, 0)
	for _, event := range events {
		task := buildTaskUpdateView(event.Payload.Metadata)
		if task == nil || strings.TrimSpace(task.TaskID) == "" {
			continue
		}
		item, ok := index[task.TaskID]
		if !ok {
			item = &liveTaskSummaryView{TaskID: task.TaskID}
			index[task.TaskID] = item
			order = append(order, task.TaskID)
		}
		item.UpdateCount++
		item.Status = firstNonEmptyString(task.Status, item.Status)
		item.Description = firstNonEmptyString(task.Description, item.Description)
		item.AssignedTo = firstNonEmptyString(task.AssignedTo, item.AssignedTo)
		item.Progress = firstNonEmptyString(task.Progress, item.Progress)
		item.LastSender = firstNonEmptyString(event.Sender, item.LastSender)
		item.LastUpdatedAt = firstNonEmptyString(event.Timestamp, item.LastUpdatedAt)
	}
	summaries := make([]liveTaskSummaryView, 0, len(order))
	for _, key := range order {
		summaries = append(summaries, *index[key])
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].LastUpdatedAt > summaries[j].LastUpdatedAt
	})
	return summaries
}

func groupTasksByStatus(tasks []liveTaskSummaryView) []liveTaskGroupView {
	return groupTasks(tasks, func(task liveTaskSummaryView) string {
		return firstNonEmptyString(task.Status, "未标记状态")
	})
}

func groupTasksByAssignee(tasks []liveTaskSummaryView) []liveTaskGroupView {
	return groupTasks(tasks, func(task liveTaskSummaryView) string {
		return firstNonEmptyString(task.AssignedTo, "未分配")
	})
}

func groupTasks(tasks []liveTaskSummaryView, fn func(liveTaskSummaryView) string) []liveTaskGroupView {
	counts := map[string]int{}
	for _, task := range tasks {
		key := strings.TrimSpace(fn(task))
		if key == "" {
			continue
		}
		counts[key]++
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	groups := make([]liveTaskGroupView, 0, len(keys))
	for _, key := range keys {
		groups = append(groups, liveTaskGroupView{Key: key, Count: counts[key]})
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Count == groups[j].Count {
			return groups[i].Key < groups[j].Key
		}
		return groups[i].Count > groups[j].Count
	})
	return groups
}

func eventHeading(event live.LiveMessage) string {
	content := strings.TrimSpace(event.Payload.Content)
	if content != "" {
		return content
	}
	switch event.Type {
	case live.TypeJoin:
		return "加入房间"
	case live.TypeLeave:
		return "离开房间"
	case live.TypeHeartbeat:
		return "在线心跳"
	case live.TypeTaskUpdate:
		return "任务状态更新"
	case live.TypeArchiveNotice:
		return "房间归档通知"
	default:
		return "控制事件"
	}
}

func buildTaskUpdateView(metadata map[string]any) *liveTaskUpdateView {
	if len(metadata) == 0 {
		return nil
	}
	taskID := metadataString(metadata, "task_id")
	status := metadataString(metadata, "status")
	description := metadataString(metadata, "description")
	assignedTo := metadataString(metadata, "assigned_to")
	progress := metadataProgress(metadata["progress"])
	if taskID == "" && status == "" && description == "" && assignedTo == "" && progress == "" {
		return nil
	}
	return &liveTaskUpdateView{
		TaskID:      taskID,
		Status:      status,
		Description: description,
		AssignedTo:  assignedTo,
		Progress:    progress,
	}
}

func metadataFields(metadata map[string]any) []liveFieldView {
	if len(metadata) == 0 {
		return nil
	}
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fields := make([]liveFieldView, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(fmt.Sprint(metadata[key]))
		if value == "" || value == "<nil>" {
			continue
		}
		fields = append(fields, liveFieldView{Key: key, Value: value})
	}
	return fields
}

func metadataProgress(value any) string {
	if value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	if strings.HasSuffix(text, "%") {
		return text
	}
	return text + "%"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
