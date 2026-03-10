package skills

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

const DefaultClawHubRegistry = "https://clawhub.ai"

type ClawHubSource struct {
	Registry string
	Slug     string
	Owner    string
	Version  string
	Tag      string
}

type RecommendedSource struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Source      string `json:"source,omitempty"`
	BrowseURL   string `json:"browseUrl,omitempty"`
	Example     string `json:"example,omitempty"`
}

func RecommendedSources() []RecommendedSource {
	return []RecommendedSource{
		{
			ID:          "anthropics-official",
			Name:        "Anthropics (Official)",
			Type:        "github",
			Description: "Anthropic 官方技能库",
			Source:      "https://github.com/anthropics/skills/tree/main/skills",
			BrowseURL:   "https://github.com/anthropics/skills/tree/main/skills",
		},
		{
			ID:          "playwright-cli",
			Name:        "Playwright CLI",
			Type:        "github",
			Description: "Microsoft Playwright 自动化测试技能",
			Source:      "https://github.com/microsoft/playwright-cli/tree/main/skills",
			BrowseURL:   "https://github.com/microsoft/playwright-cli/tree/main/skills",
		},
		{
			ID:          "vercel-labs",
			Name:        "Vercel Labs",
			Type:        "github",
			Description: "Vercel Labs 技能库",
			Source:      "https://github.com/vercel-labs/agent-skills/tree/main/skills",
			BrowseURL:   "https://github.com/vercel-labs/agent-skills/tree/main/skills",
		},
		{
			ID:          "vercel-skills",
			Name:        "Vercel Skills",
			Type:        "github",
			Description: "Vercel 官方技能",
			Source:      "https://github.com/vercel-labs/skills/tree/main/skills",
			BrowseURL:   "https://github.com/vercel-labs/skills/tree/main/skills",
		},
		{
			ID:          "remotion",
			Name:        "Remotion",
			Type:        "github",
			Description: "Remotion 视频编辑技能",
			Source:      "https://github.com/remotion-dev/skills/tree/main/skills",
			BrowseURL:   "https://github.com/remotion-dev/skills/tree/main/skills",
		},
		{
			ID:          "superpowers",
			Name:        "Superpowers",
			Type:        "github",
			Description: "Superpowers 增强技能",
			Source:      "https://github.com/obra/superpowers/tree/main/skills",
			BrowseURL:   "https://github.com/obra/superpowers/tree/main/skills",
		},
		{
			ID:          "clawhub",
			Name:        "ClawHub Registry",
			Type:        "clawhub",
			Description: "ClawHub 公共技能市场，支持 slug、技能页 URL 和 API URL 安装",
			BrowseURL:   "https://clawhub.ai/skills",
			Example:     "clawhub://gifgrep",
		},
	}
}

func ParseClawHubSource(raw string) (ClawHubSource, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ClawHubSource{}, fmt.Errorf("clawhub source is required")
	}

	if strings.HasPrefix(raw, "clawhub://") {
		slug := strings.TrimSpace(strings.TrimPrefix(raw, "clawhub://"))
		if slug == "" {
			return ClawHubSource{}, fmt.Errorf("clawhub slug is required")
		}
		return ClawHubSource{
			Registry: DefaultClawHubRegistry,
			Slug:     slug,
		}, nil
	}

	if strings.HasPrefix(raw, "clawhub:") {
		slug := strings.TrimSpace(strings.TrimPrefix(raw, "clawhub:"))
		if slug == "" {
			return ClawHubSource{}, fmt.Errorf("clawhub slug is required")
		}
		return ClawHubSource{
			Registry: DefaultClawHubRegistry,
			Slug:     slug,
		}, nil
	}

	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return ClawHubSource{}, fmt.Errorf("invalid clawhub source: %w", err)
	}

	host := strings.ToLower(u.Hostname())
	if host != "clawhub.ai" && host != "www.clawhub.ai" {
		return ClawHubSource{}, fmt.Errorf("unsupported clawhub host %q", u.Hostname())
	}

	registry := "https://" + u.Host
	segments := splitSourcePath(u.Path)

	if len(segments) >= 4 && segments[0] == "api" && segments[1] == "v1" && segments[2] == "skills" {
		slug := segments[3]
		return ClawHubSource{
			Registry: registry,
			Slug:     slug,
			Version:  strings.TrimSpace(u.Query().Get("version")),
			Tag:      strings.TrimSpace(u.Query().Get("tag")),
		}, nil
	}

	if len(segments) == 2 && segments[0] != "skills" && segments[0] != "souls" {
		return ClawHubSource{
			Registry: registry,
			Owner:    segments[0],
			Slug:     segments[1],
			Version:  strings.TrimSpace(u.Query().Get("version")),
			Tag:      strings.TrimSpace(u.Query().Get("tag")),
		}, nil
	}

	return ClawHubSource{}, fmt.Errorf("unsupported clawhub URL: %s", raw)
}

func splitSourcePath(rawPath string) []string {
	rawPath = path.Clean(strings.TrimSpace(rawPath))
	if rawPath == "." || rawPath == "/" || rawPath == "" {
		return nil
	}
	parts := strings.Split(strings.Trim(rawPath, "/"), "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." {
			continue
		}
		out = append(out, part)
	}
	return out
}
