package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
	postgresv1 "github.com/silverswarm/pg-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Client struct {
	k8sClient client.Client
}

func NewClient(k8sClient client.Client) *Client {
	return &Client{
		k8sClient: k8sClient,
	}
}

func (c *Client) Connect(ctx context.Context, pgConn *postgresv1.PostGresConnection) (*sql.DB, error) {
	host, port, username, password, err := c.getConnectionDetails(ctx, pgConn)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection details: %w", err)
	}

	log := logf.FromContext(ctx)
	log.Info("Attempting PostgreSQL connection", "host", host, "port", port, "user", username)

	sslMode := pgConn.Spec.SSLMode
	if sslMode == "" {
		sslMode = "require"
	}

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s sslmode=%s",
		host, port, username, password, sslMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Error(err, "Failed to open database connection")
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Error(err, "Failed to ping database")
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Successfully connected to database")
	return db, nil
}

func (c *Client) getConnectionDetails(ctx context.Context, pgConn *postgresv1.PostGresConnection) (string, int32, string, string, error) {
	host := pgConn.Spec.Host
	port := pgConn.Spec.Port
	if port == 0 {
		port = 5432
	}

	if host == "" {
		clusterNamespace := pgConn.Spec.ClusterNamespace
		if clusterNamespace == "" {
			clusterNamespace = pgConn.Namespace
		}

		clusterDomain := os.Getenv("KUBERNETES_CLUSTER_DOMAIN")
		if clusterDomain == "" {
			clusterDomain = "cluster.local"
		}

		host = fmt.Sprintf("%s-rw.%s.svc.%s", pgConn.Spec.ClusterName, clusterNamespace, clusterDomain)
	}

	username, password, err := c.getCredentials(ctx, pgConn)
	if err != nil {
		return "", 0, "", "", err
	}

	return host, port, username, password, nil
}

func (c *Client) getCredentials(ctx context.Context, pgConn *postgresv1.PostGresConnection) (string, string, error) {
	var secretName, secretNamespace string

	if pgConn.Spec.SuperUserSecret != nil {
		secretName = pgConn.Spec.SuperUserSecret.Name
		secretNamespace = pgConn.Spec.SuperUserSecret.Namespace
		if secretNamespace == "" {
			secretNamespace = pgConn.Namespace
		}
	} else {
		secretName = fmt.Sprintf("%s-superuser", pgConn.Spec.ClusterName)
		secretNamespace = pgConn.Spec.ClusterNamespace
		if secretNamespace == "" {
			secretNamespace = pgConn.Namespace
		}
	}

	var secret corev1.Secret
	secretKey := types.NamespacedName{
		Name:      secretName,
		Namespace: secretNamespace,
	}

	if err := c.k8sClient.Get(ctx, secretKey, &secret); err != nil {
		return "", "", fmt.Errorf("failed to get secret %s: %w", secretKey, err)
	}

	username := string(secret.Data["username"])
	password := string(secret.Data["password"])

	if username == "" || password == "" {
		return "", "", fmt.Errorf("secret %s is missing username or password", secretKey)
	}

	return username, password, nil
}
