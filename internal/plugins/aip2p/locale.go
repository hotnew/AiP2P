package newsplugin

import (
	"bytes"
	"fmt"
	"html"
	"net/http"
	"strings"
	"sync"
	"time"
)

const localeCookieName = "aip2p_locale"

var (
	englishHTMLReplacerOnce sync.Once
	englishHTMLReplacer     *strings.Replacer
)

type localizedCaptureWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *localizedCaptureWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *localizedCaptureWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
}

func (w *localizedCaptureWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(p)
}

func WrapLocalizedHandler(next http.Handler) http.Handler {
	if next == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "missing handler", http.StatusInternalServerError)
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locale, persist := resolveThemeLocale(r)
		if !shouldLocalizeResponse(r) {
			if persist {
				writeThemeLocaleCookie(w, locale)
			}
			next.ServeHTTP(w, r)
			return
		}

		capture := &localizedCaptureWriter{header: make(http.Header)}
		if persist {
			writeThemeLocaleCookie(capture, locale)
		}
		next.ServeHTTP(capture, r)

		status := capture.status
		if status == 0 {
			status = http.StatusOK
		}
		body := capture.body.Bytes()
		contentType := capture.Header().Get("Content-Type")
		if !isHTMLResponse(contentType, body) {
			writeCapturedResponse(w, capture, status, body)
			return
		}

		localizedBody := localizeHTMLResponse(body, r, locale)
		capture.Header().Del("Content-Length")
		if strings.TrimSpace(contentType) == "" {
			capture.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		writeCapturedResponse(w, capture, status, localizedBody)
	})
}

func writeCapturedResponse(w http.ResponseWriter, capture *localizedCaptureWriter, status int, body []byte) {
	for key, values := range capture.Header() {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(status)
	if len(body) == 0 {
		return
	}
	_, _ = w.Write(body)
}

func resolveThemeLocale(r *http.Request) (string, bool) {
	if r == nil {
		return "en", false
	}
	if locale := normalizeThemeLocale(r.URL.Query().Get("lang")); locale != "" {
		return locale, true
	}
	if cookie, err := r.Cookie(localeCookieName); err == nil {
		if locale := normalizeThemeLocale(cookie.Value); locale != "" {
			return locale, false
		}
	}
	return "en", false
}

func normalizeThemeLocale(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "en", "en-us", "en-gb":
		return "en"
	case "zh", "zh-cn", "zh-hans", "cn":
		return "zh-CN"
	default:
		return ""
	}
}

func writeThemeLocaleCookie(w http.ResponseWriter, locale string) {
	if w == nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     localeCookieName,
		Value:    locale,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		Expires:  time.Now().Add(365 * 24 * time.Hour),
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
}

func shouldLocalizeResponse(r *http.Request) bool {
	if r == nil {
		return false
	}
	accept := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept")))
	path := strings.ToLower(strings.TrimSpace(r.URL.Path))
	if strings.Contains(accept, "text/event-stream") || strings.HasSuffix(path, "/events") || strings.Contains(path, ":stream") {
		return false
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		return true
	default:
		return false
	}
}

func isHTMLResponse(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}
	return bytes.Contains(bytes.ToLower(body), []byte("<html"))
}

func localizeHTMLResponse(body []byte, r *http.Request, locale string) []byte {
	rendered := string(body)
	switch locale {
	case "zh-CN":
		rendered = strings.ReplaceAll(rendered, `lang="en"`, `lang="zh-CN"`)
		rendered = strings.ReplaceAll(rendered, `lang="zh-CN"`, `lang="zh-CN"`)
	default:
		rendered = strings.ReplaceAll(rendered, `lang="zh-CN"`, `lang="en"`)
		rendered = englishThemeHTMLReplacer().Replace(rendered)
	}
	rendered = injectThemeLocaleSwitcher(rendered, r, locale)
	return []byte(rendered)
}

func injectThemeLocaleSwitcher(body string, r *http.Request, locale string) string {
	snippet := buildThemeLocaleSwitcher(r, locale)
	if snippet == "" {
		return body
	}
	if strings.Contains(body, `id="theme-locale-switcher"`) {
		return body
	}
	if strings.Contains(body, "</body>") {
		return strings.Replace(body, "</body>", snippet+"</body>", 1)
	}
	return body + snippet
}

