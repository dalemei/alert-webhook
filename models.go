package main

import "time"

// ========== Alertmanager webhook 数据结构 ==========

type alertmanagerPayload struct {
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	Status            string            `json:"status"` // firing / resolved
	Receiver          string            `json:"receiver"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Alerts            []alertItem       `json:"alerts"`
}

type alertItem struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"startsAt"`
	EndsAt       string            `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// ========== Ollama API 数据结构 ==========

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaReq struct {
	Model       string    `json:"model"`
	Messages    []chatMsg `json:"messages"`
	Stream      bool      `json:"stream"`
	Format      string    `json:"format"`
	Temperature float64   `json:"temperature"`
}

type ollamaResp struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// ========== AI 分析结果 ==========

type analysisResult struct {
	Category   string   `json:"category"`
	Severity   string   `json:"severity"`
	Summary    string   `json:"summary"`
	RootCauses []string `json:"root_causes"`
	Actions    []string `json:"actions"`
}

// ========== 数据库模型 ==========

type AlertRecord struct {
	ID           int64     `json:"id"`
	Fingerprint  string    `json:"fingerprint"`
	Alertname    string    `json:"alertname"`
	Status       string    `json:"status"`
	Severity     string    `json:"severity"`
	Instance     string    `json:"instance"`
	Job          string    `json:"job"`
	Summary      string    `json:"summary"`
	Description  string    `json:"description"`
	StartsAt     string    `json:"starts_at"`
	EndsAt       string    `json:"ends_at"`
	GeneratorURL string    `json:"generator_url"`
	RawJSON      string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AnalysisRecord struct {
	ID          int64     `json:"id"`
	AlertID     int64     `json:"alert_id"`
	Category    string    `json:"category"`
	Severity    string    `json:"severity"`
	Summary     string    `json:"summary"`
	RootCauses  string    `json:"root_causes"`
	Actions     string    `json:"actions"`
	DurationMs  int64     `json:"duration_ms"`
	RawResponse string    `json:"-"`
	CreatedAt   time.Time `json:"created_at"`
}

type NotificationRecord struct {
	ID        int64     `json:"id"`
	AlertID   int64     `json:"alert_id"`
	Channel   string    `json:"channel"`
	Status    string    `json:"status"`
	ErrorMsg  string    `json:"error_msg"`
	CreatedAt time.Time `json:"created_at"`
}

// ========== REST API 响应 ==========

type AlertDetail struct {
	Alert       AlertRecord      `json:"alert"`
	Analysis    *AnalysisRecord  `json:"analysis,omitempty"`
	Notifications []NotificationRecord `json:"notifications,omitempty"`
}

type AlertListResponse struct {
	Total  int64         `json:"total"`
	Page   int           `json:"page"`
	Size   int           `json:"size"`
	Alerts []AlertRecord `json:"alerts"`
}

type StatsResponse struct {
	TotalAlerts       int64            `json:"total_alerts"`
	FiringCount       int64            `json:"firing_count"`
	ResolvedCount     int64            `json:"resolved_count"`
	AnalyzedCount     int64            `json:"analyzed_count"`
	ByCategory        []CategoryCount  `json:"by_category"`
	BySeverity        []SeverityCount  `json:"by_severity"`
	TodayCount        int64            `json:"today_count"`
	AvgAnalysisMs     float64          `json:"avg_analysis_ms"`
}

type CategoryCount struct {
	Category string `json:"category"`
	Count    int64  `json:"count"`
}

type SeverityCount struct {
	Severity string `json:"severity"`
	Count    int64  `json:"count"`
}
