/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	_ "github.com/lib/pq"
	corev1 "k8s.io/api/core/v1"

	postgresv1 "github.com/silverswarm/pg-operator/api/v1"
)

// DatabaseReconciler reconciles a Database object
type DatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=postgres.silverswarm.io,resources=databases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=postgres.silverswarm.io,resources=databases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=postgres.silverswarm.io,resources=databases/finalizers,verbs=update
// +kubebuilder:rbac:groups=postgres.silverswarm.io,resources=postgresconnections,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var database postgresv1.Database
	if err := r.Get(ctx, req.NamespacedName, &database); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Database resource not found")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Database")
		return ctrl.Result{}, err
	}

	pgConn, err := r.getPostGresConnection(ctx, &database)
	if err != nil {
		return r.updateStatus(ctx, &database, false, false, nil, err.Error())
	}

	if !pgConn.Status.Ready {
		return r.updateStatus(ctx, &database, false, false, nil, "PostgreSQL connection is not ready")
	}

	db, err := r.connectToDatabase(ctx, pgConn)
	if err != nil {
		return r.updateStatus(ctx, &database, false, false, nil, fmt.Sprintf("Failed to connect to database: %v", err))
	}
	defer db.Close()

	databaseCreated, err := r.ensureDatabase(ctx, db, &database)
	if err != nil {
		return r.updateStatus(ctx, &database, false, false, nil, fmt.Sprintf("Failed to ensure database: %v", err))
	}

	usersCreated, err := r.ensureUsers(ctx, db, &database)
	if err != nil {
		return r.updateStatus(ctx, &database, false, databaseCreated, usersCreated, fmt.Sprintf("Failed to ensure users: %v", err))
	}

	return r.updateStatus(ctx, &database, true, databaseCreated, usersCreated, "Database and users ready")
}

func (r *DatabaseReconciler) getPostGresConnection(ctx context.Context, database *postgresv1.Database) (*postgresv1.PostGresConnection, error) {
	connNamespace := database.Spec.ConnectionRef.Namespace
	if connNamespace == "" {
		connNamespace = database.Namespace
	}

	var pgConn postgresv1.PostGresConnection
	connKey := types.NamespacedName{
		Name:      database.Spec.ConnectionRef.Name,
		Namespace: connNamespace,
	}

	if err := r.Get(ctx, connKey, &pgConn); err != nil {
		return nil, fmt.Errorf("failed to get PostGresConnection %s: %w", connKey, err)
	}

	return &pgConn, nil
}

func (r *DatabaseReconciler) connectToDatabase(ctx context.Context, pgConn *postgresv1.PostGresConnection) (*sql.DB, error) {
	host := pgConn.Spec.Host
	port := pgConn.Spec.Port
	if port == 0 {
		port = 5432
	}
	if host == "" {
		host = fmt.Sprintf("%s-rw", pgConn.Spec.ClusterName)
	}

	var username, password string
	if pgConn.Spec.SuperUserSecret != nil {
		secretNamespace := pgConn.Spec.SuperUserSecret.Namespace
		if secretNamespace == "" {
			secretNamespace = pgConn.Namespace
		}

		var secret corev1.Secret
		secretKey := types.NamespacedName{
			Name:      pgConn.Spec.SuperUserSecret.Name,
			Namespace: secretNamespace,
		}

		if err := r.Get(ctx, secretKey, &secret); err != nil {
			return nil, fmt.Errorf("failed to get secret %s: %w", secretKey, err)
		}

		username = string(secret.Data["username"])
		password = string(secret.Data["password"])
	} else {
		secretName := fmt.Sprintf("%s-superuser", pgConn.Spec.ClusterName)
		var secret corev1.Secret
		secretKey := types.NamespacedName{
			Name:      secretName,
			Namespace: pgConn.Namespace,
		}

		if err := r.Get(ctx, secretKey, &secret); err != nil {
			return nil, fmt.Errorf("failed to get CNPG superuser secret %s: %w", secretKey, err)
		}

		username = string(secret.Data["username"])
		password = string(secret.Data["password"])
	}

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s sslmode=require", host, port, username, password)
	return sql.Open("postgres", connStr)
}

