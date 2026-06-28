package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动，无 CGO，能进 distroless（评审约束 2）
)

// metricColumn 把对外的 metric 名映射到表列。
var metricColumn = map[string]string{
	"cpu":    "cpu",
	"mem":    "mem_use",
	"disk":   "disk_use",
	"net_rx": "net_rx",
	"net_tx": "net_tx",
	"load1":  "load1",
}

type sqliteStore struct {
	db *sql.DB
}

// NewSQLiteStore 打开（或新建）SQLite 存储。
// path == ":memory:" 时用单连接内存库（仅测试）；否则文件库开 WAL + busy_timeout，
// 允许并发读 + 单写，并把 -wal/-shm 边车文件落在数据卷上（评审 N3）。
func NewSQLiteStore(path string) (MetricStore, error) {
	memory := path == ":memory:"
	dsn := path
	if !memory {
		// modernc 支持用 _pragma 连接参数，每条连接生效；WAL 持久化在库头。
		dsn = "file:" + path +
			"?_pragma=busy_timeout(5000)" +
			"&_pragma=journal_mode(WAL)" +
			"&_pragma=synchronous(NORMAL)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if memory {
		// 内存库每条连接是独立 DB，限单连接避免数据互相看不到。
		db.SetMaxOpenConns(1)
	}
	s := &sqliteStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *sqliteStore) migrate() error {
	const ddl = `
CREATE TABLE IF NOT EXISTS samples (
  kind TEXT, target TEXT, ts INTEGER,
  cpu REAL, mem_use INTEGER, mem_tot INTEGER,
  disk_use INTEGER, disk_tot INTEGER,
  net_rx INTEGER, net_tx INTEGER, load1 REAL
);
CREATE INDEX IF NOT EXISTS idx_samples ON samples(kind, target, ts);
-- 供告警"匹配全部"的 Targets 查询（WHERE kind=? AND ts>=?）走范围扫描。
CREATE INDEX IF NOT EXISTS idx_samples_kind_ts ON samples(kind, ts);
CREATE TABLE IF NOT EXISTS alert_rules (
  id TEXT PRIMARY KEY, name TEXT, kind TEXT, target TEXT,
  metric TEXT, cmp TEXT, threshold REAL, for_sec INTEGER, enabled INTEGER
);`
	_, err := s.db.Exec(ddl)
	return err
}

