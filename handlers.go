package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"strings"
	"time"
)

// ========== Prometheus 指标 ==========

var (
	mu sync.Mutex

	alertsReceivedTotal  int64
	alertsAnalyzedTotal  int64
	analysisErrorsTotal  int64
	lastAnalysisDuration float64
	webhookStartTime     = time.Now()

	categorySeverityCount = map[string]int64{}
)

// ========== CORS 中间件 ==========

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ========== Webhook 接收 ==========

func webhookHandler(model, baseURL, weixinURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "仅支持 POST", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("[ERROR] 读取请求体失败: %v", err)
			http.Error(w, "读取请求体失败", http.StatusBadRequest)
			return
		}

		var payload alertmanagerPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			log.Printf("[ERROR] 解析 Alertmanager 载荷失败: %v", err)
			http.Error(w, "解析 JSON 失败", http.StatusBadRequest)
			return
		}

		now := time.Now().Format("2006-01-02 15:04:05")
		log.Printf("════════════════════════════════════════════")
		log.Printf("[%s] 收到告警组: status=%s, receiver=%s, 告警数=%d",
			now, payload.Status, payload.Receiver, len(payload.Alerts))

		// 立刻返回 200 OK
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))

		atomic.AddInt64(&alertsReceivedTotal, int64(len(payload.Alerts)))

		// 后台异步处理
		alertsCopy := make([]alertItem, len(payload.Alerts))
		copy(alertsCopy, payload.Alerts)
		statusCopy := payload.Status

		go func(alerts []alertItem, groupStatus string) {
			for i, a := range alerts {
				alertName := a.Labels["alertname"]
				if alertName == "" {
					alertName = "unknown"
				}

				log.Printf("  [后台] 告警 %d/%d: %s [%s]", i+1, len(alerts), alertName, a.Status)

				// 1. 存入数据库
				alertJSON, err := json.Marshal(a)
				if err != nil {
					log.Printf("  [DB错误] 序列化告警失败: %v", err)
					continue
				}
				alertID, dbErr := insertAlert(a, string(alertJSON))
				if dbErr != nil {
					log.Printf("  [DB错误] 插入告警失败: %v", dbErr)
				}

				// 2. 处理 resolved 事件
				if groupStatus == "resolved" || a.Status == "resolved" {
					log.Printf("  [恢复] 告警 %s 已恢复", alertName)
					if weixinURL != "" && alertID > 0 {
						ok, errMsg := sendWeixinResolved(a, weixinURL)
						if ok {
							insertNotification(alertID, "weixin", "success", "")
						} else {
							insertNotification(alertID, "weixin", "failed", errMsg)
							log.Printf("  [企业微信] 恢复通知发送失败: %s", errMsg)
						}
					}
					continue
				}

				// 3. AI 分析（仅 firing 告警）
				alertText := formatAlert(a)
				analysisStart := time.Now()
				ar, rawResp, aiErr := analyzeWithOllama(alertText, model, baseURL)
				analysisDur := time.Since(analysisStart).Milliseconds()

				if aiErr != nil {
					atomic.AddInt64(&analysisErrorsTotal, 1)
					log.Printf("  [AI分析失败] %v", aiErr)
					if alertID > 0 {
						insertAnalysis(alertID, analysisResult{
							Category: "分析失败",
							Severity: "info",
							Summary:  aiErr.Error(),
						}, analysisDur, rawResp)
					}
					continue
				}

				// 4. 记录分析指标 + 存库
				atomic.AddInt64(&alertsAnalyzedTotal, 1)
				mu.Lock()
				lastAnalysisDuration = float64(analysisDur) / 1000
				key := ar.Category + "|" + ar.Severity
				categorySeverityCount[key]++
				mu.Unlock()

				if alertID > 0 {
					insertAnalysis(alertID, ar, analysisDur, rawResp)
				}

				log.Printf("  ┌─ AI 分析结果 ──────────────────────────")
				log.Printf("  │ 类别: %s / 等级: %s", ar.Category, ar.Severity)
				log.Printf("  │ 概要: %s", ar.Summary)
				log.Printf("  │ 耗时: %dms", analysisDur)
				for j, rc := range ar.RootCauses {
					log.Printf("  │ 根因%d: %s", j+1, rc)
				}
				for j, act := range ar.Actions {
					log.Printf("  │ 建议%d: %s", j+1, act)
				}
				log.Printf("  └──────────────────────────────────────────")

				// 5. 企业微信通知
				if weixinURL != "" && alertID > 0 {
					ok, errMsg := sendWeixinAlert(a, ar, weixinURL)
					if ok {
						insertNotification(alertID, "weixin", "success", "")
					} else {
						insertNotification(alertID, "weixin", "failed", errMsg)
						log.Printf("  [企业微信] 通知发送失败: %s", errMsg)
					}
				}
			}
			log.Printf("════════════════════════════════════════════")
		}(alertsCopy, statusCopy)
	}
}

