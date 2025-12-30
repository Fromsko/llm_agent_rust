package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// CratesIOSearch crates.io 搜索工具
type CratesIOSearch struct {
	client *http.Client
}

func NewCratesIOSearch() *CratesIOSearch {
	return &CratesIOSearch{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *CratesIOSearch) Name() string { return "crates_search" }
func (t *CratesIOSearch) Description() string {
	return "搜索 crates.io 上的 Rust 库，获取库名、版本、描述、下载量等信息"
}
func (t *CratesIOSearch) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":    map[string]any{"type": "string", "description": "搜索关键词"},
			"per_page": map[string]any{"type": "integer", "description": "每页结果数（默认5）"},
		},
		"required": []string{"query"},
	}
}

func (t *CratesIOSearch) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		Query   string `json:"query"`
		PerPage int    `json:"per_page"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	if args.PerPage <= 0 {
		args.PerPage = 5
	}

	apiURL := fmt.Sprintf("https://crates.io/api/v1/crates?q=%s&per_page=%d",
		url.QueryEscape(args.Query), args.PerPage)

	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	req.Header.Set("User-Agent", "rust-agent/1.0")

	resp, err := t.client.Do(req)
	if err != nil {
		return FormatOutput(map[string]any{"success": false, "error": err.Error()}), nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Crates []struct {
			Name          string `json:"name"`
			Description   string `json:"description"`
			MaxVersion    string `json:"max_version"`
			Downloads     int    `json:"downloads"`
			Documentation string `json:"documentation"`
			Repository    string `json:"repository"`
		} `json:"crates"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return FormatOutput(map[string]any{"success": false, "error": "解析响应失败"}), nil
	}

	var crates []map[string]any
	for _, c := range result.Crates {
		crates = append(crates, map[string]any{
			"name":          c.Name,
			"version":       c.MaxVersion,
			"description":   c.Description,
			"downloads":     c.Downloads,
			"documentation": c.Documentation,
			"repository":    c.Repository,
			"cargo_toml":    fmt.Sprintf(`%s = "%s"`, c.Name, c.MaxVersion),
		})
	}

	return FormatOutput(map[string]any{
		"success": true,
		"query":   args.Query,
		"count":   len(crates),
		"crates":  crates,
	}), nil
}

// CratesIOInfo 获取 crate 详细信息
type CratesIOInfo struct {
	client *http.Client
}

func NewCratesIOInfo() *CratesIOInfo {
	return &CratesIOInfo{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *CratesIOInfo) Name() string { return "crates_info" }
func (t *CratesIOInfo) Description() string {
	return "获取指定 crate 的详细信息，包括版本、features、依赖等"
}
func (t *CratesIOInfo) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"crate_name": map[string]any{"type": "string", "description": "crate 名称"},
		},
		"required": []string{"crate_name"},
	}
}

func (t *CratesIOInfo) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		CrateName string `json:"crate_name"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf("https://crates.io/api/v1/crates/%s", url.PathEscape(args.CrateName))

	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	req.Header.Set("User-Agent", "rust-agent/1.0")

	resp, err := t.client.Do(req)
	if err != nil {
		return FormatOutput(map[string]any{"success": false, "error": err.Error()}), nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Crate struct {
			Name          string   `json:"name"`
			Description   string   `json:"description"`
			MaxVersion    string   `json:"max_version"`
			Downloads     int      `json:"downloads"`
			Documentation string   `json:"documentation"`
			Repository    string   `json:"repository"`
			Homepage      string   `json:"homepage"`
			Keywords      []string `json:"keywords"`
			Categories    []string `json:"categories"`
		} `json:"crate"`
		Versions []struct {
			Num      string              `json:"num"`
			Features map[string][]string `json:"features"`
		} `json:"versions"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return FormatOutput(map[string]any{"success": false, "error": "crate 不存在或解析失败"}), nil
	}

	c := result.Crate
	info := map[string]any{
		"name":          c.Name,
		"version":       c.MaxVersion,
		"description":   c.Description,
		"downloads":     c.Downloads,
		"documentation": c.Documentation,
		"repository":    c.Repository,
		"homepage":      c.Homepage,
		"keywords":      c.Keywords,
		"cargo_toml":    fmt.Sprintf(`%s = "%s"`, c.Name, c.MaxVersion),
		"docs_rs_url":   fmt.Sprintf("https://docs.rs/%s/%s", c.Name, c.MaxVersion),
	}

	// 获取最新版本的 features
	if len(result.Versions) > 0 {
		info["features"] = result.Versions[0].Features
	}

	return FormatOutput(map[string]any{
		"success": true,
		"crate":   info,
	}), nil
}

// DocsRSFetch 获取 docs.rs 文档
type DocsRSFetch struct {
	client *http.Client
}

