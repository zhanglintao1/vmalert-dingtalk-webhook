package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
)

type Alert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    string            `json:"startsAt"`
	EndAt       string            `json:"endAt"`
}

type WebhookMessage struct {
	Alerts []Alert `json:"alerts"`
}

type DingTalkMessage struct {
	MsgType    string `json:"msgType"`
	ActionCard struct {
		Title          string `json:"title"`
		Text           string `json:"text"`
		BtnOrientation string `json:"btnOrientation"`
		Btns           []struct {
			Title     string `json:"title"`
			ActionURL string `json:"actionURL"`
		} `json:"btns"`
	} `json:"actionCard"`
	At struct {
		AtMobiles []string `json:"atMobiles"`
		IsAtAll   bool     `json:"isAtAll"`
	} `json:"at"`
}

func sendToDingTalk(webhookURL string, message DingTalkMessage) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send message to DingTalk, status code: %d", resp.StatusCode)
	}
	return nil
}

func formatMarkdown(alert Alert) string {
	return fmt.Sprintf(
		"### 🚨 **Alert: %s**\n\n"+
			"**Status:** %s\n\n"+
			"**Severity:** %s\n\n"+
			"**Description:** %s\n\n"+
			"**Starts At:** %s\n\n"+
			"**Ends At:** %s\n\n",
		alert.Annotations["summary"],
		alert.Status,
		alert.Labels["severity"],
		alert.Annotations["description"],
		alert.StartsAt,
		alert.EndAt,
	)
}

func main() {
	r := gin.Default()

	r.POST("/node/:ddkey", func(c *gin.Context) {
		var message WebhookMessage
		if err := c.ShouldBindJSON(&message); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ddkey := c.Param("ddkey")
		dingtalkWebhookBaseURL := os.Getenv("DINGTALK_WEBHOOK_BASE_URL")
		if dingtalkWebhookBaseURL == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DINGTALK_WEBHOOK_BASE_URL environment variable is required"})
			return
		}
		dingtalkWebhookURL := dingtalkWebhookBaseURL + ddkey
		for _, alert := range message.Alerts {
			dingTalkMessage := DingTalkMessage{
				MsgType: "actionCard",
			}
			dingTalkMessage.ActionCard.Title = "🚨 New Alert"
			dingTalkMessage.ActionCard.Text = formatMarkdown(alert)
			dingTalkMessage.ActionCard.BtnOrientation = "0" // 0: horizontal, 1: vertical
			dingTalkMessage.ActionCard.Btns = []struct {
				Title     string `json:"title"`
				ActionURL string `json:"actionURL"`
			}{
				{
					Title:     "Silence Alert",
					ActionURL: "http://your-silence-alert-url.com", // 请替换为实际的静默告警URL
				},
			}
			dingTalkMessage.At.AtMobiles = []string{"值班人的手机号"}           // 将值班人的手机号填入此处
			dingTalkMessage.At.IsAtAll = alert.Labels["severity"] == "critical" // 如果告警严重性为紧急，则@所有人

			if err := sendToDingTalk(dingtalkWebhookURL, dingTalkMessage); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":", port)
}
