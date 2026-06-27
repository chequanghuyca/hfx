//go:build ignore

package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type tableInfo struct {
	Name string
	Rows int64
}

type sequenceInfo struct {
	Name       string
	DataType   string
	StartValue int64
	MinValue   int64
	MaxValue   int64
	Increment  int64
	CacheSize  int64
	Cycle      bool
	LastValue  sql.NullInt64
}

type columnDefaultInfo struct {
	Table   string
	Column  string
	Default string
}

func main() {
	source := flag.String("source", "nofx_bot_v2", "source PostgreSQL schema")
	dest := flag.String("dest", "hfx_clean", "destination PostgreSQL schema")
	inspectOnly := flag.Bool("inspect", false, "only inspect source/destination schemas")
	disableRunning := flag.Bool("disable-running", false, "set copied traders.is_running=false after cloning")
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Fatalf("failed to load .env: %v", err)
	}
	mustValidateSchema(*source)
	mustValidateSchema(*dest)

	db, err := sql.Open("postgres", postgresURLFromEnv())
	if err != nil {
		log.Fatalf("failed to open postgres connection: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to connect to postgres: %v", err)
	}

	if *inspectOnly {
		if err := inspectSchema(db, *source); err != nil {
			log.Fatalf("inspect source failed: %v", err)
		}
		if *source != *dest {
			if err := inspectSchema(db, *dest); err != nil {
				log.Printf("inspect destination failed: %v", err)
			}
		}
		return
	}

	backupSchema, copied, err := cloneSchema(db, *source, *dest)
	if err != nil {
		log.Fatalf("clone failed: %v", err)
	}
	if backupSchema != "" {
		log.Printf("backed up previous destination schema: %s", backupSchema)
	}
	for _, t := range copied {
		log.Printf("copied table %-32s rows=%d", t.Name, t.Rows)
	}

	if *disableRunning {
		changed, err := disableRunningTraders(db, *dest)
		if err != nil {
			log.Fatalf("failed to disable running traders: %v", err)
		}
		log.Printf("disabled running traders in destination: %d", changed)
	}

	if err := warnAboutSequenceDefaults(db, *dest); err != nil {
		log.Printf("sequence default check failed: %v", err)
	}
	if err := inspectSchema(db, *dest); err != nil {
		log.Fatalf("inspect destination failed: %v", err)
	}
}

func postgresURLFromEnv() string {
	host := mustEnv("DB_HOST")
	port := getenv("DB_PORT", "5432")
	user := mustEnv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbName := getenv("DB_NAME", "postgres")
	sslMode := getenv("DB_SSLMODE", "require")

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, port),
		Path:   dbName,
	}
	q := u.Query()
	q.Set("sslmode", sslMode)
	u.RawQuery = q.Encode()
	return u.String()
}

func mustEnv(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		log.Fatalf("%s is required", key)
	}
	return value
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func mustValidateSchema(name string) {
	if !identifierPattern.MatchString(name) {
		log.Fatalf("invalid schema name: %s", name)
	}
}

func cloneSchema(db *sql.DB, source, dest string) (string, []tableInfo, error) {
	tx, err := db.Begin()
	if err != nil {
		return "", nil, err
	}
	defer tx.Rollback()

	exists, err := schemaExists(tx, source)
	if err != nil {
		return "", nil, err
	}
	if !exists {
		return "", nil, fmt.Errorf("source schema %s does not exist", source)
	}

	var backupName string
	destExists, err := schemaExists(tx, dest)
	if err != nil {
		return "", nil, err
	}
	if destExists {
		backupName = backupSchemaName(dest)
		if _, err := tx.Exec(fmt.Sprintf(`ALTER SCHEMA %s RENAME TO %s`, quoteIdent(dest), quoteIdent(backupName))); err != nil {
			return "", nil, fmt.Errorf("backup destination schema: %w", err)
		}
	}

	if _, err := tx.Exec(fmt.Sprintf(`CREATE SCHEMA %s`, quoteIdent(dest))); err != nil {
		return "", nil, fmt.Errorf("create destination schema: %w", err)
	}

	if err := cloneSequences(tx, source, dest); err != nil {
		return "", nil, err
	}

	tables, err := listTables(tx, source)
	if err != nil {
		return "", nil, err
	}
	copied := make([]tableInfo, 0, len(tables))
	for _, table := range tables {
		createSQL := fmt.Sprintf(
			`CREATE TABLE %s (LIKE %s INCLUDING ALL)`,
			qualify(dest, table),
			qualify(source, table),
		)
		if _, err := tx.Exec(createSQL); err != nil {
			return "", nil, fmt.Errorf("create table %s: %w", table, err)
		}

		insertSQL := fmt.Sprintf(`INSERT INTO %s SELECT * FROM %s`, qualify(dest, table), qualify(source, table))
		result, err := tx.Exec(insertSQL)
		if err != nil {
			return "", nil, fmt.Errorf("copy table %s: %w", table, err)
		}
		rows, _ := result.RowsAffected()
		copied = append(copied, tableInfo{Name: table, Rows: rows})
	}

	if err := rewriteSequenceDefaults(tx, source, dest); err != nil {
		return "", nil, err
	}

	if err := tx.Commit(); err != nil {
		return "", nil, err
	}
	return backupName, copied, nil
}

