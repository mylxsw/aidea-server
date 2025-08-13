package dingding

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mylxsw/go-utils/must"
	"github.com/mylxsw/go-utils/ternary"

	"github.com/mylxsw/go-utils/str"
)

// DingdingMessage is a message holds all information for a dingding sender
type DingdingMessage struct {
	Message MarkdownMessage `json:"message"`
	Token   string          `json:"token"`
}

func (dm *DingdingMessage) Encode() []byte {
	data, _ := json.Marshal(dm)
	return data
}

func (dm *DingdingMessage) Decode(data []byte) error {
	return json.Unmarshal(data, &dm)
}

// MarkdownMessage is a markdown message for dingding
type MarkdownMessage struct {
	Type     string              `json:"msgtype,omitempty"`
	Markdown MarkdownMessageBody `json:"markdown,omitempty"`
	At       MessageAtSomebody   `json:"at,omitempty"`
}

// Encode markdown message to json bytes
func (m MarkdownMessage) Encode() ([]byte, error) {
	return json.Marshal(m)
}

func (m MarkdownMessage) EncodeSlack() ([]byte, error) {
	return []byte(slackRequestBody("event", "AIdea", "sparkles", m.Markdown.Text)), nil
}

func (m MarkdownMessage) EncodeApprise(tags string) ([]byte, error) {
	return []byte(appriseRequestBody(m.Markdown.Title, m.Markdown.Text, tags)), nil
}

// NewMarkdownMessage create a new MarkdownMessage
func NewMarkdownMessage(title string, body string, mobiles []string) MarkdownMessage {
	mobilesFromBody := ExtractAtSomeones(body)
	mobiles = str.Diff(mobiles, mobilesFromBody)
	if len(mobiles) > 0 {
		var atSomeone = ""
		for _, mobile := range mobiles {
			atSomeone += fmt.Sprintf("@%s ", mobile)
		}

		body += "\n\n" + atSomeone
	}

	return MarkdownMessage{
		Type: "markdown",
		Markdown: MarkdownMessageBody{
			Title: title,
			Text:  body,
		},
		At: MessageAtSomebody{
			Mobiles: str.Union(mobilesFromBody, mobiles),
		},
	}
}

var atSomebodyRegexp = regexp.MustCompile(`@1\d{10}(\s|\n|$)`)

func ExtractAtSomeones(body string) []string {
	results := make([]string, 0)
	for _, s := range atSomebodyRegexp.FindAllString(body, -1) {
		results = append(results, strings.TrimSpace(strings.TrimLeft(s, "@")))
	}

	return str.Distinct(results)
}

// MarkdownMessageBody is markdown body
type MarkdownMessageBody struct {
	Title      string `json:"title,omitempty"`
	Text       string `json:"text,omitempty"`
	MessageURL string `json:"messageUrl,omitempty"`
}

// MessageAtSomebody @ someone
type MessageAtSomebody struct {
	Mobiles []string `json:"atMobiles"`
	ToAll   bool     `json:"isAtAll"`
}

type Dingding struct {
	Endpoint     string
	Token        string
	Secret       string
	SlackMode    bool
	AppriseMode  bool
	AppriseURL   string
	AppriseToken string
	AppriseTags  string
}

func NewDingding(slackMode bool, token string, secret string, appriseMode bool, appriseURL string, appriseToken string, appriseTags string) *Dingding {
	return &Dingding{SlackMode: slackMode, Endpoint: "https://oapi.dingtalk.com/robot/send", Token: token, Secret: secret, AppriseMode: appriseMode, AppriseURL: appriseURL, AppriseToken: appriseToken, AppriseTags: appriseTags}
}

type Message interface {
	Encode() ([]byte, error)
	EncodeSlack() ([]byte, error)
	EncodeApprise(tags string) ([]byte, error)
}

// dingResponse 钉钉响应
type dingResponse struct {
	ErrorCode    int    `json:"errcode"`
	ErrorMessage string `json:"errmsg"`
}

func (ding *Dingding) Send(msg Message) error {
	if ding.Token == "" {
		// token 为空，不发送消息
		return nil
	}

	if ding.AppriseMode {
		return ding.sendAppriseNotify(msg)
	}

	if ding.SlackMode {
		return ding.sendSlackNotify(msg)
	}

	return ding.sendDingDingNotify(msg)
}