func (s *sqliteStore) Write(ctx context.Context, samples []Sample) error {
	if len(samples) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO samples
		(kind,target,ts,cpu,mem_use,mem_tot,disk_use,disk_tot,net_rx,net_tx,load1)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, sm := range samples {
		if _, err := stmt.ExecContext(ctx, string(sm.Kind), sm.Target, sm.TS.Unix(),
			sm.CPU, sm.MemUse, sm.MemTot, sm.DiskUse, sm.DiskTot,
			sm.NetRx, sm.NetTx, sm.Load1); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// counterMetrics 是累计计数器（查询时换算成速率，字节/秒）。
var counterMetrics = map[string]bool{"net_rx": true, "net_tx": true}

func (s *sqliteStore) Query(ctx context.Context, kind TargetKind, target, metric string, from, to time.Time, stepSec int) ([]Point, error) {
	col, ok := metricColumn[metric]
	if !ok {
		return nil, fmt.Errorf("unknown metric %q", metric)
	}
	// 计数器分桶取桶内最大值（≈桶末累计值），网关/速率类才有意义；其余取均值。
	agg := "AVG"
	if counterMetrics[metric] {
		agg = "MAX"
	}
	// col / agg / 算术拼接均来自白名单常量/整数，无注入。
	var q string
	if stepSec > 0 {
		q = fmt.Sprintf(`SELECT (ts/%d)*%d AS bucket, %s(%s) FROM samples
			WHERE kind=? AND target=? AND ts BETWEEN ? AND ?
			GROUP BY bucket ORDER BY bucket`, stepSec, stepSec, agg, col)
	} else {
		q = fmt.Sprintf(`SELECT ts, %s FROM samples
			WHERE kind=? AND target=? AND ts BETWEEN ? AND ? ORDER BY ts`, col)
	}
	rows, err := s.db.QueryContext(ctx, q, string(kind), target, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Point
	for rows.Next() {
		var ts int64
		var v float64
		if err := rows.Scan(&ts, &v); err != nil {
			return nil, err
		}
		out = append(out, Point{TS: time.Unix(ts, 0), Value: v})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if counterMetrics[metric] {
		out = toRate(out) // 累计计数器 → 速率（字节/秒）
	}
	return out, nil
}

// toRate 把累计计数器序列换算成相邻两点的速率（每秒增量）；
// 计数器回绕/重启（增量为负）记 0，避免出现负的尖刺。
func toRate(cum []Point) []Point {
	if len(cum) < 2 {
		return []Point{}
	}
	out := make([]Point, 0, len(cum)-1)
	for i := 1; i < len(cum); i++ {
		dt := cum[i].TS.Sub(cum[i-1].TS).Seconds()
		dv := cum[i].Value - cum[i-1].Value
		r := 0.0
		if dt > 0 && dv >= 0 {
			r = dv / dt
		}
		out = append(out, Point{TS: cum[i].TS, Value: r})
	}
	return out
}

func (s *sqliteStore) Latest(ctx context.Context, kind TargetKind, target string) (Sample, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT kind,target,ts,cpu,mem_use,mem_tot,disk_use,disk_tot,net_rx,net_tx,load1
		FROM samples WHERE kind=? AND target=? ORDER BY ts DESC LIMIT 1`, string(kind), target)
	var sm Sample
	var k string
	var ts int64
	err := row.Scan(&k, &sm.Target, &ts, &sm.CPU, &sm.MemUse, &sm.MemTot,
		&sm.DiskUse, &sm.DiskTot, &sm.NetRx, &sm.NetTx, &sm.Load1)
	if err == sql.ErrNoRows {
		return Sample{}, false, nil
	}
	if err != nil {
		return Sample{}, false, err
	}
	sm.Kind = TargetKind(k)
	sm.TS = time.Unix(ts, 0)
	return sm, true, nil
}

func (s *sqliteStore) Targets(ctx context.Context, kind TargetKind, since time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT target FROM samples WHERE kind=? AND ts>=? ORDER BY target`,
		string(kind), since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *sqliteStore) TrafficTotal(ctx context.Context, kind TargetKind, target string, from, to time.Time) (uint64, uint64, error) {
	// 用窗口函数取相邻样本的计数器增量，只累加正增量（计数器重启时增量为负，记 0）。
	const q = `
SELECT COALESCE(SUM(CASE WHEN drx > 0 THEN drx ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN dtx > 0 THEN dtx ELSE 0 END), 0)
FROM (
  SELECT net_rx - LAG(net_rx) OVER w AS drx,
         net_tx - LAG(net_tx) OVER w AS dtx
  FROM samples
  WHERE kind=? AND target=? AND ts BETWEEN ? AND ?
  WINDOW w AS (ORDER BY ts)
)`
	var rx, tx int64
	err := s.db.QueryRowContext(ctx, q, string(kind), target, from.Unix(), to.Unix()).Scan(&rx, &tx)
	if err != nil {
		return 0, 0, err
	}
	return uint64(rx), uint64(tx), nil
}

func (s *sqliteStore) Prune(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM samples WHERE ts < ?`, cutoff.Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *sqliteStore) Close() error { return s.db.Close() }

// 确保 metric 白名单与列名不含异常字符（防御性）。
func init() {
	for _, c := range metricColumn {
		if strings.ContainsAny(c, " ;'\"") {
			panic("invalid metric column: " + c)
		}
	}
}
