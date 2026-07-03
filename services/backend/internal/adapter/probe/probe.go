// Package probe holds driven adapters implementing the health.Probe port.
package probe

import (
	"context"
	"database/sql"
)

// Ready is a trivial readiness probe: a stateless service is ready as soon as
// it can serve.
type Ready struct{}

func NewReady() Ready             { return Ready{} }
func (Ready) Name() string        { return "service" }
func (Ready) Check(context.Context) error { return nil }

// DB pings the SQL database to verify connectivity.
type DB struct{ sqlDB *sql.DB }

func NewDB(sqlDB *sql.DB) DB            { return DB{sqlDB: sqlDB} }
func (DB) Name() string                 { return "database" }
func (d DB) Check(ctx context.Context) error { return d.sqlDB.PingContext(ctx) }
