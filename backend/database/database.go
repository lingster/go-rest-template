// Package database wires up the Beego ORM for SQLite (CGO-free) or PostgreSQL.
package database

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/beego/beego/v2/client/orm"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"

	"github.com/lingster/go-rest-template/backend/models"
)

var once sync.Once

// Init registers the driver, connects and creates missing tables. It may be called once per process.
func Init(driver, dsn string) error {
	var err error
	once.Do(func() { err = initDB(driver, dsn) })
	return err
}

func initDB(driver, dsn string) error {
	orm.DefaultTimeLoc = time.UTC
	var opts []orm.DBOption
	switch driver {
	case "sqlite":
		if err := orm.RegisterDriver("sqlite", orm.DRSqlite); err != nil {
			return err
		}
		dsn = sqliteDSN(dsn)
		opts = append(opts, orm.MaxOpenConnections(1)) // serialise writers; avoids SQLITE_BUSY
	case "postgres":
		if err := orm.RegisterDriver("postgres", orm.DRPostgres); err != nil {
			return err
		}
		opts = append(opts, orm.MaxOpenConnections(20), orm.MaxIdleConnections(5))
	default:
		return fmt.Errorf("unsupported DB_DRIVER %q", driver)
	}
	if err := orm.RegisterDataBase("default", driver, dsn, opts...); err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	models.Register()
	// force=false: only creates missing tables/indexes (template-grade auto-migration).
	if err := orm.RunSyncdb("default", false, false); err != nil {
		return fmt.Errorf("sync schema: %w", err)
	}
	return nil
}

// sqliteDSN makes sure the parent dir exists and adds sane pragmas.
func sqliteDSN(dsn string) string {
	path := strings.TrimPrefix(dsn, "file:")
	if i := strings.Index(path, "?"); i >= 0 {
		path = path[:i]
	}
	if path != ":memory:" && path != "" && !strings.HasPrefix(path, "file::memory:") {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
}
