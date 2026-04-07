package aip2parchive

import (
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	newsplugin "aip2p/internal/plugins/aip2p"
)

func newHandler(app *newsplugin.App, staticFS fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/archive", func(w http.ResponseWriter, r *http.Request) {
		handleArchiveIndex(app, w, r)
	})
	mux.HandleFunc("/archive/topics", func(w http.ResponseWriter, r *http.Request) {
		handleArchiveIndex(app, w, r)
	})
	mux.HandleFunc("/archive/", func(w http.ResponseWriter, r *http.Request) {
		handleArchiveSubtree(app, w, r)
	})
	mux.HandleFunc("/api/history/list", func(w http.ResponseWriter, r *http.Request) {
		handleAPIHistoryList(app, w, r)
	})
	mux.HandleFunc("/api/history/manifest", func(w http.ResponseWriter, r *http.Request) {
		handleAPIHistoryList(app, w, r)
	})
	mux.HandleFunc("/api/archive/topics/list", func(w http.ResponseWriter, r *http.Request) {
		handleAPIHistoryList(app, w, r)
	})
	mux.HandleFunc("/api/archive/topics/manifest", func(w http.ResponseWriter, r *http.Request) {
		handleAPIHistoryList(app, w, r)
	})
	mux.Handle("/static/", newsplugin.NoStoreStaticHandler(staticFS))
	return mux
}

func handleArchiveIndex(app *newsplugin.App, w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/archive" && r.URL.Path != "/archive/topics" {
		http.NotFound(w, r)
		return
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rules, err := app.SubscriptionRules()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	days := newsplugin.BuildArchiveDays(index)
	data := newsplugin.ArchiveIndexPageData{
		Project:       app.ProjectName(),
		Version:       app.VersionString(),
		PageNav:       app.PageNav("/archive/topics"),
		Now:           time.Now(),
		BasePath:      "/archive/topics",
		Section:       "topics",
		Days:          days,
		SummaryStats:  newsplugin.BuildArchiveSummaryStats(days, len(index.Bundles)),
		Subscriptions: rules,
		NodeStatus:    app.NodeStatus(index),
	}
	if err := app.Templates().ExecuteTemplate(w, "archive_index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleArchiveSubtree(app *newsplugin.App, w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(r.URL.Path, "/archive/topics/messages/"):
		handleArchiveMessage(app, w, r)
	case strings.HasPrefix(r.URL.Path, "/archive/topics/raw/"):
		handleArchiveRaw(app, w, r)
	case strings.HasPrefix(r.URL.Path, "/archive/topics/"):
		handleArchiveDay(app, w, r)
	case strings.HasPrefix(r.URL.Path, "/archive/messages/"):
		handleArchiveMessage(app, w, r)
	case strings.HasPrefix(r.URL.Path, "/archive/raw/"):
		handleArchiveRaw(app, w, r)
	default:
		handleArchiveDay(app, w, r)
	}
}

func handleArchiveDay(app *newsplugin.App, w http.ResponseWriter, r *http.Request) {
	basePath := "/archive/topics"
	day := strings.TrimPrefix(r.URL.Path, "/archive/topics/")
	if day == r.URL.Path {
		basePath = "/archive"
		day = strings.TrimPrefix(r.URL.Path, "/archive/")
	}
	if day == "" || day == r.URL.Path || strings.Contains(day, "/") {
		http.NotFound(w, r)
		return
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rules, err := app.SubscriptionRules()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	days := newsplugin.BuildArchiveDays(index)
	if !newsplugin.HasArchiveDay(days, day) {
		http.NotFound(w, r)
		return
	}
	entries := newsplugin.BuildArchiveEntries(index, day)
	data := newsplugin.ArchiveDayPageData{
		Project:       app.ProjectName(),
		Version:       app.VersionString(),
		PageNav:       app.PageNav("/archive/topics"),
		Now:           time.Now(),
		BasePath:      basePath,
		Section:       "topics",
		Day:           day,
		Days:          newsplugin.MarkArchiveDayActive(days, day),
		Entries:       entries,
		SummaryStats:  newsplugin.BuildArchiveDayStats(entries),
		Subscriptions: rules,
		NodeStatus:    app.NodeStatus(index),
	}
	if err := app.Templates().ExecuteTemplate(w, "archive_day.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleArchiveMessage(app *newsplugin.App, w http.ResponseWriter, r *http.Request) {
	basePath := "/archive/topics"
	infoHash := strings.TrimPrefix(r.URL.Path, "/archive/topics/messages/")
	if infoHash == r.URL.Path {
		basePath = "/archive"
		infoHash = strings.TrimPrefix(r.URL.Path, "/archive/messages/")
	}
	if infoHash == "" || infoHash == r.URL.Path {
		http.NotFound(w, r)
		return
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entry, err := newsplugin.EnsureArchiveEntry(&index, app.ArchiveRoot(), infoHash)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	content, err := os.ReadFile(entry.ArchiveMD)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := newsplugin.ArchiveMessagePageData{
		Project:    app.ProjectName(),
		Version:    app.VersionString(),
		PageNav:    app.PageNav("/archive/topics"),
		Now:        time.Now(),
		BasePath:   basePath,
		Section:    "topics",
		Entry:      entry,
		Content:    string(content),
		Thread:     entry.ThreadURL,
		RawURL:     basePath + "/raw/" + entry.InfoHash,
		DayURL:     basePath + "/" + entry.Day,
		Archive:    entry.ArchiveMD,
		NodeStatus: app.NodeStatus(index),
	}
	if err := app.Templates().ExecuteTemplate(w, "archive_message.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleArchiveRaw(app *newsplugin.App, w http.ResponseWriter, r *http.Request) {
	infoHash := strings.TrimPrefix(r.URL.Path, "/archive/topics/raw/")
	if infoHash == r.URL.Path {
		infoHash = strings.TrimPrefix(r.URL.Path, "/archive/raw/")
	}
	if infoHash == "" || infoHash == r.URL.Path {
		http.NotFound(w, r)
		return
	}
	index, err := app.Index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entry, err := newsplugin.EnsureArchiveEntry(&index, app.ArchiveRoot(), infoHash)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	http.ServeFile(w, r, entry.ArchiveMD)
}

func handleAPIHistoryList(app *newsplugin.App, w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/history/list" &&
		r.URL.Path != "/api/history/manifest" &&
		r.URL.Path != "/api/archive/topics/list" &&
		r.URL.Path != "/api/archive/topics/manifest" {
		http.NotFound(w, r)
		return
	}
	payload, err := app.HistoryListPayload(r.URL.Query().Get("cursor"), parseHistoryPageSize(r.URL.Query().Get("page_size")))
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newsplugin.WriteJSON(w, http.StatusOK, payload)
}

func parseHistoryPageSize(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}
