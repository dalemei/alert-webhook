package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type weixinMarkdown struct {
	MsgType  string `json:"msgtype"`
	Markdown struct {
		Content string `json:"content"`
	} `json:"markdown"`
}

type weixinText struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

type weixinResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func sendWeixinAlert(a alertItem, ar analysisResult, webhookURL string) (bool, string) {
	// 用 <font color="warning"> 这是企业微信的绿色（warning）/红色（comment）
	// 实际规范：
	//   <font color="warning">  = 橙黄色
	//   <font color="comment">  = 灰色
	//   <font color="info">     = 绿色
	severityColor := "warning"
	severityEmoji := "⚠️"
	if strings.ToLower(a.Labels["severity"]) == "critical" {
		severityColor = "warning"
		severityEmoji = "🚨"
	}

	statusText := "🔥 触发"
	if a.Status == "resolved" {
		statusText = "✅ 恢复"
		severityEmoji = "✅"
	}

	// 构建根因和建议列表
	rcLines := ""
	for i, rc := range ar.RootCauses {
		rcLines += fmt.Sprintf("> %d. %s\n", i+1, rc)
	}
	if rcLines == "" {
		rcLines = "> （暂无分析结果）\n"
	}

	actLines := ""
	for i, act := range ar.Actions {
		actLines += fmt.Sprintf("> %d. %s\n", i+1, act)
	}
	if actLines == "" {
		actLines = "> （暂无建议）\n"
	}

	// 使用 markdown_v2 兼容的消息格式
	// 企业微信机器人 markdown 支持有限，用 text 格式保底可读性好
	content := fmt.Sprintf(`%s **AIOps 告警通知**

> **告警名称**: %s
> **状态**: <font color="%s">%s</font>
> **等级**: <font color="%s">%s</font>
> **实例**: %s
> **时间**: %s

**AI 分析结果**
> **类别**: %s | **等级**: %s
> **概要**: %s

**可能根因**
%s
**建议操作**
%s
---
<font color="comment">AIOps 智能告警分析系统</font>`,
		severityEmoji,
		a.Labels["alertname"],
		severityColor, statusText,
		severityColor, a.Labels["severity"],
		a.Labels["instance"],
		a.StartsAt,
		ar.Category, ar.Severity, ar.Summary,
		rcLines,
		actLines,
	)

	msg := weixinMarkdown{MsgType: "markdown"}
	msg.Markdown.Content = content

	data, err := json.Marshal(msg)
	if err != nil {
		return false, fmt.Sprintf("序列化失败: %v", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return false, fmt.Sprintf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Sprintf("读取企业微信响应失败: %v", err)
	}
	var wr weixinResponse
	if err := json.Unmarshal(body, &wr); err != nil {
		return false, fmt.Sprintf("解析企业微信响应失败: %v", err)
	}

	if wr.ErrCode != 0 {
		return false, fmt.Sprintf("企业微信返回错误: errcode=%d errmsg=%s", wr.ErrCode, wr.ErrMsg)
	}

	log.Printf("[企业微信] 通知发送成功, errcode=0")
	return true, ""
}

// sendWeixinResolved 发送告警恢复通知
func sendWeixinResolved(a alertItem, webhookURL string) (bool, string) {
	content := fmt.Sprintf(`✅ **告警已恢复**

> **告警名称**: %s
> **实例**: %s
> **开始时间**: %s
> **恢复时间**: %s

---
<font color="comment">AIOps 智能告警分析系统</font>`,
		a.Labels["alertname"],
		a.Labels["instance"],
		a.StartsAt,
		a.EndsAt,
	)

	msg := weixinMarkdown{MsgType: "markdown"}
	msg.Markdown.Content = content

	data, err := json.Marshal(msg)
	if err != nil {
		return false, fmt.Sprintf("序列化失败: %v", err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return false, fmt.Sprintf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Sprintf("读取企业微信响应失败: %v", err)
	}
	var wr weixinResponse
	if err := json.Unmarshal(body, &wr); err != nil {
		return false, fmt.Sprintf("解析企业微信响应失败: %v", err)
	}

	if wr.ErrCode != 0 {
		return false, fmt.Sprintf("errmsg=%s", wr.ErrMsg)
	}
	return true, ""
}
