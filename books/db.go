package main

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InitDB(parentContext context.Context, dbUrl string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(parentContext, 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbUrl)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return pool, nil
}
