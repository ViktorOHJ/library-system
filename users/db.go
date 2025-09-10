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

func InitDB(parentContext context.Context, dbUrl string, logger *logrus.Logger) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(parentContext, 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbUrl)
	if err != nil {
		logger.Errorf("Error to create pool: %v", err)
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		logger.Errorf("Error to connect db: %v", err)
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
		logger.Fatalf("Error of creating migrations: %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		logger.Fatal("Migrations failed: ", err)
	}
}
