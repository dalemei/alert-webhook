package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	port := flag.Int("port", 9091, "webhook 监听端口")
	model := flag.String("model", "qwen2.5:7b", "Ollama 模型名")
	baseURL := flag.String("url", "http://localhost:11434", "Ollama 地址")
	logFile := flag.String("log", "", "可选：写入分析日志到指定文件（同时输出到 stdout）")
	dsn := flag.String("dsn", "", "MySQL DSN，留空则从环境变量 MYSQL_DSN 读取")
	weixinURL := flag.String("weixin", "", "企业微信 Webhook URL，留空则从环境变量 WEIXIN_WEBHOOK_URL 读取")
	frontend := flag.String("frontend", "./frontend-dist", "前端静态文件目录")
	flag.Parse()

	// 环境变量兜底：flag 为空时从环境变量读取
	if *dsn == "" {
		*dsn = os.Getenv("MYSQL_DSN")
	}
	if *weixinURL == "" {
		*weixinURL = os.Getenv("WEIXIN_WEBHOOK_URL")
	}

	// 日志：同时输出到 stdout（+可选文件）
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("无法打开日志文件 %s: %v", *logFile, err)
		}
		defer f.Close()
		log.SetOutput(io.MultiWriter(os.Stdout, f))
		log.Printf("日志同时输出到: %s", *logFile)
	}

	// 初始化数据库
	if *dsn != "" {
		if err := initDB(*dsn); err != nil {
			log.Printf("[WARN] 数据库连接失败: %v — 将继续运行但不持久化", err)
		} else {
			defer db.Close()
		}
	} else {
		log.Println("[WARN] 未设置 MySQL DSN（环境变量 MYSQL_DSN 或 -dsn flag），跳过数据库")
	}

	addr := fmt.Sprintf(":%d", *port)
	mux := http.NewServeMux()

	// Webhook 接收
	mux.HandleFunc("/webhook", webhookHandler(*model, *baseURL, *weixinURL))

	// 健康检查 & 监控
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/metrics", metricsHandler)

	// REST API
	mux.HandleFunc("GET /api/alerts", apiListAlerts)
	mux.HandleFunc("/api/stats", apiGetStats)
	mux.HandleFunc("/api/alerts/", apiGetAlert)

	// 前端静态文件（SPA fallback）
	fs := http.FileServer(http.Dir(*frontend))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/") || path == "/webhook" || path == "/health" || path == "/metrics" {
			http.NotFound(w, r)
			return
		}
		fs.ServeHTTP(w, r)
	})

	handler := corsMiddleware(mux)

	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("[SHUTDOWN] 收到信号 %v，正在优雅关闭...", sig)
		srv.Close()
	}()

	log.Printf("=======================================")
	log.Printf("AIOps 告警分析 Webhook v2.1 已启动")
	log.Printf("监听地址: %s", addr)
	log.Printf("Ollama:   %s (模型: %s)", *baseURL, *model)
	log.Printf("MySQL:    %s", maskDSN(*dsn))
	log.Printf("企业微信: %s", func() string {
		if *weixinURL != "" {
			return "已配置"
		}
		return "未配置"
	}())
	log.Printf("前端文件: %s", *frontend)
	log.Printf("=======================================")

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("服务启动失败: %v", err)
	}
	log.Println("[SHUTDOWN] 服务已安全关闭")
}

// maskDSN 隐藏 DSN 中的密码部分，避免日志泄露
func maskDSN(dsn string) string {
	if dsn == "" {
		return "(无)"
	}
	// 格式: user:password@tcp(host:port)/db?params
	// 只显示 user@host:port/db
	idxAt := strings.LastIndex(dsn, "@")
	if idxAt < 0 {
		return dsn
	}
	// 从 user:password 中取 user 部分
	userPart := dsn[:idxAt]
	if idxColon := strings.Index(userPart, ":"); idxColon > 0 {
		userPart = userPart[:idxColon]
	}
	return userPart + "@" + dsn[idxAt+1:]
}
