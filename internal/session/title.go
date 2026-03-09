package session

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	TitleSourceAuto = "auto"
	TitleSourceUser = "user"

	TitleStatePending = "pending"
	TitleStateStable  = "stable"
)

const maxAutoTitleRunes = 28

var titleTrimPrefixes = []string{
	"请帮我", "帮我", "麻烦你", "麻烦", "请你", "请", "我想让你", "我想", "我需要你", "我需要",
	"帮忙", "帮忙看下", "帮忙看看", "看下", "看看", "修复一下", "优化一下", "实现一下",
	"please ", "please,", "help me ", "can you ", "could you ", "would you ", "i want you to ", "i need you to ",
}

var titleSkipValues = map[string]struct{}{
	"":          {},
	"/new":      {},
	"/help":     {},
	"继续":        {},
	"继续执行":      {},
	"继续吧":       {},
	"继续任务":      {},
	"resume":    {},
	"continue":  {},
	"go on":     {},
	"thanks":    {},
	"thank you": {},
}

type titleCandidate struct {
	title string
	score int
}

// RefreshTitle updates the session title when it is auto-managed.
func RefreshTitle(sess *Session) bool {
	if sess == nil {
		return false
	}

	changed := false
	if sess.TitleSource == TitleSourceUser {
		if sess.TitleState != TitleStateStable {
			sess.TitleState = TitleStateStable
			changed = true
		}
		return changed
	}

	nextTitle, nextState := deriveAutoTitle(sess)
	if nextTitle == "" {
		return changed
	}

	if sess.Title == "" || sess.TitleSource != TitleSourceAuto {
		sess.Title = nextTitle
		sess.TitleSource = TitleSourceAuto
		sess.TitleState = nextState
		sess.TitleUpdatedAt = time.Now()
		return true
	}

	if sess.TitleState == "" {
		sess.TitleState = nextState
		changed = true
	}

	// Allow a single auto refinement while the title is still pending.
	if sess.TitleState == TitleStatePending && nextState == TitleStateStable && sess.Title != nextTitle {
		sess.Title = nextTitle
		sess.TitleState = nextState
		sess.TitleUpdatedAt = time.Now()
		return true
	}

	if sess.TitleState != nextState {
		sess.TitleState = nextState
		sess.TitleUpdatedAt = time.Now()
		return true
	}

	return changed
}

func deriveAutoTitle(sess *Session) (string, string) {
	candidate := selectAutoTitle(sess.Messages)
	if candidate == "" {
		return "", ""
	}
	if isStableTitleSession(sess.Messages) {
		return candidate, TitleStateStable
	}
	return candidate, TitleStatePending
}

func selectAutoTitle(messages []Message) string {
	best := titleCandidate{}
	for idx, msg := range messages {
		if msg.Role != "user" {
			continue
		}
		title := normalizeTitleCandidate(msg.Content)
		if title == "" {
			continue
		}
		score := scoreTitleCandidate(title, idx)
		if score > best.score {
			best = titleCandidate{title: title, score: score}
		}
	}
	return best.title
}

func normalizeTitleCandidate(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}

	text = stripPrefixedBlocks(text)
	text = strings.SplitN(text, "\n", 2)[0]
	text = strings.TrimSpace(text)
	text = strings.Trim(text, "\"'`[](){}<>")
	text = collapseWhitespace(text)
	if text == "" {
		return ""
	}

	lower := strings.ToLower(text)
	if _, skip := titleSkipValues[lower]; skip {
		return ""
	}

	for _, prefix := range titleTrimPrefixes {
		if strings.HasPrefix(lower, prefix) {
			text = strings.TrimSpace(text[len(prefix):])
			lower = strings.ToLower(text)
			break
		}
	}

	text = strings.TrimLeftFunc(text, func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSpace(r)
	})
	text = strings.TrimSpace(strings.TrimRight(text, "。！？!?.,;:"))
	text = collapseWhitespace(text)
	if text == "" {
		return ""
	}

	if utf8.RuneCountInString(text) > maxAutoTitleRunes {
		runes := []rune(text)
		text = string(runes[:maxAutoTitleRunes]) + "..."
	}
	return text
}

func stripPrefixedBlocks(text string) string {
	for _, marker := range []string{"[自动恢复任务]", "[恢复任务]"} {
		if strings.HasPrefix(text, marker) {
			if parts := strings.SplitN(text, "用户指令:", 2); len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return text
}

func collapseWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func scoreTitleCandidate(title string, index int) int {
	length := utf8.RuneCountInString(title)
	score := length
	if length >= 8 {
		score += 10
	}
	if length >= 14 {
		score += 8
	}
	if strings.ContainsAny(title, "/._-") {
		score += 2
	}
	if index == 0 {
		score += 4
	}
	if index > 2 {
		score -= 2
	}
	return score
}

func isStableTitleSession(messages []Message) bool {
	userCount := 0
	assistantCount := 0
	toolResultCount := 0

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			if normalizeTitleCandidate(msg.Content) != "" {
				userCount++
			}
		case "assistant":
			assistantCount++
		}
		for _, entry := range msg.Timeline {
			if entry.Kind == "activity" && entry.Activity != nil && entry.Activity.Type == "tool_result" {
				toolResultCount++
			}
		}
	}

	if assistantCount == 0 {
		return false
	}
	return userCount >= 2 || toolResultCount >= 2 || len(messages) >= 4
}
