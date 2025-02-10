package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type PostgresClient struct {
	conn *pgx.Conn
}

func NewPostgresClient(connString string) (*PostgresClient, error) {
	conn, err := pgx.Connect(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("[PosgreSQL] - failed to connect to Postgres: %w", err)
	}

	return &PostgresClient{conn: conn}, nil
}

func (pc *PostgresClient) Query(query string, args ...interface{}) (pgx.Rows, error) {
	return pc.conn.Query(context.Background(), query, args...)
}

func (pc *PostgresClient) Exec(query string, args ...interface{}) error {
	_, err := pc.conn.Exec(context.Background(), query, args...)
	return err
}

func (pc *PostgresClient) Close() {
	pc.conn.Close(context.Background())
}
