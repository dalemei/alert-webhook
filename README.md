# AIOps 告警智能分析平台

> SRE 用 Ollama + Go 搭建的 AI 告警分析管道 —— 从 Prometheus 告警到 AI 根因分析，全链路自动化。

```
Prometheus → Alertmanager → Go Webhook → Ollama AI 分析
                                    ↓
                              MySQL 持久化 + Web UI + 企业微信通知
```

---

## 功能

| 功能 | 说明 |
|------|------|
| 🔔 告警接收 | 接收 Alertmanager webhook，兼容 v1/v2 格式 |
| 🧠 AI 根因分析 | Ollama 本地推理，零外部 API 依赖，2-3 条根因 + 可执行建议 |
| ⚡ 异步处理 | 收到告警立即返回 200，后台 goroutine 分析（不占用 Alertmanager 超时） |
| 💾 持久化 | MySQL 存储告警、分析结果、通知记录，支持指纹去重 |
| 🌐 Web UI | 纯 HTML 单页应用：仪表盘 + 告警列表 + 详情页 |
| 📱 企业微信通知 | firing 告警推送 markdown 消息（含根因+建议），resolved 推送恢复通知 |
| 📊 Prometheus 指标 | `/metrics` 端点暴露 6 个指标，可接入 Grafana |
| 🔌 REST API | `GET /api/stats`、`GET /api/alerts`、`GET /api/alerts/{id}` |

---

## 架构

```
                    ┌──────────┐
                    │Prometheus│
                    └────┬─────┘
                         │ alert rules trigger
                         ▼
                   ┌─────────────┐
                   │Alertmanager │
                   └──────┬──────┘
                          │ POST /webhook
                          ▼
┌─────────────────────────────────────────────────────┐
│                 Go Webhook (:9091)                   │
│                                                      │
│  ┌─────────┐   ┌──────────┐   ┌─────────────────┐  │
│  │Webhook  │──▶│ Async    │──▶│ Ollama (qwen2.5) │  │
│  │Handler  │   │ Analysis │   │ root cause + fix │  │
│  └─────────┘   └────┬─────┘   └─────────────────┘  │
│                     │                                │
│         ┌───────────┼───────────┐                   │
│         ▼           ▼           ▼                   │
│  ┌──────┴──┐ ┌──────┴──┐ ┌─────┴──────┐            │
│  │  MySQL  │ │ 企业微信 │ │ Web UI    │            │
│  │ 持久化  │ │ 通知    │ │ (SPA)     │            │
│  └─────────┘ └─────────┘ └────────────┘            │
└─────────────────────────────────────────────────────┘
```

---

## 快速开始

### 前置依赖

- Go 1.23+
- MySQL 5.7+（可选，不配也能跑但不持久化）
- Ollama（本地或远程，模型 `qwen2.5:7b` 或任意兼容模型）
- Alertmanager（Prometheus 生态的标准告警组件）

### 1. 拉模型

```bash
ollama pull qwen2.5:7b
```

### 2. 配置环境变量

```bash
# MySQL DSN（格式: user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=true）
export MYSQL_DSN="root:password@tcp(127.0.0.1:3306)/alert?charset=utf8mb4&parseTime=true&loc=Local"

# 企业微信机器人 Webhook URL（可选）
export WEIXIN_WEBHOOK_URL="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
```

也可以直接通过命令行参数传入（优先级高于环境变量）：

```bash
./alert-webhook -dsn "root:password@tcp(127.0.0.1:3306)/alert?..." -weixin "https://qyapi.weixin.qq.com/..."
```

### 3. 编译运行

```bash
go build -o alert-webhook .
./alert-webhook -port 9091 -url http://localhost:11434 -model qwen2.5:7b
```

### 4. 配置 Alertmanager

在 `alertmanager.yml` 中添加 webhook receiver：

```yaml
receivers:
  - name: 'aiops-webhook'
    webhook_configs:
      - url: 'http://<webhook-host>:9091/webhook'
        send_resolved: true
```

---

## 命令行参数

| 参数 | 默认值 | 环境变量 | 说明 |
|------|--------|----------|------|
| `-port` | `9091` | — | Webhook 监听端口 |
| `-url` | `http://localhost:11434` | — | Ollama 地址 |
| `-model` | `qwen2.5:7b` | — | Ollama 模型名 |
| `-dsn` | `""` | `MYSQL_DSN` | MySQL 连接串，空则跳过 DB |
| `-weixin` | `""` | `WEIXIN_WEBHOOK_URL` | 企业微信 Webhook URL，空则不发通知 |
| `-frontend` | `./frontend-dist` | — | 前端静态文件目录 |
| `-log` | `""` | — | 日志文件路径（同时输出 stdout） |

---

## API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/webhook` | Alertmanager webhook 接收 |
| `GET` | `/health` | 健康检查 |
| `GET` | `/metrics` | Prometheus 指标 |
| `GET` | `/api/stats` | 仪表盘统计数据 |
| `GET` | `/api/alerts` | 告警列表（支持 `status`/`severity`/`category`/`page`/`size` 过滤） |
| `GET` | `/api/alerts/{id}` | 告警详情（含分析结果 + 通知记录） |
| `GET` | `/` | Web UI 仪表盘 |

---

## 数据库表结构

数据库名：`alert`，包含三张核心表：

- **alerts** — 告警记录（fingerprint 去重 upsert）
- **analyses** — AI 分析结果（关联 alert_id，级联删除）
- **notifications** — 通知记录（关联 alert_id，记录企业微信推送状态）

---

## Prometheus 指标

| 指标 | 类型 | 说明 |
|------|------|------|
| `aiops_webhook_uptime_seconds` | gauge | 服务运行时长 |
| `aiops_alerts_received_total` | counter | 收到的告警总数 |
| `aiops_alerts_analyzed_total` | counter | AI 分析完成数 |
| `aiops_analysis_errors_total` | counter | AI 分析失败数 |
| `aiops_analysis_duration_seconds` | gauge | 最近一次分析耗时 |
| `aiops_analysis_by_category` | counter | 按 category × severity 分类统计 |

---

## 相关项目

- [inference-gateway](https://github.com/dalemei/inference-gateway) — Go 推理网关，多后端负载均衡 + SSE 流式代理
- [SRE 用 Ollama+Go 搭建 AI 告警分析管道](https://github.com/dalemei/alert-webhook/blob/main/SRE%E7%94%A8Ollama%2BGo%E6%90%AD%E5%BB%BAAI%E5%91%8A%E8%AD%A6%E5%88%86%E6%9E%90%E7%AE%A1%E9%81%93.md) — 配套博客文章

---

## License

MIT