// ========== Health / Metrics ==========

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	var sb strings.Builder

	uptime := time.Since(webhookStartTime).Seconds()
	sb.WriteString("# HELP aiops_webhook_uptime_seconds Webhook 运行时长\n")
	sb.WriteString("# TYPE aiops_webhook_uptime_seconds gauge\n")
	sb.WriteString(FormatFloat("aiops_webhook_uptime_seconds", uptime, "%.2f"))

	sb.WriteString("\n# HELP aiops_alerts_received_total 收到的告警总数\n")
	sb.WriteString("# TYPE aiops_alerts_received_total counter\n")
	sb.WriteString(FormatInt("aiops_alerts_received_total", atomic.LoadInt64(&alertsReceivedTotal)))

	sb.WriteString("\n# HELP aiops_alerts_analyzed_total AI 分析完成总数\n")
	sb.WriteString("# TYPE aiops_alerts_analyzed_total counter\n")
	sb.WriteString(FormatInt("aiops_alerts_analyzed_total", atomic.LoadInt64(&alertsAnalyzedTotal)))

	sb.WriteString("\n# HELP aiops_analysis_errors_total AI 分析失败总数\n")
	sb.WriteString("# TYPE aiops_analysis_errors_total counter\n")
	sb.WriteString(FormatInt("aiops_analysis_errors_total", atomic.LoadInt64(&analysisErrorsTotal)))

	sb.WriteString("\n# HELP aiops_analysis_duration_seconds 最近一次 AI 分析耗时\n")
	sb.WriteString("# TYPE aiops_analysis_duration_seconds gauge\n")
	mu.Lock()
	dur := lastAnalysisDuration
	mu.Unlock()
	sb.WriteString(FormatFloat("aiops_analysis_duration_seconds", dur, "%.2f"))

	sb.WriteString("\n# HELP aiops_analysis_by_category 按 category × severity 的分析计数\n")
	sb.WriteString("# TYPE aiops_analysis_by_category counter\n")
	mu.Lock()
	for key, count := range categorySeverityCount {
		parts := strings.SplitN(key, "|", 2)
		cat := parts[0]
		sev := "unknown"
		if len(parts) > 1 {
			sev = parts[1]
		}
		sb.WriteString(FormatLabelInt("aiops_analysis_by_category", count, "category", cat, "severity", sev))
	}
	mu.Unlock()

	w.Write([]byte(sb.String()))
}

// ========== REST API Handler ==========

func apiListAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "仅支持 GET", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	status := query.Get("status")
	severity := query.Get("severity")
	category := query.Get("category")

	page, _ := strconv.Atoi(query.Get("page"))
	if page < 1 { page = 1 }
	size, _ := strconv.Atoi(query.Get("size"))
	if size < 1 || size > 100 { size = 20 }

	resp, err := listAlerts(status, severity, category, page, size)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func apiGetAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "仅支持 GET", http.StatusMethodNotAllowed)
		return
	}

	// URL: /api/alerts/123
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/alerts/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "缺少 alert ID", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "无效的 alert ID", http.StatusBadRequest)
		return
	}

	detail, err := getAlertDetail(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "告警不存在"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func apiGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "仅支持 GET", http.StatusMethodNotAllowed)
		return
	}

	stats, err := getStats()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// ========== 工具函数 ==========

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[ERROR] JSON 编码失败: %v", err)
	}
}

func FormatInt(name string, val int64) string {
	return name + " " + strconv.FormatInt(val, 10) + "\n"
}

func FormatFloat(name string, val float64, fmtStr string) string {
	return name + " " + formatFloat(val, fmtStr) + "\n"
}

func formatFloat(val float64, fmtStr string) string {
	// fmtStr 格式如 "%.2f"，提取精度
	prec := 2
	if n, _ := fmt.Sscanf(fmtStr, "%%.%df", &prec); n == 0 {
		prec = 2
	}
	return strconv.FormatFloat(val, 'f', prec, 64)
}

func FormatLabelInt(name string, val int64, labels ...string) string {
	lb := ""
	for i := 0; i < len(labels); i += 2 {
		lb += labels[i] + `="` + labels[i+1] + `",`
	}
	lb = strings.TrimRight(lb, ",")
	return name + "{" + lb + "} " + strconv.FormatInt(val, 10) + "\n"
}