func (r *DatabaseReconciler) ensureDatabase(ctx context.Context, db *sql.DB, database *postgresv1.Database) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
	if err := db.QueryRowContext(ctx, query, database.Spec.DatabaseName).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check if database exists: %w", err)
	}

	if exists {
		return true, nil
	}

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

	if _, err := db.ExecContext(ctx, createQuery); err != nil {
		return false, fmt.Errorf("failed to create database: %w", err)
	}

	return true, nil
}

func (r *DatabaseReconciler) ensureUsers(ctx context.Context, db *sql.DB, database *postgresv1.Database) ([]string, error) {
	usersCreated := make([]string, 0, len(database.Spec.Users))

	for _, user := range database.Spec.Users {
		if err := r.ensureUser(ctx, db, user); err != nil {
			return usersCreated, fmt.Errorf("failed to ensure user %s: %w", user.Name, err)
		}

		if err := r.grantPermissions(ctx, db, database.Spec.DatabaseName, user); err != nil {
			return usersCreated, fmt.Errorf("failed to grant permissions to user %s: %w", user.Name, err)
		}

		if user.CreateSecret == nil || *user.CreateSecret {
			if err := r.createUserSecret(ctx, database, user); err != nil {
				return usersCreated, fmt.Errorf("failed to create secret for user %s: %w", user.Name, err)
			}
		}

		usersCreated = append(usersCreated, user.Name)
	}

	return usersCreated, nil
}

func (r *DatabaseReconciler) ensureUser(ctx context.Context, db *sql.DB, user postgresv1.DatabaseUser) error {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)"
	if err := db.QueryRowContext(ctx, query, user.Name).Scan(&exists); err != nil {
		return fmt.Errorf("failed to check if user exists: %w", err)
	}

	if exists {
		return nil
	}

	password, err := generatePassword()
	if err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}

	createQuery := fmt.Sprintf("CREATE ROLE %s WITH LOGIN PASSWORD '%s'", user.Name, password)
	if _, err := db.ExecContext(ctx, createQuery); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (r *DatabaseReconciler) grantPermissions(ctx context.Context, db *sql.DB, databaseName string, user postgresv1.DatabaseUser) error {
	for _, permission := range user.Permissions {
		var query string
		switch permission {
		case postgresv1.PermissionConnect:
			query = fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s", databaseName, user.Name)
		case postgresv1.PermissionCreate:
			query = fmt.Sprintf("GRANT CREATE ON DATABASE %s TO %s", databaseName, user.Name)
		case postgresv1.PermissionAll:
			query = fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", databaseName, user.Name)
		default:
			continue
		}

		if _, err := db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to grant permission %s: %w", permission, err)
		}
	}

	return nil
}

func (r *DatabaseReconciler) createUserSecret(ctx context.Context, database *postgresv1.Database, user postgresv1.DatabaseUser) error {
	secretName := user.SecretName
	if secretName == "" {
		secretName = fmt.Sprintf("%s-%s", database.Spec.DatabaseName, user.Name)
	}

	var existingSecret corev1.Secret
	secretKey := types.NamespacedName{
		Name:      secretName,
		Namespace: database.Namespace,
	}

	err := r.Get(ctx, secretKey, &existingSecret)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get existing secret: %w", err)
	}

	if err == nil {
		return nil
	}

	password, err := generatePassword()
	if err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: database.Namespace,
		},
		Data: map[string][]byte{
			"username": []byte(user.Name),
			"password": []byte(password),
		},
	}

	if err := controllerutil.SetControllerReference(database, secret, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	if err := r.Create(ctx, secret); err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	return nil
}

func (r *DatabaseReconciler) updateStatus(ctx context.Context, database *postgresv1.Database, ready, databaseCreated bool, usersCreated []string, message string) (ctrl.Result, error) {
	database.Status.Ready = ready
	database.Status.DatabaseCreated = databaseCreated
	database.Status.UsersCreated = usersCreated
	database.Status.Message = message

	conditionType := "Ready"
	conditionStatus := metav1.ConditionFalse
	if ready {
		conditionStatus = metav1.ConditionTrue
	}

	condition := metav1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		LastTransitionTime: metav1.NewTime(time.Now()),
		Reason:             "DatabaseReconciled",
		Message:            message,
	}

	database.Status.Conditions = []metav1.Condition{condition}

	if err := r.Status().Update(ctx, database); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
	}

	requeueAfter := time.Minute * 5
	if !ready {
		requeueAfter = time.Minute
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func generatePassword() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&postgresv1.Database{}).
		Owns(&corev1.Secret{}).
		Named("database").
		Complete(r)
}
