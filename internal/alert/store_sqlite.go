package alert

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// sqliteRuleStore 把规则存在 SQLite 的 alert_rules 表（与指标同库不同表）。
type sqliteRuleStore struct {
	db *sql.DB
}

// NewSQLiteRuleStore 打开规则存储。path 与指标库相同（同文件不同表），
// 或独立路径；:memory: 仅测试用。
func NewSQLiteRuleStore(path string) (RuleStore, error) {
	memory := path == ":memory:"
	dsn := path
	if !memory {
		dsn = "file:" + path +
			"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if memory {
		db.SetMaxOpenConns(1)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS alert_rules (
		id TEXT PRIMARY KEY, name TEXT, kind TEXT, target TEXT,
		metric TEXT, cmp TEXT, threshold REAL, for_sec INTEGER, enabled INTEGER)`); err != nil {
		db.Close()
		return nil, err
	}
	return &sqliteRuleStore{db: db}, nil
}

func (s *sqliteRuleStore) List(ctx context.Context) ([]AlertRule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,name,kind,target,metric,cmp,threshold,for_sec,enabled FROM alert_rules ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AlertRule
	for rows.Next() {
		var r AlertRule
		var cmp string
		var enabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.Kind, &r.Target, &r.Metric, &cmp, &r.Threshold, &r.ForSec, &enabled); err != nil {
			return nil, err
		}
		r.Cmp = Comparator(cmp)
		r.Enabled = enabled != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *sqliteRuleStore) Upsert(ctx context.Context, r AlertRule) error {
	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO alert_rules (id,name,kind,target,metric,cmp,threshold,for_sec,enabled)
		 VALUES (?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET
		   name=excluded.name, kind=excluded.kind, target=excluded.target,
		   metric=excluded.metric, cmp=excluded.cmp, threshold=excluded.threshold,
		   for_sec=excluded.for_sec, enabled=excluded.enabled`,
		r.ID, r.Name, r.Kind, r.Target, r.Metric, string(r.Cmp), r.Threshold, r.ForSec, enabled)
	return err
}

func (s *sqliteRuleStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM alert_rules WHERE id=?`, id)
	return err
}

func (s *sqliteRuleStore) Close() error { return s.db.Close() }