func NewDocsRSFetch() *DocsRSFetch {
	return &DocsRSFetch{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *DocsRSFetch) Name() string { return "docs_rs_fetch" }
func (t *DocsRSFetch) Description() string {
	return "获取 docs.rs 上的 Rust 库文档，提取 API 用法和示例代码"
}
func (t *DocsRSFetch) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"crate_name": map[string]any{"type": "string", "description": "crate 名称"},
			"module":     map[string]any{"type": "string", "description": "模块路径（可选，如 client::Client）"},
		},
		"required": []string{"crate_name"},
	}
}

func (t *DocsRSFetch) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		CrateName string `json:"crate_name"`
		Module    string `json:"module"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	// 构建 docs.rs URL
	crateName := strings.ReplaceAll(args.CrateName, "-", "_")
	docsURL := fmt.Sprintf("https://docs.rs/%s/latest/%s/", args.CrateName, crateName)
	if args.Module != "" {
		docsURL += strings.ReplaceAll(args.Module, "::", "/") + "/"
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", docsURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := t.client.Do(req)
	if err != nil {
		return FormatOutput(map[string]any{"success": false, "error": err.Error()}), nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// 提取关键信息
	doc := extractDocsContent(html)

	return FormatOutput(map[string]any{
		"success": true,
		"url":     docsURL,
		"crate":   args.CrateName,
		"module":  args.Module,
		"content": doc,
	}), nil
}

// extractDocsContent 从 docs.rs HTML 中提取文档内容
func extractDocsContent(html string) map[string]any {
	doc := make(map[string]any)

	// 提取描述
	descRe := regexp.MustCompile(`<div class="docblock"[^>]*>([\s\S]*?)</div>`)
	if matches := descRe.FindStringSubmatch(html); len(matches) > 1 {
		doc["description"] = cleanHTML(matches[1])
	}

	// 提取代码示例
	codeRe := regexp.MustCompile(`<pre class="rust[^"]*"><code>([\s\S]*?)</code></pre>`)
	var examples []string
	for _, match := range codeRe.FindAllStringSubmatch(html, 5) {
		if len(match) > 1 {
			code := cleanHTML(match[1])
			if len(code) > 50 && len(code) < 2000 {
				examples = append(examples, code)
			}
		}
	}
	if len(examples) > 0 {
		doc["examples"] = examples
	}

	// 提取函数/方法签名
	fnRe := regexp.MustCompile(`<h4[^>]*class="code-header"[^>]*>([\s\S]*?)</h4>`)
	var signatures []string
	for _, match := range fnRe.FindAllStringSubmatch(html, 20) {
		if len(match) > 1 {
			sig := cleanHTML(match[1])
			if strings.Contains(sig, "fn ") || strings.Contains(sig, "pub ") {
				signatures = append(signatures, sig)
			}
		}
	}
	if len(signatures) > 0 {
		doc["signatures"] = signatures
	}

	// 提取 struct/enum 定义
	structRe := regexp.MustCompile(`<pre class="rust item-decl"><code>([\s\S]*?)</code></pre>`)
	if matches := structRe.FindStringSubmatch(html); len(matches) > 1 {
		doc["definition"] = cleanHTML(matches[1])
	}

	return doc
}

// cleanHTML 清理 HTML 标签
func cleanHTML(html string) string {
	// 移除 HTML 标签
	tagRe := regexp.MustCompile(`<[^>]+>`)
	text := tagRe.ReplaceAllString(html, "")

	// 解码 HTML 实体
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	// 清理多余空白
	spaceRe := regexp.MustCompile(`\s+`)
	text = spaceRe.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// WebSearch 网页搜索工具（使用 DuckDuckGo）
type WebSearch struct {
	client *http.Client
}

func NewWebSearch() *WebSearch {
	return &WebSearch{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *WebSearch) Name() string { return "web_search" }
func (t *WebSearch) Description() string {
	return "搜索网页获取信息，用于查找 Rust 库用法、API 文档、错误解决方案等"
}
func (t *WebSearch) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string", "description": "搜索关键词"},
		},
		"required": []string{"query"},
	}
}

func (t *WebSearch) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	// 使用 DuckDuckGo HTML 搜索
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(args.Query))

	req, _ := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return FormatOutput(map[string]any{"success": false, "error": err.Error()}), nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// 提取搜索结果
	results := extractSearchResults(html)

	return FormatOutput(map[string]any{
		"success": true,
		"query":   args.Query,
		"count":   len(results),
		"results": results,
	}), nil
}

