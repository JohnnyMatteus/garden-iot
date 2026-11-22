// Package postgres implementa as portas de persistência sobre o TimescaleDB.
// Requisições de usuário rodam como o papel tenant_<id> (SET LOCAL ROLE), então o RLS (ADR 0010)
// isola os tenants mesmo que uma consulta esqueça o filtro.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

type DB struct{ pool *pgxpool.Pool }

func Connect(ctx context.Context, dsn string) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("banco indisponível: %w", err)
	}
	return &DB{pool: pool}, nil
}

func (d *DB) Close() { d.pool.Close() }

// asTenant executa fn numa transação com o papel do tenant.
func (d *DB) asTenant(ctx context.Context, tenant string, fn func(pgx.Tx) error) error {
	if !domain.ValidIdentifier(tenant) {
		return fmt.Errorf("%w: tenant", domain.ErrInvalidInput)
	}
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+pgx.Identifier{"tenant_" + tenant}.Sanitize()); err != nil {
		return fmt.Errorf("assumir papel do tenant: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// mapErr converte erros do Postgres em erros de domínio.
func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			return domain.ErrConflict
		case "23503":
			return domain.ErrNotFound // horta, canteiro ou nó referenciado não existe neste tenant
		case "42501":
			return domain.ErrForbidden
		}
	}
	return err
}
