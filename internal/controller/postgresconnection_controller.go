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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	postgresv1 "github.com/silverswarm/pg-operator/api/v1"
	"github.com/silverswarm/pg-operator/pkg/k8s"
	"github.com/silverswarm/pg-operator/pkg/postgres"
	"github.com/silverswarm/pg-operator/pkg/utils"
)

// PostGresConnectionReconciler reconciles a PostGresConnection object
type PostGresConnectionReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	pgClient      *postgres.Client
	statusService *k8s.StatusService
}

// +kubebuilder:rbac:groups=postgres.silverswarm.io,resources=postgresconnections,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=postgres.silverswarm.io,resources=postgresconnections/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=postgres.silverswarm.io,resources=postgresconnections/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch

func (r *PostGresConnectionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var pgConn postgresv1.PostGresConnection
	if err := r.Get(ctx, req.NamespacedName, &pgConn); err != nil {
		return utils.HandleReconcileError(err, "Failed to get PostGresConnection", log)
	}

	if err := r.validateConnection(ctx, &pgConn); err != nil {
		return r.statusService.UpdatePostGresConnectionStatus(ctx, &pgConn, false, err.Error())
	}

	return r.statusService.UpdatePostGresConnectionStatus(ctx, &pgConn, true, "Connection validated successfully")
}

func (r *PostGresConnectionReconciler) validateConnection(ctx context.Context, pgConn *postgresv1.PostGresConnection) error {
	db, err := r.pgClient.Connect(ctx, pgConn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	return nil
}

// NewPostGresConnectionReconciler creates a new PostGresConnectionReconciler with all required services
func NewPostGresConnectionReconciler(client client.Client, scheme *runtime.Scheme) *PostGresConnectionReconciler {
	return &PostGresConnectionReconciler{
		Client:        client,
		Scheme:        scheme,
		pgClient:      postgres.NewClient(client),
		statusService: k8s.NewStatusService(client),
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostGresConnectionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&postgresv1.PostGresConnection{}).
		Named("postgresconnection").
		Complete(r)
}
