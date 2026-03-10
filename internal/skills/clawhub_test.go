package skills

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseClawHubSource(t *testing.T) {
	t.Run("slug shorthand", func(t *testing.T) {
		got, err := ParseClawHubSource("clawhub://gifgrep")
		require.NoError(t, err)
		assert.Equal(t, DefaultClawHubRegistry, got.Registry)
		assert.Equal(t, "gifgrep", got.Slug)
	})

	t.Run("skill page url", func(t *testing.T) {
		got, err := ParseClawHubSource("https://clawhub.ai/steipete/gifgrep")
		require.NoError(t, err)
		assert.Equal(t, "https://clawhub.ai", got.Registry)
		assert.Equal(t, "steipete", got.Owner)
		assert.Equal(t, "gifgrep", got.Slug)
	})

	t.Run("api detail url", func(t *testing.T) {
		got, err := ParseClawHubSource("https://clawhub.ai/api/v1/skills/gifgrep?version=1.2.3")
		require.NoError(t, err)
		assert.Equal(t, "gifgrep", got.Slug)
		assert.Equal(t, "1.2.3", got.Version)
	})

	t.Run("invalid url", func(t *testing.T) {
		_, err := ParseClawHubSource("https://clawhub.ai/skills")
		require.Error(t, err)
	})
}

func TestInstallFromClawHub(t *testing.T) {
	workspace := t.TempDir()
	zipBytes := buildTestSkillZip(t, map[string]string{
		"SKILL.md":       "# GifGrep\n\nSearch GIFs.",
		"docs/README.md": "hello",
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/skills/gifgrep":
			_, _ = io.WriteString(w, `{"skill":{"slug":"gifgrep","displayName":"GifGrep","tags":{"latest":"1.2.3"}},"latestVersion":{"version":"1.2.3","createdAt":0}}`)
		case r.URL.Path == "/api/v1/download":
			assert.Equal(t, "gifgrep", r.URL.Query().Get("slug"))
			assert.Equal(t, "1.2.3", r.URL.Query().Get("version"))
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	installer := NewInstaller(workspace)
	got, err := installer.InstallFromClawHub(ClawHubSource{
		Registry: server.URL,
		Slug:     "gifgrep",
	})
	require.NoError(t, err)

	assert.Equal(t, "gifgrep", got.Name)
	assert.Equal(t, "clawhub", got.Type)
	assert.Equal(t, "1.2.3", got.Version)
	assert.Equal(t, filepath.Join(workspace, "skills", "gifgrep"), got.Location)

	skillPath := filepath.Join(workspace, "skills", "gifgrep", "SKILL.md")
	docPath := filepath.Join(workspace, "skills", "gifgrep", "docs", "README.md")
	content, err := os.ReadFile(skillPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "GifGrep")

	_, err = os.Stat(docPath)
	require.NoError(t, err)
}

func buildTestSkillZip(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := writer.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(f, content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buf.Bytes()
}
