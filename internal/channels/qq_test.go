package channels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestResolveQQBotCredentials(t *testing.T) {
	t.Run("parses openclaw token", func(t *testing.T) {
		appID, appSecret := ResolveQQBotCredentials("", "", "1903066401:oX4NTL6ey96pKgoi")
		assert.Equal(t, "1903066401", appID)
		assert.Equal(t, "oX4NTL6ey96pKgoi", appSecret)
	})

	t.Run("uses access token as secret when app id already set", func(t *testing.T) {
		appID, appSecret := ResolveQQBotCredentials("1903066401", "", "oX4NTL6ey96pKgoi")
		assert.Equal(t, "1903066401", appID)
		assert.Equal(t, "oX4NTL6ey96pKgoi", appSecret)
	})

	t.Run("explicit secret wins", func(t *testing.T) {
		appID, appSecret := ResolveQQBotCredentials("1903066401", "real-secret", "1903066401:ignored")
		assert.Equal(t, "1903066401", appID)
		assert.Equal(t, "real-secret", appSecret)
	})
}

func TestQQChannelHandleC2CMessageUsesUserOpenID(t *testing.T) {
	ch := NewQQChannel(&QQConfig{
		Enabled:     true,
		AccessToken: "1903066401:oX4NTL6ey96pKgoi",
		AllowFrom:   []string{"414797086"},
	})

	var got *Message
	ch.SetMessageHandler(func(msg *Message) {
		got = msg
	})

	ch.handleC2CMessage(&qqC2CMessageEvent{
		ID:      "msg-1",
		Content: "Hello QQ",
		Author: qqC2CAuthor{
			ID:          "opaque-id",
			UserOpenID:  "USER-OPENID",
			UnionOpenID: "UNION-OPENID",
		},
	})

	require.NotNil(t, got)
	assert.Equal(t, "Hello QQ", got.Text)
	assert.Equal(t, "USER-OPENID", got.ChatID)

	ch.mu.RLock()
	defer ch.mu.RUnlock()
	assert.Equal(t, "msg-1", ch.lastInboundMsg["USER-OPENID"])
}

func TestQQChannelHandleC2CMessageUsesImageAttachmentPlaceholder(t *testing.T) {
	ch := NewQQChannel(&QQConfig{
		Enabled:     true,
		AccessToken: "1903066401:oX4NTL6ey96pKgoi",
	})

	var got *Message
	ch.SetMessageHandler(func(msg *Message) {
		got = msg
	})

	ch.handleC2CMessage(&qqC2CMessageEvent{
		ID: "msg-2",
		Author: qqC2CAuthor{
			UserOpenID: "USER-OPENID",
		},
		Attachments: []qqAttachment{
			{
				URL:         "//multimedia.nt.qq.com/image.png",
				FileName:    "image.png",
				ContentType: "image/png",
			},
		},
	})

	require.NotNil(t, got)
	assert.Equal(t, "[Image]", got.Text)
	require.NotNil(t, got.Media)
	assert.Equal(t, "image", got.Media.Type)
	assert.Equal(t, "https://multimedia.nt.qq.com/image.png", got.Media.URL)
	assert.Equal(t, "image/png", got.Media.MimeType)
}

func TestQQChannelHandleC2CMessageBlocksNonMatchingOpenIDAllowlist(t *testing.T) {
	ch := NewQQChannel(&QQConfig{
		Enabled:     true,
		AccessToken: "1903066401:oX4NTL6ey96pKgoi",
		AllowFrom:   []string{"ALLOWED-OPENID"},
	})

	var got *Message
	ch.SetMessageHandler(func(msg *Message) {
		got = msg
	})

	ch.handleC2CMessage(&qqC2CMessageEvent{
		ID:      "msg-1",
		Content: "Hello QQ",
		Author: qqC2CAuthor{
			UserOpenID: "OTHER-OPENID",
		},
	})

	assert.Nil(t, got)
}

