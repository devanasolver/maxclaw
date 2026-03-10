package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsClawHubSource(t *testing.T) {
	assert.True(t, isClawHubSource("clawhub://gifgrep"))
	assert.True(t, isClawHubSource("clawhub:gifgrep"))
	assert.True(t, isClawHubSource("https://clawhub.ai/steipete/gifgrep"))
	assert.False(t, isClawHubSource("https://github.com/anthropics/skills"))
}
