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
	"database/sql"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"

	postgresv1 "github.com/silverswarm/pg-operator/api/v1"
	"github.com/silverswarm/pg-operator/pkg/k8s"
	"github.com/silverswarm/pg-operator/pkg/postgres"
	"github.com/silverswarm/pg-operator/pkg/utils"
)

// DatabaseReconciler reconciles a Database object
type DatabaseReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	pgClient      *postgres.Client
	dbService     *postgres.DatabaseService
	userService   *postgres.UserService
	secretService *k8s.SecretService
	statusService *k8s.StatusService
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
		return utils.HandleReconcileError(err, "Failed to get Database", log)
	}

	pgConn, err := r.getPostGresConnection(ctx, &database)
	if err != nil {
		return r.statusService.UpdateDatabaseStatus(ctx, &database, false, false, nil, err.Error())
	}

	if !pgConn.Status.Ready {
		return r.statusService.UpdateDatabaseStatus(ctx, &database, false, false, nil, "PostgreSQL connection is not ready")
	}

	db, err := r.pgClient.Connect(ctx, pgConn)
	if err != nil {
		return r.statusService.UpdateDatabaseStatus(ctx, &database, false, false, nil, fmt.Sprintf("Failed to connect to database: %v", err))
	}
	defer db.Close()

	databaseCreated, err := r.dbService.EnsureDatabase(ctx, db, &database)
	if err != nil {
		return r.statusService.UpdateDatabaseStatus(ctx, &database, false, false, nil, fmt.Sprintf("Failed to ensure database: %v", err))
	}

	usersCreated, err := r.ensureUsers(ctx, db, &database)
	if err != nil {
		return r.statusService.UpdateDatabaseStatus(ctx, &database, false, databaseCreated, usersCreated, fmt.Sprintf("Failed to ensure users: %v", err))
	}

	return r.statusService.UpdateDatabaseStatus(ctx, &database, true, databaseCreated, usersCreated, "Database and users ready")
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

func (r *DatabaseReconciler) ensureUsers(ctx context.Context, db *sql.DB, database *postgresv1.Database) ([]string, error) {
	usersCreated, err := r.userService.EnsureUsers(ctx, db, database)
	if err != nil {
		return usersCreated, err
	}

	for _, user := range database.Spec.Users {
		if user.CreateSecret == nil || *user.CreateSecret {
			password, err := utils.GenerateSecurePassword()
			if err != nil {
				return usersCreated, fmt.Errorf("failed to generate password for user %s: %w", user.Name, err)
			}

			if err := r.secretService.CreateUserSecret(ctx, database, user, password); err != nil {
				return usersCreated, fmt.Errorf("failed to create secret for user %s: %w", user.Name, err)
			}
		}
	}

	return usersCreated, nil
}

// NewDatabaseReconciler creates a new DatabaseReconciler with all required services
func NewDatabaseReconciler(client client.Client, scheme *runtime.Scheme) *DatabaseReconciler {
	pgClient := postgres.NewClient(client)
	return &DatabaseReconciler{
		Client:        client,
		Scheme:        scheme,
		pgClient:      pgClient,
		dbService:     postgres.NewDatabaseService(pgClient),
		userService:   postgres.NewUserService(pgClient),
		secretService: k8s.NewSecretService(client, scheme),
		statusService: k8s.NewStatusService(client),
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&postgresv1.Database{}).
		Owns(&corev1.Secret{}).
		Named("database").
		Complete(r)
}