type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

func schemaExists(q querier, schema string) (bool, error) {
	var exists bool
	err := q.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)`, schema).Scan(&exists)
	return exists, err
}

func listTables(q querier, schema string) ([]string, error) {
	rows, err := q.Query(`
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind IN ('r', 'p')
		ORDER BY c.relname
	`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

func cloneSequences(q querier, source, dest string) error {
	sequences, err := listSequences(q, source)
	if err != nil {
		return err
	}
	for _, seq := range sequences {
		createSQL := fmt.Sprintf(
			`CREATE SEQUENCE %s AS %s INCREMENT BY %d MINVALUE %d MAXVALUE %d START WITH %d CACHE %d %s`,
			qualify(dest, seq.Name),
			seq.DataType,
			seq.Increment,
			seq.MinValue,
			seq.MaxValue,
			seq.StartValue,
			seq.CacheSize,
			cycleClause(seq.Cycle),
		)
		if _, err := q.Exec(createSQL); err != nil {
			return fmt.Errorf("create sequence %s: %w", seq.Name, err)
		}
		if seq.LastValue.Valid {
			if _, err := q.Exec(`SELECT setval($1::regclass, $2, true)`, regclassName(dest, seq.Name), seq.LastValue.Int64); err != nil {
				return fmt.Errorf("set sequence %s value: %w", seq.Name, err)
			}
		}
	}
	return nil
}

func listSequences(q querier, schema string) ([]sequenceInfo, error) {
	rows, err := q.Query(`
		SELECT sequencename, data_type, start_value, min_value, max_value,
		       increment_by, cache_size, cycle, last_value
		FROM pg_sequences
		WHERE schemaname = $1
		ORDER BY sequencename
	`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sequences []sequenceInfo
	for rows.Next() {
		var seq sequenceInfo
		if err := rows.Scan(
			&seq.Name,
			&seq.DataType,
			&seq.StartValue,
			&seq.MinValue,
			&seq.MaxValue,
			&seq.Increment,
			&seq.CacheSize,
			&seq.Cycle,
			&seq.LastValue,
		); err != nil {
			return nil, err
		}
		sequences = append(sequences, seq)
	}
	return sequences, rows.Err()
}

func rewriteSequenceDefaults(q querier, source, dest string) error {
	rows, err := q.Query(`
		SELECT table_name, column_name, column_default
		FROM information_schema.columns
		WHERE table_schema = $1 AND column_default LIKE '%nextval%'
		ORDER BY table_name, column_name
	`, source)
	if err != nil {
		return err
	}
	defer rows.Close()

	var defaults []columnDefaultInfo
	for rows.Next() {
		var table, column, def string
		if err := rows.Scan(&table, &column, &def); err != nil {
			return err
		}
		defaults = append(defaults, columnDefaultInfo{Table: table, Column: column, Default: def})
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range defaults {
		seq, ok := sequenceNameFromDefault(item.Default)
		if !ok {
			return fmt.Errorf("could not parse sequence default for %s.%s: %s", item.Table, item.Column, item.Default)
		}
		defaultExpr := fmt.Sprintf(`nextval('%s'::regclass)`, regclassName(dest, seq))
		alterDefault := fmt.Sprintf(
			`ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s`,
			qualify(dest, item.Table),
			quoteIdent(item.Column),
			defaultExpr,
		)
		if _, err := q.Exec(alterDefault); err != nil {
			return fmt.Errorf("rewrite default for %s.%s: %w", item.Table, item.Column, err)
		}
		ownedBy := fmt.Sprintf(
			`ALTER SEQUENCE %s OWNED BY %s.%s`,
			qualify(dest, seq),
			qualify(dest, item.Table),
			quoteIdent(item.Column),
		)
		if _, err := q.Exec(ownedBy); err != nil {
			return fmt.Errorf("set sequence owner for %s: %w", seq, err)
		}
	}
	return nil
}

func inspectSchema(db *sql.DB, schema string) error {
	exists, err := schemaExists(db, schema)
	if err != nil {
		return err
	}
	if !exists {
		log.Printf("schema %s does not exist", schema)
		return nil
	}

	tables, err := listTables(db, schema)
	if err != nil {
		return err
	}

	infos := make([]tableInfo, 0, len(tables))
	for _, table := range tables {
		count, err := tableCount(db, schema, table)
		if err != nil {
			return err
		}
		infos = append(infos, tableInfo{Name: table, Rows: count})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })

	log.Printf("schema %s tables=%d", schema, len(infos))
	for _, info := range infos {
		log.Printf("  %-32s rows=%d", info.Name, info.Rows)
	}

	if contains(tables, "users") {
		count, err := tableCount(db, schema, "users")
		if err != nil {
			return err
		}
		log.Printf("schema %s users=%d", schema, count)
	}
	if contains(tables, "traders") {
		var total, running int64
		if err := db.QueryRow(fmt.Sprintf(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN is_running THEN 1 ELSE 0 END), 0) FROM %s`, qualify(schema, "traders"))).Scan(&total, &running); err != nil {
			return err
		}
		log.Printf("schema %s traders=%d running=%d", schema, total, running)
	}
	return nil
}