func TestQQChannelSendMessageUsesLatestInboundMessageID(t *testing.T) {
	var (
		gotAuth string
		gotPath string
		gotBody map[string]interface{}
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp-1"}`))
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	ch := NewQQChannel(&QQConfig{
		Enabled:     true,
		AccessToken: "1903066401:oX4NTL6ey96pKgoi",
	})
	ch.httpClient = &http.Client{
		Transport: &rewriteHostTransport{
			target: http.DefaultTransport,
			base:   serverURL,
		},
	}

	ch.mu.Lock()
	ch.tokenSource = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-access-token"})
	ch.lastInboundMsg["USER-OPENID"] = "msg-1"
	ch.mu.Unlock()

	require.NoError(t, ch.SendMessage("USER-OPENID", "reply text"))
	assert.Equal(t, "QQBot test-access-token", gotAuth)
	assert.Equal(t, "/v2/users/USER-OPENID/messages", gotPath)
	assert.Equal(t, "reply text", gotBody["content"])
	assert.Equal(t, "msg-1", gotBody["msg_id"])
	assert.Equal(t, float64(1), gotBody["msg_seq"])
	assert.Equal(t, float64(0), gotBody["msg_type"])
}

func TestQQChannelSendPhotoUploadsFileDataAndSendsRichMedia(t *testing.T) {
	var (
		gotPaths  []string
		gotAuths  []string
		gotBodies []map[string]interface{}
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuths = append(gotAuths, r.Header.Get("Authorization"))
		gotPaths = append(gotPaths, r.URL.Path)

		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		gotBodies = append(gotBodies, body)

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/users/USER-OPENID/files":
			_, _ = w.Write([]byte(`{"file_info":"uploaded-file-info"}`))
		case "/v2/users/USER-OPENID/messages":
			_, _ = w.Write([]byte(`{"id":"resp-2"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	tmpFile, err := os.CreateTemp(t.TempDir(), "qq-image-*.png")
	require.NoError(t, err)
	_, err = tmpFile.Write([]byte("fake-image"))
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	ch := NewQQChannel(&QQConfig{
		Enabled:     true,
		AccessToken: "1903066401:oX4NTL6ey96pKgoi",
	})
	ch.httpClient = &http.Client{
		Transport: &rewriteHostTransport{
			target: http.DefaultTransport,
			base:   serverURL,
		},
	}

	ch.mu.Lock()
	ch.tokenSource = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-access-token"})
	ch.lastInboundMsg["USER-OPENID"] = "msg-9"
	ch.mu.Unlock()

	require.NoError(t, ch.SendPhoto("USER-OPENID", tmpFile.Name(), "caption text"))
	require.Len(t, gotPaths, 2)
	assert.Equal(t, []string{
		"/v2/users/USER-OPENID/files",
		"/v2/users/USER-OPENID/messages",
	}, gotPaths)
	assert.Equal(t, []string{
		"QQBot test-access-token",
		"QQBot test-access-token",
	}, gotAuths)

	assert.Equal(t, float64(1), gotBodies[0]["file_type"])
	assert.Equal(t, false, gotBodies[0]["srv_send_msg"])
	assert.NotEmpty(t, gotBodies[0]["file_data"])

	assert.Equal(t, "caption text", gotBodies[1]["content"])
	assert.Equal(t, "msg-9", gotBodies[1]["msg_id"])
	assert.Equal(t, float64(1), gotBodies[1]["msg_seq"])
	assert.Equal(t, float64(7), gotBodies[1]["msg_type"])

	media, ok := gotBodies[1]["media"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "uploaded-file-info", media["file_info"])
}

type rewriteHostTransport struct {
	target http.RoundTripper
	base   *url.URL
}

func (t *rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(context.Background())
	cloned.URL.Scheme = t.base.Scheme
	cloned.URL.Host = t.base.Host
	return t.target.RoundTrip(cloned)
}
