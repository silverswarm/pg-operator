package postgres

import (
	"context"
	"database/sql"
	"fmt"

	postgresv1 "github.com/silverswarm/pg-operator/api/v1"
)

type DatabaseService struct {
	client *Client
}

func NewDatabaseService(client *Client) *DatabaseService {
	return &DatabaseService{
		client: client,
	}
}

func (s *DatabaseService) EnsureDatabase(ctx context.Context, db *sql.DB, database *postgresv1.Database) (bool, error) {
	exists, err := s.databaseExists(ctx, db, database.Spec.DatabaseName)
	if err != nil {
		return false, fmt.Errorf("failed to check if database exists: %w", err)
	}

	if exists {
		return true, nil
	}

	if err := s.createDatabase(ctx, db, database); err != nil {
		return false, fmt.Errorf("failed to create database: %w", err)
	}

	return true, nil
}

func (s *DatabaseService) databaseExists(ctx context.Context, db *sql.DB, databaseName string) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
	err := db.QueryRowContext(ctx, query, databaseName).Scan(&exists)
	return exists, err
}

func (s *DatabaseService) createDatabase(ctx context.Context, db *sql.DB, database *postgresv1.Database) error {
	owner := database.Spec.Owner
	if owner == "" {
		owner = "postgres"
	}

	encoding := database.Spec.Encoding
	if encoding == "" {
		encoding = "UTF8"
	}

	createQuery := fmt.Sprintf("CREATE DATABASE %s WITH OWNER %s ENCODING '%s'",
		database.Spec.DatabaseName, owner, encoding)

	_, err := db.ExecContext(ctx, createQuery)
	return err
}
