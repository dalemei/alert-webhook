package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const analysisPrompt = `你是一个资深的 SRE/AIOps 根因分析专家。用户会给你一条 Prometheus 告警，请完成：
1. 归类（类别 + 严重等级）
2. 分析最可能的根因（2-3条，按可能性从高到低排列）
3. 给出建议的排查/修复步骤（2-3条可执行的操作）

要求：
1. 只输出一个 JSON 对象，不要任何多余文字，不要 markdown 代码块。
2. 字段定义：
   - "category"：从以下枚举选一：磁盘存储 / CPU内存 / 网络 / 应用错误 / 数据库 / 中间件 / 安全 / 其他
   - "severity"：critical / warning / info
   - "summary"：一句话概括这个告警（中文，不超过40字）
   - "root_causes"：字符串数组，2-3条最可能的根因，每条不超过30字
   - "actions"：字符串数组，2-3条可执行的建议操作，每条不超过30字
3. 类别边界：
   - 磁盘存储：仅宿主机磁盘使用率/空间超标
   - CPU内存：仅宿主机 load/mem%/disk% 指标超标。进程内 OOM/heap → 应用错误
   - 应用错误：进程内异常(OutOfMemoryError/panic/exception/5xx/崩溃重启)，带 PID/服务名
   - 数据库：MySQL/PG/Oracle/Mongo 慢查询/死锁/连接满
   - 中间件：Redis/Kafka/RabbitMQ/Nginx/ES 连接/拒绝/集群问题
   - 安全：认证失败/爆破/越权/扫描
   - 网络：连通性/延迟/丢包/DNS/端口不通
4. 示例输出：
{"category":"CPU内存","severity":"critical","summary":"db-master-01 CPU负载18.5，超过阈值8.0已达5分钟","root_causes":["存在慢查询或全表扫描拖高CPU","业务高峰期并发请求激增","节点上其他容器争抢CPU资源"],"actions":["登录节点执行 top 查看 Top 进程","检查数据库慢查询日志确认瓶颈SQL","若为周期性突发考虑临时扩容或限流"]}`

func analyzeWithOllama(alertText, model, baseURL string) (analysisResult, string, error) {
	reqBody := ollamaReq{
		Model:       model,
		Stream:      false,
		Format:      "json",
		Temperature: 0,
		Messages: []chatMsg{
			{Role: "system", Content: analysisPrompt},
			{Role: "user", Content: alertText},
		},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return analysisResult{}, "", fmt.Errorf("序列化请求失败: %w", err)
	}
	req, err := http.NewRequest("POST", baseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return analysisResult{}, "", fmt.Errorf("创建 HTTP 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return analysisResult{}, "", fmt.Errorf("Ollama 请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return analysisResult{}, "", fmt.Errorf("读取 Ollama 响应失败: %w", err)
	}

	var or ollamaResp
	if err := json.Unmarshal(body, &or); err != nil {
		return analysisResult{}, "", fmt.Errorf("解析 Ollama 响应失败: %s, 原始: %s", err, string(body))
	}

	rawContent := or.Message.Content
	var ar analysisResult
	if err := json.Unmarshal([]byte(rawContent), &ar); err != nil {
		return analysisResult{
			Category: "解析失败",
			Severity: "info",
			Summary:  rawContent,
		}, rawContent, nil
	}
	return ar, rawContent, nil
}

func formatAlert(a alertItem) string {
	alertName := a.Labels["alertname"]
	if alertName == "" {
		alertName = "未知告警"
	}
	severity := a.Labels["severity"]
	instance := a.Labels["instance"]
	job := a.Labels["job"]

	desc := a.Annotations["description"]
	if desc == "" {
		desc = a.Annotations["summary"]
	}
	if desc == "" {
		desc = "无描述"
	}

	return fmt.Sprintf("[%s][%s] | 实例: %s | 任务: %s\n描述: %s\n状态: %s | 开始时间: %s",
		severity, alertName, instance, job, desc, a.Status, a.StartsAt)
}
