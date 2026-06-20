package explorer

import (
	"bytes"
	"embed"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/russross/blackfriday/v2"
	"go.uber.org/zap"
)

//go:embed embedded-docs/*.md
var docsFS embed.FS

// docNav defines sidebar order and titles.
var docNav = []struct {
	Slug  string
	Title string
}{
	{"index", "Overview"},
	{"getting-started", "Getting started"},
	{"examples", "Examples"},
	{"cli-reference", "CLI reference"},
	{"nsmith", "nsmith compiler"},
	{"rpc-reference", "RPC reference"},
	{"explorer", "Block explorer"},
	{"forking", "Forking"},
	{"fork-troubleshooting", "Fork troubleshooting"},
	{"impersonation", "Impersonation"},
	{"state-management", "State management"},
	{"tracing", "Tracing"},
	{"architecture", "Architecture"},
	{"anvil-comparison", "Anvil comparison"},
	{"development", "Development"},
	{"upstream-sync", "Upstream sync"},
}

var slugPattern = regexp.MustCompile(`^[a-z0-9-]+$`)
var mdLinkPattern = regexp.MustCompile(`href="([^"]+)\.md([^"]*)"`)

const docsPageTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Title}} · Nanvil Docs</title>
  <script>(function(){try{var k='nanvil-theme',t=localStorage.getItem(k);if(t!=='light'&&t!=='dark')t=matchMedia('(prefers-color-scheme:light)').matches?'light':'dark';document.documentElement.setAttribute('data-theme',t);}catch(e){document.documentElement.setAttribute('data-theme','dark');}})();</script>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,400;0,9..40,500;0,9..40,600;0,9..40,700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
  <link rel="icon" type="image/png" href="/favicon.png">
  <link rel="stylesheet" href="/theme.css">
  <link rel="stylesheet" href="/docs.css">
</head>
<body>
  <div class="bg-mesh" aria-hidden="true"></div>
  <header class="docs-topbar">
    <a href="/" class="docs-brand">
      <img src="/nanvil-logo.png" alt="" class="logo-img" width="36" height="36">
      <span>
        <strong>Nanvil</strong>
        <small>Documentation</small>
      </span>
    </a>
    <nav class="docs-topnav">
      <a href="/">Explorer</a>
      <a href="/docs/" class="active">Docs</a>
      <div class="theme-switcher" role="group" aria-label="Theme">
        <button type="button" class="theme-option" data-theme="light" aria-pressed="false" title="Light theme">
          <span class="theme-option-icon" aria-hidden="true">☀</span>
          <span class="theme-option-label">Light</span>
        </button>
        <button type="button" class="theme-option" data-theme="dark" aria-pressed="false" title="Dark theme">
          <span class="theme-option-icon" aria-hidden="true">☾</span>
          <span class="theme-option-label">Dark</span>
        </button>
      </div>
    </nav>
  </header>
  <div class="docs-layout">
    <aside class="docs-sidebar">
      <p class="docs-sidebar-label">Contents</p>
      <ul>
        {{range .Nav}}
        <li><a href="/docs/{{.Slug}}"{{if eq .Slug $.Current}} class="active"{{end}}>{{.Title}}</a></li>
        {{end}}
      </ul>
    </aside>
    <main class="docs-content">
      <article class="markdown-body">
        {{.Body}}
      </article>
    </main>
  </div>
  <footer class="docs-footer">
    <span>Embedded from <code>docs/</code> at build time</span>
    <span>·</span>
    <a href="/">Back to explorer</a>
  </footer>
  <script src="/theme.js"></script>
</body>
</html>`

type docsPageData struct {
	Title   string
	Body    template.HTML
	Nav     []docNavItem
	Current string
}

type docNavItem struct {
	Slug  string
	Title string
}

func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	p := strings.TrimPrefix(r.URL.Path, "/docs")
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		p = "index"
	}
	if strings.HasSuffix(p, ".md") {
		p = strings.TrimSuffix(p, ".md")
	}
	if !slugPattern.MatchString(p) {
		http.NotFound(w, r)
		return
	}

	content, title, err := loadDocPage(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		s.log.Warn("docs render failed", zap.String("page", p), zap.Error(err))
		http.Error(w, "failed to render docs", http.StatusInternalServerError)
		return
	}

	nav := make([]docNavItem, len(docNav))
	for i, item := range docNav {
		nav[i] = docNavItem{Slug: item.Slug, Title: item.Title}
	}

	tmpl, err := template.New("docs").Parse(docsPageTmpl)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, docsPageData{
		Title:   title,
		Body:    template.HTML(content),
		Nav:     nav,
		Current: p,
	}); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

func loadDocPage(slug string) (html string, title string, err error) {
	name := path.Join("embedded-docs", slug+".md")
	raw, err := docsFS.ReadFile(name)
	if err != nil {
		return "", "", err
	}

	title = pageTitleFromMarkdown(string(raw), slug)
	body := blackfriday.Run(raw, blackfriday.WithExtensions(
		blackfriday.CommonExtensions|blackfriday.AutoHeadingIDs|blackfriday.HardLineBreak,
	))
	html = string(body)
	html = mdLinkPattern.ReplaceAllString(html, `href="/docs/$1$2"`)
	return html, title, nil
}

func pageTitleFromMarkdown(md, slug string) string {
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	for _, item := range docNav {
		if item.Slug == slug {
			return item.Title
		}
	}
	return slug
}
