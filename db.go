package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

func initDB(dsn string) error {
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("打开数据库连接失败: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return fmt.Errorf("数据库 ping 失败: %w", err)
	}

	if err := createTables(); err != nil {
		return fmt.Errorf("创建表失败: %w", err)
	}

	log.Println("[DB] 数据库连接成功，表结构就绪")
	return nil
}

func createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS alerts (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		fingerprint VARCHAR(64) NOT NULL,
		alertname VARCHAR(255) NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'firing',
		severity VARCHAR(20) DEFAULT '',
		instance VARCHAR(255) DEFAULT '',
		job VARCHAR(255) DEFAULT '',
		summary TEXT,
		description TEXT,
		starts_at VARCHAR(64) DEFAULT '',
		ends_at VARCHAR(64) DEFAULT '',
		generator_url TEXT,
		raw_json LONGTEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_fingerprint (fingerprint),
		INDEX idx_status (status),
		INDEX idx_severity (severity),
		INDEX idx_created_at (created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

	CREATE TABLE IF NOT EXISTS analyses (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		alert_id BIGINT NOT NULL,
		category VARCHAR(50) DEFAULT '',
		severity VARCHAR(20) DEFAULT '',
		summary TEXT,
		root_causes TEXT,
		actions TEXT,
		duration_ms BIGINT DEFAULT 0,
		raw_response LONGTEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_alert_id (alert_id),
		INDEX idx_category (category),
		FOREIGN KEY (alert_id) REFERENCES alerts(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

	CREATE TABLE IF NOT EXISTS notifications (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		alert_id BIGINT NOT NULL,
		channel VARCHAR(50) NOT NULL DEFAULT 'weixin',
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		error_msg TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_alert_id (alert_id),
		FOREIGN KEY (alert_id) REFERENCES alerts(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`

	for _, stmt := range strings.Split(schema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("执行 SQL 失败: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}

// ========== Alert CRUD ==========

func insertAlert(a alertItem, rawJSON string) (int64, error) {
	// 先查是否已有同 fingerprint 的记录
	var existingID int64
	err := db.QueryRow("SELECT id FROM alerts WHERE fingerprint = ? LIMIT 1", a.Fingerprint).Scan(&existingID)

	if err == nil {
		// 已存在，更新
		_, err = db.Exec(`
			UPDATE alerts SET status=?, severity=?, summary=?, description=?, ends_at=?, raw_json=?, updated_at=CURRENT_TIMESTAMP
			WHERE id=?`,
			a.Status,
			a.Labels["severity"],
			a.Annotations["summary"],
			a.Annotations["description"],
			a.EndsAt,
			rawJSON,
			existingID,
		)
		return existingID, err
	}

	if err != sql.ErrNoRows {
		// 查询本身失败（非"没找到记录"）
		return 0, fmt.Errorf("查询已有告警失败: %w", err)
	}

	// 不存在，插入
	result, err := db.Exec(`
		INSERT INTO alerts (fingerprint, alertname, status, severity, instance, job, summary, description, starts_at, ends_at, generator_url, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		a.Fingerprint,
		a.Labels["alertname"],
		a.Status,
		a.Labels["severity"],
		a.Labels["instance"],
		a.Labels["job"],
		a.Annotations["summary"],
		a.Annotations["description"],
		a.StartsAt,
		a.EndsAt,
		a.GeneratorURL,
		rawJSON,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func listAlerts(status, severity, category string, page, size int) (AlertListResponse, error) {
	var resp AlertListResponse
	resp.Page = page
	resp.Size = size

	where := []string{"1=1"}
	args := []interface{}{}

	if status != "" {
		where = append(where, "a.status = ?")
		args = append(args, status)
	}
	if severity != "" {
		where = append(where, "a.severity = ?")
		args = append(args, severity)
	}
	if category != "" {
		where = append(where, "an.category = ?")
		args = append(args, category)
	}

	whereClause := strings.Join(where, " AND ")

	// count
	countSQL := `SELECT COUNT(*) FROM alerts a
		LEFT JOIN analyses an ON a.id = an.alert_id
		WHERE ` + whereClause
	if err := db.QueryRow(countSQL, args...).Scan(&resp.Total); err != nil {
		return resp, err
	}

	// list
	offset := (page - 1) * size
	listSQL := `SELECT a.id, a.fingerprint, a.alertname, a.status, COALESCE(a.severity,''),
		COALESCE(a.instance,''), COALESCE(a.job,''), COALESCE(a.summary,''), COALESCE(a.description,''),
		COALESCE(a.starts_at,''), COALESCE(a.ends_at,''), COALESCE(a.generator_url,''),
		a.created_at, a.updated_at
		FROM alerts a
		LEFT JOIN analyses an ON a.id = an.alert_id
		WHERE ` + whereClause + `
		ORDER BY a.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, size, offset)

	rows, err := db.Query(listSQL, args...)
	if err != nil {
		return resp, err
	}
	defer rows.Close()

	for rows.Next() {
		var r AlertRecord
		if err := rows.Scan(&r.ID, &r.Fingerprint, &r.Alertname, &r.Status, &r.Severity,
			&r.Instance, &r.Job, &r.Summary, &r.Description,
			&r.StartsAt, &r.EndsAt, &r.GeneratorURL,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return resp, err
		}
		resp.Alerts = append(resp.Alerts, r)
	}
	return resp, nil
}

func getAlertDetail(alertID int64) (AlertDetail, error) {
	var d AlertDetail
	err := db.QueryRow(`SELECT id, fingerprint, alertname, status, COALESCE(severity,''),
		COALESCE(instance,''), COALESCE(job,''), COALESCE(summary,''), COALESCE(description,''),
		COALESCE(starts_at,''), COALESCE(ends_at,''), COALESCE(generator_url,''),
		created_at, updated_at FROM alerts WHERE id = ?`, alertID).
		Scan(&d.Alert.ID, &d.Alert.Fingerprint, &d.Alert.Alertname, &d.Alert.Status,
			&d.Alert.Severity, &d.Alert.Instance, &d.Alert.Job,
			&d.Alert.Summary, &d.Alert.Description,
			&d.Alert.StartsAt, &d.Alert.EndsAt, &d.Alert.GeneratorURL,
			&d.Alert.CreatedAt, &d.Alert.UpdatedAt)
	if err != nil {
		return d, err
	}

	// analysis
	var ar AnalysisRecord
	err = db.QueryRow(`SELECT id, alert_id, COALESCE(category,''), COALESCE(severity,''),
		COALESCE(summary,''), COALESCE(root_causes,'[]'), COALESCE(actions,'[]'),
		duration_ms, created_at FROM analyses WHERE alert_id = ? ORDER BY id DESC LIMIT 1`, alertID).
		Scan(&ar.ID, &ar.AlertID, &ar.Category, &ar.Severity,
			&ar.Summary, &ar.RootCauses, &ar.Actions,
			&ar.DurationMs, &ar.CreatedAt)
	if err == nil {
		d.Analysis = &ar
	}

	// notifications
	notifRows, err := db.Query(`SELECT id, alert_id, channel, status, COALESCE(error_msg,''), created_at
		FROM notifications WHERE alert_id = ? ORDER BY created_at DESC`, alertID)
	if err == nil {
		defer notifRows.Close()
		for notifRows.Next() {
			var n NotificationRecord
			if err := notifRows.Scan(&n.ID, &n.AlertID, &n.Channel, &n.Status, &n.ErrorMsg, &n.CreatedAt); err != nil {
				log.Printf("[DB] 扫描通知记录失败: %v", err)
				continue
			}
			d.Notifications = append(d.Notifications, n)
		}
	}

	return d, nil
}

// ========== Analysis CRUD ==========

func insertAnalysis(alertID int64, ar analysisResult, durationMs int64, rawResponse string) (int64, error) {
	rcJSON, err := jsonMarshal(ar.RootCauses)
	if err != nil {
		log.Printf("[DB] 序列化 root_causes 失败: %v", err)
		rcJSON = "[]"
	}
	actJSON, err := jsonMarshal(ar.Actions)
	if err != nil {
		log.Printf("[DB] 序列化 actions 失败: %v", err)
		actJSON = "[]"
	}

	result, err := db.Exec(`
		INSERT INTO analyses (alert_id, category, severity, summary, root_causes, actions, duration_ms, raw_response)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, alertID, ar.Category, ar.Severity, ar.Summary, rcJSON, actJSON, durationMs, rawResponse)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// ========== Notification CRUD ==========

func insertNotification(alertID int64, channel, status, errorMsg string) (int64, error) {
	result, err := db.Exec(`
		INSERT INTO notifications (alert_id, channel, status, error_msg)
		VALUES (?, ?, ?, ?)
	`, alertID, channel, status, errorMsg)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// ========== Stats ==========

func getStats() (StatsResponse, error) {
	var s StatsResponse

	// 基础统计：单条查询失败不致命，但记录日志
	if err := db.QueryRow("SELECT COUNT(*) FROM alerts").Scan(&s.TotalAlerts); err != nil {
		log.Printf("[DB] 查询告警总数失败: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM alerts WHERE status = 'firing'").Scan(&s.FiringCount); err != nil {
		log.Printf("[DB] 查询活跃告警数失败: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM alerts WHERE status = 'resolved'").Scan(&s.ResolvedCount); err != nil {
		log.Printf("[DB] 查询已恢复告警数失败: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM analyses").Scan(&s.AnalyzedCount); err != nil {
		log.Printf("[DB] 查询分析总数失败: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM alerts WHERE DATE(created_at) = CURDATE()").Scan(&s.TodayCount); err != nil {
		log.Printf("[DB] 查询今日告警数失败: %v", err)
	}
	if err := db.QueryRow("SELECT COALESCE(AVG(duration_ms), 0) FROM analyses").Scan(&s.AvgAnalysisMs); err != nil {
		log.Printf("[DB] 查询平均分析耗时失败: %v", err)
	}

	// by category
	catRows, err := db.Query(`SELECT COALESCE(category,'未分类'), COUNT(*) FROM analyses GROUP BY category ORDER BY COUNT(*) DESC LIMIT 10`)
	if err == nil {
		defer catRows.Close()
		for catRows.Next() {
			var c CategoryCount
			if err := catRows.Scan(&c.Category, &c.Count); err != nil {
				log.Printf("[DB] 扫描分类统计失败: %v", err)
				continue
			}
			s.ByCategory = append(s.ByCategory, c)
		}
	}

	// by severity
	sevRows, err := db.Query(`SELECT COALESCE(severity,'unknown'), COUNT(*) FROM alerts GROUP BY severity`)
	if err == nil {
		defer sevRows.Close()
		for sevRows.Next() {
			var sc SeverityCount
			if err := sevRows.Scan(&sc.Severity, &sc.Count); err != nil {
				log.Printf("[DB] 扫描等级统计失败: %v", err)
				continue
			}
			s.BySeverity = append(s.BySeverity, sc)
		}
	}

	return s, nil
}

// jsonMarshal is a simple wrapper for json.Marshal returning string
func jsonMarshal(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]", err
	}
	return string(b), nil
}
