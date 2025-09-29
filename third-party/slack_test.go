package thirdPkg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

var slackConfig = &SlackConfig{
	WebHook:     "",
	ServiceName: "test-service",
}

func TestSlackConnection_SendMsgStr(t *testing.T) {
	slackConn, _ := NewSlackConnection(slackConfig)

	ctx := context.Background()
	err := slackConn.SendMsgStr(ctx, "Test message")

	assert.Nil(t, err)
}

func TestSlackConnection_SendMsg(t *testing.T) {
	slackConn, _ := NewSlackConnection(slackConfig)

	mockMsg := &Msg{
		Text: "Test message",
		Blocks: []any{
			map[string]any{
				"type": "section",
				"text": map[string]any{
					"type": "mrkdwn",
					"text": "Danny Torrence left the following review for your property:",
				},
			},
		},
		Attachments: []any{
			map[string]any{
				"mrkdwn_in":   []any{"text"},
				"color":       "#36a64f",
				"pretext":     "Optional pre-text that appears above the attachment block",
				"author_name": "author_name",
				"author_link": "http://flickr.com/bobby/",
				"author_icon": "https://placeimg.com/16/16/people",
				"title":       "title",
				"title_link":  "https://api.slack.com/",
				"text":        "Optional `text` that appears within the attachment",
				"thumb_url":   "http://placekitten.com/g/200/200",
				"footer":      "footer",
				"footer_icon": "https://platform.slack-edge.com/img/default_application_icon.png",
				"ts":          123456789,
			},
		},
	}

	ctx := context.Background()
	err := slackConn.SendMsg(ctx, mockMsg)

	assert.Nil(t, err)
}