// extractSearchResults 从 DuckDuckGo HTML 提取搜索结果
func extractSearchResults(html string) []map[string]string {
	var results []map[string]string

	// 匹配搜索结果
	resultRe := regexp.MustCompile(`<a[^>]*class="result__a"[^>]*href="([^"]*)"[^>]*>([^<]*)</a>`)
	snippetRe := regexp.MustCompile(`<a[^>]*class="result__snippet"[^>]*>([^<]*)</a>`)

	titles := resultRe.FindAllStringSubmatch(html, 10)
	snippets := snippetRe.FindAllStringSubmatch(html, 10)

	for i, match := range titles {
		if len(match) > 2 {
			result := map[string]string{
				"url":   match[1],
				"title": cleanHTML(match[2]),
			}
			if i < len(snippets) && len(snippets[i]) > 1 {
				result["snippet"] = cleanHTML(snippets[i][1])
			}
			results = append(results, result)
		}
	}

	return results
}

// WebFetch 获取网页内容
type WebFetch struct {
	client *http.Client
}

func NewWebFetch() *WebFetch {
	return &WebFetch{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *WebFetch) Name() string { return "web_fetch" }
func (t *WebFetch) Description() string {
	return "获取指定 URL 的网页内容，提取文本和代码"
}
func (t *WebFetch) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{"type": "string", "description": "网页 URL"},
		},
		"required": []string{"url"},
	}
}

func (t *WebFetch) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", args.URL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return FormatOutput(map[string]any{"success": false, "error": err.Error()}), nil
	}
	defer resp.Body.Close()

	// 限制读取大小
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 100*1024))
	html := string(body)

	// 提取文本内容
	content := extractTextContent(html)

	// 提取代码块
	codes := extractCodeBlocks(html)

	return FormatOutput(map[string]any{
		"success": true,
		"url":     args.URL,
		"content": content,
		"codes":   codes,
	}), nil
}

// extractTextContent 提取网页文本内容
func extractTextContent(html string) string {
	// 移除 script 和 style
	scriptRe := regexp.MustCompile(`<script[^>]*>[\s\S]*?</script>`)
	html = scriptRe.ReplaceAllString(html, "")
	styleRe := regexp.MustCompile(`<style[^>]*>[\s\S]*?</style>`)
	html = styleRe.ReplaceAllString(html, "")

	// 提取正文
	bodyRe := regexp.MustCompile(`<body[^>]*>([\s\S]*)</body>`)
	if matches := bodyRe.FindStringSubmatch(html); len(matches) > 1 {
		html = matches[1]
	}

	text := cleanHTML(html)

	// 限制长度
	if len(text) > 5000 {
		text = text[:5000] + "...[truncated]"
	}

	return text
}

// extractCodeBlocks 提取代码块
func extractCodeBlocks(html string) []string {
	var codes []string

	// 匹配 <pre><code> 或 <pre class="rust">
	codeRe := regexp.MustCompile(`<pre[^>]*><code[^>]*>([\s\S]*?)</code></pre>`)
	for _, match := range codeRe.FindAllStringSubmatch(html, 10) {
		if len(match) > 1 {
			code := cleanHTML(match[1])
			if len(code) > 20 && len(code) < 3000 {
				codes = append(codes, code)
			}
		}
	}

	return codes
}

// GitHubReadme 获取 GitHub README
type GitHubReadme struct {
	client *http.Client
}

func NewGitHubReadme() *GitHubReadme {
	return &GitHubReadme{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *GitHubReadme) Name() string { return "github_readme" }
func (t *GitHubReadme) Description() string {
	return "获取 GitHub 仓库的 README 文件，通常包含库的使用说明和示例"
}
func (t *GitHubReadme) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"repo": map[string]any{"type": "string", "description": "仓库路径，如 owner/repo"},
		},
		"required": []string{"repo"},
	}
}

func (t *GitHubReadme) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		Repo string `json:"repo"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	// 尝试获取 README
	readmeURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/README.md", args.Repo)

	req, _ := http.NewRequestWithContext(ctx, "GET", readmeURL, nil)
	resp, err := t.client.Do(req)
	if err != nil {
		return FormatOutput(map[string]any{"success": false, "error": err.Error()}), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// 尝试 master 分支
		readmeURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/master/README.md", args.Repo)
		req, _ = http.NewRequestWithContext(ctx, "GET", readmeURL, nil)
		resp, err = t.client.Do(req)
		if err != nil {
			return FormatOutput(map[string]any{"success": false, "error": err.Error()}), nil
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != 200 {
		return FormatOutput(map[string]any{"success": false, "error": "README not found"}), nil
	}

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	// 限制长度
	if len(content) > 10000 {
		content = content[:10000] + "\n...[truncated]"
	}

	return FormatOutput(map[string]any{
		"success": true,
		"repo":    args.Repo,
		"content": content,
	}), nil
}
