package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"

	postgresv1 "github.com/silverswarm/pg-operator/api/v1"
)

type UserService struct {
	client *Client
}

func NewUserService(client *Client) *UserService {
	return &UserService{
		client: client,
	}
}

func (s *UserService) EnsureUsers(ctx context.Context, db *sql.DB, database *postgresv1.Database) ([]string, error) {
	usersCreated := make([]string, 0, len(database.Spec.Users))

	for _, user := range database.Spec.Users {
		if err := s.EnsureUser(ctx, db, user); err != nil {
			return usersCreated, fmt.Errorf("failed to ensure user %s: %w", user.Name, err)
		}
		usersCreated = append(usersCreated, user.Name)

		if err := s.GrantPermissions(ctx, db, database.Spec.DatabaseName, user); err != nil {
			return usersCreated, fmt.Errorf("failed to grant permissions to user %s: %w", user.Name, err)
		}
	}

	return usersCreated, nil
}

func (s *UserService) EnsureUser(ctx context.Context, db *sql.DB, user postgresv1.DatabaseUser) error {
	exists, err := s.userExists(ctx, db, user.Name)
	if err != nil {
		return fmt.Errorf("failed to check if user exists: %w", err)
	}

	if exists {
		return nil
	}

	password, err := s.generatePassword()
	if err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}

	createUserQuery := fmt.Sprintf("CREATE USER %s WITH ENCRYPTED PASSWORD '%s'", user.Name, password)
	if _, err := db.ExecContext(ctx, createUserQuery); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (s *UserService) GrantPermissions(ctx context.Context, db *sql.DB, databaseName string, user postgresv1.DatabaseUser) error {
	for _, permission := range user.Permissions {
		var grantQuery string
		switch permission {
		case postgresv1.PermissionAll:
			grantQuery = fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", databaseName, user.Name)
		case postgresv1.PermissionConnect:
			grantQuery = fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s", databaseName, user.Name)
		case postgresv1.PermissionCreate:
			grantQuery = fmt.Sprintf("GRANT CREATE ON DATABASE %s TO %s", databaseName, user.Name)
		case postgresv1.PermissionUsage:
			grantQuery = fmt.Sprintf("GRANT USAGE ON SCHEMA public TO %s", user.Name)
		case postgresv1.PermissionSelect:
			grantQuery = fmt.Sprintf("GRANT SELECT ON ALL TABLES IN SCHEMA public TO %s", user.Name)
		case postgresv1.PermissionInsert:
			grantQuery = fmt.Sprintf("GRANT INSERT ON ALL TABLES IN SCHEMA public TO %s", user.Name)
		case postgresv1.PermissionUpdate:
			grantQuery = fmt.Sprintf("GRANT UPDATE ON ALL TABLES IN SCHEMA public TO %s", user.Name)
		case postgresv1.PermissionDelete:
			grantQuery = fmt.Sprintf("GRANT DELETE ON ALL TABLES IN SCHEMA public TO %s", user.Name)
		default:
			return fmt.Errorf("unsupported permission: %s", permission)
		}

		if _, err := db.ExecContext(ctx, grantQuery); err != nil {
			return fmt.Errorf("failed to grant %s permission: %w", permission, err)
		}
	}

	return nil
}

func (s *UserService) userExists(ctx context.Context, db *sql.DB, username string) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_user WHERE usename = $1)"
	err := db.QueryRowContext(ctx, query, username).Scan(&exists)
	return exists, err
}

func (s *UserService) generatePassword() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