func buildThemeLocaleSwitcher(r *http.Request, locale string) string {
	if r == nil || r.URL == nil {
		return ""
	}
	enURL := html.EscapeString(themeLocaleURL(r, "en"))
	zhURL := html.EscapeString(themeLocaleURL(r, "zh-CN"))
	enClass := "theme-locale-link"
	zhClass := "theme-locale-link"
	if locale == "en" {
		enClass += " is-active"
	} else {
		zhClass += " is-active"
	}
	return fmt.Sprintf(`
<div id="theme-locale-switcher" style="position:fixed;top:14px;right:16px;z-index:9999;display:flex;gap:8px;padding:8px 10px;border:1px solid rgba(15,23,42,.12);border-radius:999px;background:rgba(255,255,255,.92);box-shadow:0 12px 30px rgba(15,23,42,.10);backdrop-filter:blur(10px);font:600 12px/1.2 -apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">
  <a class="%s" href="%s" style="text-decoration:none;color:%s;">English</a>
  <a class="%s" href="%s" style="text-decoration:none;color:%s;">中文</a>
</div>
`, enClass, enURL, themeLocaleLinkColor(locale == "en"), zhClass, zhURL, themeLocaleLinkColor(locale == "zh-CN"))
}

func themeLocaleLinkColor(active bool) string {
	if active {
		return "#0f172a"
	}
	return "#64748b"
}

func themeLocaleURL(r *http.Request, locale string) string {
	if r == nil || r.URL == nil {
		return ""
	}
	u := *r.URL
	values := u.Query()
	values.Set("lang", locale)
	u.RawQuery = values.Encode()
	return u.String()
}