func tableCount(db *sql.DB, schema, table string) (int64, error) {
	var count int64
	err := db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, qualify(schema, table))).Scan(&count)
	return count, err
}

func disableRunningTraders(db *sql.DB, schema string) (int64, error) {
	result, err := db.Exec(fmt.Sprintf(`UPDATE %s SET is_running = FALSE WHERE is_running = TRUE`, qualify(schema, "traders")))
	if err != nil {
		return 0, err
	}
	changed, _ := result.RowsAffected()
	return changed, nil
}

func warnAboutSequenceDefaults(db *sql.DB, schema string) error {
	rows, err := db.Query(`
		SELECT table_name, column_name, column_default
		FROM information_schema.columns
		WHERE table_schema = $1 AND column_default LIKE '%nextval%'
		ORDER BY table_name, column_name
	`, schema)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var table, column, def string
		if err := rows.Scan(&table, &column, &def); err != nil {
			return err
		}
		if strings.Contains(def, schema+".") || strings.Contains(def, quoteIdent(schema)+".") {
			log.Printf("sequence default ok: %s.%s -> %s", table, column, def)
		} else {
			log.Printf("warning: %s.%s has external sequence default: %s", table, column, def)
		}
	}
	return rows.Err()
}

func backupSchemaName(dest string) string {
	stamp := time.Now().UTC().Format("20060102_150405")
	name := dest + "_precopy_" + stamp
	if len(name) <= 63 {
		return name
	}
	return name[:63]
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func regclassName(schema, name string) string {
	return strings.ReplaceAll(quoteIdent(schema)+"."+quoteIdent(name), `'`, `''`)
}

func qualify(schema, table string) string {
	return quoteIdent(schema) + "." + quoteIdent(table)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func cycleClause(cycle bool) string {
	if cycle {
		return "CYCLE"
	}
	return "NO CYCLE"
}

func sequenceNameFromDefault(def string) (string, bool) {
	start := strings.Index(def, "nextval('")
	if start < 0 {
		return "", false
	}
	rest := def[start+len("nextval('"):]
	end := strings.Index(rest, "'::regclass")
	if end < 0 {
		return "", false
	}
	qualified := strings.Trim(rest[:end], `"`)
	parts := strings.Split(qualified, ".")
	if len(parts) == 0 {
		return "", false
	}
	seq := strings.Trim(parts[len(parts)-1], `"`)
	return seq, seq != ""
}
