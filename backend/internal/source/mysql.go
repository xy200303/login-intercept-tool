package source

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var (
	poolMu sync.Mutex
	pools  = map[string]*sql.DB{}
)

// pooledDB 按 DSN 缓存连接池，配置保存后由 InvalidatePool 失效重建。
func pooledDB(dsn string) (*sql.DB, error) {
	poolMu.Lock()
	defer poolMu.Unlock()
	if db, ok := pools[dsn]; ok {
		return db, nil
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	pools[dsn] = db
	return db, nil
}

// InvalidatePool 关闭并移除指定 DSN 的连接池。
func InvalidatePool(dsn string) {
	if dsn == "" {
		return
	}
	poolMu.Lock()
	defer poolMu.Unlock()
	if db, ok := pools[dsn]; ok {
		_ = db.Close()
		delete(pools, dsn)
	}
}

type columnsCacheEntry struct {
	columns []string
	expires time.Time
}

var columnsCache sync.Map // key: dsn + "\x00" + table

const columnsCacheTTL = 30 * time.Minute

// TableColumnsCached 按 DSN+表名缓存表结构探测结果（TTL 30 分钟）；
// 只缓存成功结果，查询出错由调用方 InvalidateTableColumns 后重试。
func (s MySQLSource) TableColumnsCached(ctx context.Context, table string) ([]string, error) {
	key := s.DSN + "\x00" + table
	if value, ok := columnsCache.Load(key); ok {
		entry := value.(columnsCacheEntry)
		if time.Now().Before(entry.expires) {
			return entry.columns, nil
		}
		columnsCache.Delete(key)
	}
	columns, err := s.TableColumns(ctx, table)
	if err != nil {
		return nil, err
	}
	columnsCache.Store(key, columnsCacheEntry{columns, time.Now().Add(columnsCacheTTL)})
	return columns, nil
}

// InvalidateTableColumns 失效指定 DSN+表名 的元数据缓存（schema 变更或 DSN 切换后调用）。
func InvalidateTableColumns(dsn, table string) {
	columnsCache.Delete(dsn + "\x00" + table)
}

type MySQLSource struct {
	Name string
	DSN  string
}

func (s MySQLSource) QueryRows(ctx context.Context, table, agentColumn string, agentValues []string, limit int) ([]map[string]any, error) {
	if !validIdentifier(table) || !validIdentifier(agentColumn) || len(agentValues) == 0 {
		return nil, fmt.Errorf("invalid query parameters")
	}
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	db, err := pooledDB(s.DSN)
	if err != nil {
		return nil, err
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(agentValues)), ",")
	args := make([]any, len(agentValues))
	for i, value := range agentValues {
		args[i] = value
	}
	query := "SELECT * FROM `" + table + "` WHERE `" + agentColumn + "` IN (" + placeholders + ") LIMIT ?"
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		item := make(map[string]any, len(columns))
		for i, column := range columns {
			item[column] = normalizeValue(values[i])
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s MySQLSource) QueryIdentityRows(ctx context.Context, table, mobile, qq, email string, limit int) ([]map[string]any, error) {
	if !validIdentifier(table) || limit <= 0 {
		return nil, fmt.Errorf("invalid query parameters")
	}
	columns, err := s.TableColumnsCached(ctx, table)
	if err != nil {
		return nil, err
	}
	aliases := []struct {
		names []string
		value string
	}{
		{[]string{"mobile", "phone", "tel", "telephone"}, mobile},
		{[]string{"qq", "qq_num"}, qq},
		{[]string{"email", "mail"}, email},
	}
	conditions := make([]string, 0, len(aliases))
	args := make([]any, 0, len(aliases))
	for _, alias := range aliases {
		if strings.TrimSpace(alias.value) == "" {
			continue
		}
		for _, column := range columns {
			for _, name := range alias.names {
				if strings.EqualFold(column, name) {
					conditions = append(conditions, "`"+column+"` = ?")
					args = append(args, alias.value)
					break
				}
			}
		}
	}
	if len(conditions) == 0 {
		return []map[string]any{}, nil
	}
	if limit > 100 {
		limit = 100
	}
	db, err := pooledDB(s.DSN)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, "SELECT * FROM `"+table+"` WHERE "+strings.Join(conditions, " OR ")+" LIMIT ?", append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	resultColumns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(resultColumns))
		pointers := make([]any, len(resultColumns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		item := make(map[string]any, len(resultColumns))
		for i, column := range resultColumns {
			item[column] = normalizeValue(values[i])
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func normalizeValue(value any) any {
	if bytes, ok := value.([]byte); ok {
		return string(bytes)
	}
	return value
}
func MarshalRow(row map[string]any) ([]byte, error) { return json.Marshal(row) }

// QueryMaps runs a fully parameterized SELECT built by the caller and returns
// rows as column-name keyed maps. Identifiers inside the query must already be
// validated by the caller (see ValidIdentifier).
func (s MySQLSource) QueryMaps(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	db, err := pooledDB(s.DSN)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

// Exec runs a parameterized write statement and returns affected row count.
func (s MySQLSource) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	db, err := pooledDB(s.DSN)
	if err != nil {
		return 0, err
	}
	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Insert runs a parameterized INSERT and returns the last insert id.
func (s MySQLSource) Insert(ctx context.Context, query string, args ...any) (int64, error) {
	db, err := pooledDB(s.DSN)
	if err != nil {
		return 0, err
	}
	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// WithTx runs fn inside a single transaction on the pooled connection.
func (s MySQLSource) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	db, err := pooledDB(s.DSN)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func scanRows(rows *sql.Rows) ([]map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		item := make(map[string]any, len(columns))
		for i, column := range columns {
			item[column] = normalizeValue(values[i])
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// Test 使用一次性连接（不进入连接池缓存），用于连接测试任意表单配置。
func (s MySQLSource) Test(ctx context.Context) error {
	if strings.TrimSpace(s.DSN) == "" {
		return fmt.Errorf("%s dsn is not configured", s.Name)
	}
	db, err := sql.Open("mysql", s.DSN)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.PingContext(ctx)
}

func (s MySQLSource) TableColumns(ctx context.Context, table string) ([]string, error) {
	if !validIdentifier(table) {
		return nil, fmt.Errorf("invalid table name")
	}
	db, err := pooledDB(s.DSN)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, "SHOW COLUMNS FROM `"+table+"`")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var field, columnType, nullable, key, extra sql.NullString
		var defaultValue any
		if err := rows.Scan(&field, &columnType, &nullable, &key, &defaultValue, &extra); err != nil {
			return nil, err
		}
		columns = append(columns, field.String)
	}
	return columns, rows.Err()
}

func validIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

// ValidIdentifier reports whether value is safe to interpolate as a MySQL
// table/column identifier (letters, digits, underscore only).
func ValidIdentifier(value string) bool { return validIdentifier(value) }