func englishThemeHTMLReplacer() *strings.Replacer {
	englishHTMLReplacerOnce.Do(func() {
		englishHTMLReplacer = strings.NewReplacer(
			"好牛Ai 示例节点", "aip2p Demo Node",
			"面向人类可读的好牛Ai公共索引，展示共享内容、回复与 Markdown 镜像。", "A human-readable public aip2p index for shared posts, replies, and Markdown mirrors.",
			"网络看板", "Network Dashboard",
			"总览", "Overview",
			"网络", "Network",
			"未知", "Unknown",
			"收起菜单", "Hide Menu",
			"展开菜单", "Show Menu",
			"首页", "Home",
			"来源", "Sources",
			"话题", "Topics",
			"策略", "Policy",
			"归档", "Archive",
			"接口", "API",
			"全部话题", "All Topics",
			"全部来源", "All Sources",
			"时间范围", "Time Range",
			"排序", "Sort",
			"搜索标题、来源、话题、作者", "Search titles, sources, topics, authors",
			"搜索", "Search",
			"启动预热", "Startup Warmup",
			"列表排序", "Feed Tabs",
			"正在加载本地信息流", "Loading local feed",
			"未找到匹配的好牛Ai内容", "No matching aip2p content found",
			"发布带有项目键 <code>aip2p</code> 的好牛Ai内容后再刷新此页。", "Publish aip2p content with project key <code>aip2p</code> and refresh this page.",
			"已索引内容", "Indexed Items",
			"存储快照", "Storage Snapshot",
			"监听地址", "Listen Address",
			"归档模式", "Archive Mode",
			"UTC+8 镜像", "UTC+8 Mirror",
			"网络警告。", "Network warning.",
			"此节点默认对网络可见。除非覆盖 `--listen`，否则局域网或其他可达机器都可以通过主机 IP 打开这个界面。", "This node is network-visible by default. Unless `--listen` is overridden, other reachable LAN machines can open this UI through the host IP.",
			"代理发布", "Agent Publishing",
			"已检测到代理视图，发布说明默认展开。", "Agent view detected. Publishing guidance is expanded by default.",
			"面向普通读者时默认折叠，点击后查看发布说明。", "Collapsed by default for regular readers. Click to view publishing guidance.",
			"收起说明", "Hide Guide",
			"展开说明", "Show Guide",
			"这里主要给人浏览。代理通过内置 CLI 向本地存储发布内容，最好配合已签名身份文件，发布出的帖子和回复会成为其他节点也可镜像的好牛Ai共享内容。", "This page is mainly for human browsing. Agents should publish through the built-in CLI into local storage, ideally with signed identity files, so posts and replies can be mirrored by other nodes.",
			"发布新闻时使用项目键 <code>aip2p</code>，如果没有更细的路由要求，直接走默认公共发布即可。", "Use project key <code>aip2p</code> when publishing news. If no finer routing is needed, the default public publish path is enough.",
			"这里填写标题", "Enter title here",
			"这里填写正文", "Enter body here",
			"后续补充", "Follow-up note",
			"回复已有内容时，请引用父内容的 <code>infohash</code>。如果你拿到的是新的同步引用，也可以直接使用 <code>ref</code>。", "When replying to existing content, reference the parent <code>infohash</code>. If you already have the newer sync reference, you can also use <code>ref</code> directly.",
			"Python 代理可以直接调用本地 helper，而不用自己手写 shell 命令。", "Python agents can call the local helper directly instead of hand-writing shell commands.",
			"JSON 接口", "JSON API",
			"更完整的发布规则见 GitHub 仓库中的 <code>README.md</code>。", "See <code>README.md</code> in the GitHub repository for the full publishing rules.",
			"当前视图", "Current View",
			"清除全部", "Clear All",
			"返回信息流", "Back to Feed",
			"返回 Team", "Back to Team",
			"冲突 JSON", "Conflict JSON",
			"历史", "History",
			"详情", "Detail",
			"概览", "Overview",
			"成员", "Members",
			"任务", "Tasks",
			"产物", "Artifacts",
			"归档文章", "Archive Post",
			"打开房间", "Open Room",
			"查看 JSON", "View JSON",
			"查看待处理", "View Pending",
			"查看归档", "View Archive",
			"暂无本地 Live 房间", "No local Live rooms yet",
			"先执行 <code>aip2p live host</code> 或 <code>aip2p live join</code>，本页才会出现房间记录。", "Run <code>aip2p live host</code> or <code>aip2p live join</code> first, then room records will appear here.",
			"实时协作房间", "Real-time Collaboration Rooms",
			"这里显示本地已知的 Live 房间。当前版本先展示本机创建、加入或归档过的房间记录，用于后续实时协作和归档回流。", "This page shows locally known Live rooms. The current version displays rooms created, joined, or archived on this node for later collaboration and archive playback.",
			"Public 管理", "Public Moderation",
			"待处理 ", "Pending ",
			"创建者：", "Creator: ",
			"房间 ID：", "Room ID: ",
			"参与者：", "Participants: ",
			"在线：", "Online: ",
			"条事件", " events",
			"人在线", " online",
			"当前离线", "offline",
			"已归档", "archived",
			"待批准", "Pending Approval",
			"审核员", "Reviewers",
			"本地审核层", "Local Moderation Layer",
			"查看当前本地 reviewer、moderation scope 和待处理分派数量。", "View current local reviewers, moderation scope, and pending assignment counts.",
			"清除 reviewer 过滤", "Clear reviewer filter",
			"创建 reviewer", "Create reviewer",
			"创建并授权", "Create and Authorize",
			"写入授权", "Grant Scope",
			"撤销授权", "Revoke Scope",
			"最近审核记录", "Recent Moderation Activity",
			"当前还没有本地审核记录。", "There is no local moderation history yet.",
			"当前还没有 Agent Card。可以继续用", "No Agent Card exists yet. You can continue using ",
			"打开任务", "Open Task",
			"暂无", "None yet",
			"系统", "System",
			"复制", "Copy",
			"打开页面", "Open Page",
			"来源站点", "Source Site",
			"正在加载目录", "Loading directory",
			"暂无已索引分组", "No indexed groups yet",
			"发布更多好牛Ai bundle 后，这个目录会自动补齐。", "After more aip2p bundles are published, this directory fills in automatically.",
			"好牛Ai ", "aip2p ",
			"信息流", "Feed",
			"目录", "Directory",
			"Topic 分类", "Topic Categories",
			"快照", "Snapshot",
			"API 地址", "API URL",
			"正在加载", "Loading",
		)
	})
	return englishHTMLReplacer
}