func (ding *Dingding) sendAppriseNotify(msg Message) error {
	msgEncoded, err := msg.EncodeApprise(ding.AppriseTags)
	if err != nil {
		return fmt.Errorf("apprise message encode failed: %s", err.Error())
	}

	endpointURL := ding.AppriseURL

	reader := bytes.NewReader(msgEncoded)
	request, err := http.NewRequest("POST", endpointURL, reader)
	if err != nil {
		return fmt.Errorf("apprise create request failed: %w", err)
	}

	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	usernameAndPassword := strings.SplitN(ding.AppriseToken, ":", 2)
	request.SetBasicAuth(usernameAndPassword[0], usernameAndPassword[1])

	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("apprise send msg failed: %w", err)
	}

	if resp.StatusCode > 299 {
		respBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("apprise read response failed: %w", err)
		}

		return fmt.Errorf("apprise send finished, response： %s", string(respBytes))
	}

	return nil
}

func (ding *Dingding) sendSlackNotify(msg Message) error {
	msgEncoded, err := msg.EncodeSlack()
	if err != nil {
		return fmt.Errorf("slack message encode failed: %s", err.Error())
	}

	endpointURL := "https://hooks.slack.com/services/" + ding.Token

	reader := bytes.NewReader(msgEncoded)
	request, err := http.NewRequest("POST", endpointURL, reader)
	if err != nil {
		return fmt.Errorf("slack create request failed: %w", err)
	}

	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("slack send msg failed: %w", err)
	}

	if resp.StatusCode > 299 {
		respBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("slack read response failed: %w", err)
		}

		return fmt.Errorf("slack send finished, response： %s", string(respBytes))
	}

	return nil
}

func (ding *Dingding) sendDingDingNotify(msg Message) error {
	v := url.Values{}
	v.Add("access_token", ding.Token)

	if ding.Secret != "" {
		timestamp := time.Now().UnixNano() / 1e6
		hash := hmac.New(sha256.New, []byte(ding.Secret))
		_, _ = io.WriteString(hash, fmt.Sprintf("%d\n%s", timestamp, ding.Secret))

		v.Add("timestamp", fmt.Sprintf("%d", timestamp))
		v.Add("sign", base64.StdEncoding.EncodeToString(hash.Sum(nil)))
	}

	endpointURL := ding.Endpoint + "?" + v.Encode()

	msgEncoded, err := msg.Encode()
	if err != nil {
		return fmt.Errorf("dingding message encode failed: %s", err.Error())
	}

	reader := bytes.NewReader(msgEncoded)
	request, err := http.NewRequest("POST", endpointURL, reader)
	if err != nil {
		return fmt.Errorf("dingding create request failed: %w", err)
	}

	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("dingding send msg failed: %w", err)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("dingding read response failed: %w", err)
	}

	var dresp dingResponse
	if err := json.Unmarshal(respBytes, &dresp); err != nil {
		return fmt.Errorf("send finished, response： %s", string(respBytes))
	}

	if dresp.ErrorCode > 0 {
		return fmt.Errorf("[%d] %s", dresp.ErrorCode, dresp.ErrorMessage)
	}

	return nil
}

// slackRequestBody Build Slack request body
func slackRequestBody(channelName string, username string, emoji string, text string) string {
	payload := map[string]interface{}{
		"channel":    ternary.If(strings.HasPrefix(channelName, "#"), channelName, "#"+channelName),
		"username":   ternary.If(username == "", "AIdea", username),
		"icon_emoji": ternary.If(emoji == "", ":sparkles:", ":"+emoji+":"),
		"text":       text,
	}

	payloadStr := string(must.Must(json.Marshal(payload)))

	params := url.Values{}
	params.Add("payload", payloadStr)
	return params.Encode()
}

// appriseRequestBody Build Apprise request body
func appriseRequestBody(title string, body string, tags string) string {
	params := url.Values{}
	if title != body {
		params.Add("title", title)
	}

	params.Add("body", body)
	params.Add("tags", tags)

	return params.Encode()
}
