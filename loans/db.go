package main

import (
	"context"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

func InitDB(parentCtx context.Context, dbURL string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, err
	}
	err = pool.Ping(ctx)
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func runMigrations(path, db string, logger *logrus.Logger) {
	m, err := migrate.New(
		path,
		db,
	)
	if err != nil {
		logger.Fatal(err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		logger.Fatal("Migrations failed: ", err)
	}
}
