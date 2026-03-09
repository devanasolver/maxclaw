package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRefreshTitleCreatesAutoTitleFromUserRequest(t *testing.T) {
	sess := &Session{Key: "desktop:test"}
	sess.AddMessage("user", "帮我修复 QQ 机器人不回复的问题")

	changed := RefreshTitle(sess)

	assert.True(t, changed)
	assert.Equal(t, "修复 QQ 机器人不回复的问题", sess.Title)
	assert.Equal(t, TitleSourceAuto, sess.TitleSource)
	assert.Equal(t, TitleStatePending, sess.TitleState)
	assert.False(t, sess.TitleUpdatedAt.IsZero())
}

func TestRefreshTitleStabilizesAfterConversationProgress(t *testing.T) {
	sess := &Session{Key: "desktop:test"}
	sess.AddMessage("user", "看下这个")
	sess.AddMessage("assistant", "先给我更多上下文")
	sess.AddMessage("user", "帮我修复 telegram 图片消息不显示的问题")
	sess.AddMessageWithTimeline("assistant", "已经修复", []TimelineEntry{
		{
			Kind: "activity",
			Activity: &TimelineActivity{
				Type:    "tool_result",
				Summary: "Updated telegram handler",
			},
		},
		{
			Kind: "activity",
			Activity: &TimelineActivity{
				Type:    "tool_result",
				Summary: "Added tests",
			},
		},
	})

	changed := RefreshTitle(sess)

	assert.True(t, changed)
	assert.Equal(t, "修复 telegram 图片消息不显示的问题", sess.Title)
	assert.Equal(t, TitleSourceAuto, sess.TitleSource)
	assert.Equal(t, TitleStateStable, sess.TitleState)
}

func TestRefreshTitleDoesNotOverrideUserTitle(t *testing.T) {
	sess := &Session{
		Key:         "desktop:test",
		Title:       "手动命名",
		TitleSource: TitleSourceUser,
	}
	sess.AddMessage("user", "帮我修复 QQ 机器人不回复的问题")
	sess.AddMessage("assistant", "好的")

	changed := RefreshTitle(sess)

	assert.True(t, changed)
	assert.Equal(t, "手动命名", sess.Title)
	assert.Equal(t, TitleStateStable, sess.TitleState)
}

func TestRefreshTitleStripsResumePrefix(t *testing.T) {
	sess := &Session{Key: "desktop:test"}
	sess.AddMessage("user", "[自动恢复任务]\n1/3 步\n\n用户指令: 帮我整理 maxclaw 的 make 命令")

	changed := RefreshTitle(sess)

	assert.True(t, changed)
	assert.Equal(t, "整理 maxclaw 的 make 命令", sess.Title)
}
