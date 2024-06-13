package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type Alert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    string            `json:"startsAt"`
	EndsAt      string            `json:"endsAt"`
}

type WebhookMessage struct {
	Alerts []Alert `json:"alerts"`
}

type DingTalkMessage struct {
	MsgType    string `json:"msgtype"`
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
	defer resp.Body.Close()

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
		alert.EndsAt,
	)
}

func main() {
	r := gin.Default()

	// 为了安全起见，不要信任所有代理
	r.SetTrustedProxies(nil)

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
			dingtalkMessage := DingTalkMessage{
				MsgType: "actionCard",
			}
			dingtalkMessage.ActionCard.Title = "🚨 New Alert"
			dingtalkMessage.ActionCard.Text = formatMarkdown(alert)
			dingtalkMessage.ActionCard.BtnOrientation = "0" // 0: horizontal, 1: vertical
			dingtalkMessage.ActionCard.Btns = []struct {
				Title     string `json:"title"`
				ActionURL string `json:"actionURL"`
			}{
				{
					Title:     "Silence Alert",
					ActionURL: "http://your-silence-alert-url.com", // 请替换为实际的静默告警URL
				},
			}
			dingtalkMessage.At.AtMobiles = []string{"值班人的手机号"}           // 将值班人的手机号填入此处
			dingtalkMessage.At.IsAtAll = alert.Labels["severity"] == "critical" // 如果告警严重性为紧急，则@所有人

			if err := sendToDingTalk(dingtalkWebhookURL, dingtalkMessage); err != nil {
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
	r.Run(":" + port)
}
