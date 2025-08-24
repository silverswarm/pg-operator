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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	_ "github.com/lib/pq"
	corev1 "k8s.io/api/core/v1"

	postgresv1 "github.com/silverswarm/pg-operator/api/v1"
)

// PostGresConnectionReconciler reconciles a PostGresConnection object
type PostGresConnectionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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
		if errors.IsNotFound(err) {
			log.Info("PostGresConnection resource not found")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get PostGresConnection")
		return ctrl.Result{}, err
	}

	if err := r.validateConnection(ctx, &pgConn); err != nil {
		return r.updateStatus(ctx, &pgConn, false, err.Error())
	}

	return r.updateStatus(ctx, &pgConn, true, "Connection validated successfully")
}

func (r *PostGresConnectionReconciler) validateConnection(ctx context.Context, pgConn *postgresv1.PostGresConnection) error {
	host, port, username, password, err := r.getConnectionDetails(ctx, pgConn)
	if err != nil {
		return fmt.Errorf("failed to get connection details: %w", err)
	}

	sslMode := pgConn.Spec.SSLMode
	if sslMode == "" {
		sslMode = "require"
	}

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s sslmode=%s", host, port, username, password, sslMode)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	return nil
}

func (r *PostGresConnectionReconciler) getConnectionDetails(ctx context.Context, pgConn *postgresv1.PostGresConnection) (string, int32, string, string, error) {
	host := pgConn.Spec.Host
	port := pgConn.Spec.Port
	if port == 0 {
		port = 5432
	}

	clusterNamespace := pgConn.Spec.ClusterNamespace
	if clusterNamespace == "" {
		clusterNamespace = pgConn.Namespace
	}

	if host == "" {
		if clusterNamespace == pgConn.Namespace {
			host = fmt.Sprintf("%s-rw", pgConn.Spec.ClusterName)
		} else {
			host = fmt.Sprintf("%s-rw.%s.svc.cluster.local", pgConn.Spec.ClusterName, clusterNamespace)
		}
	}

	var username, password string
	var secret corev1.Secret
	var secretKey types.NamespacedName

	if pgConn.Spec.SuperUserSecret != nil {
		secretNamespace := pgConn.Spec.SuperUserSecret.Namespace
		if secretNamespace == "" {
			secretNamespace = pgConn.Namespace
		}

		secretKey = types.NamespacedName{
			Name:      pgConn.Spec.SuperUserSecret.Name,
			Namespace: secretNamespace,
		}
	} else {
		var secretName string
		if pgConn.Spec.UseAppSecret != nil && *pgConn.Spec.UseAppSecret {
			secretName = fmt.Sprintf("%s-app", pgConn.Spec.ClusterName)
		} else {
			secretName = fmt.Sprintf("%s-superuser", pgConn.Spec.ClusterName)
		}

		secretKey = types.NamespacedName{
			Name:      secretName,
			Namespace: clusterNamespace,
		}
	}

	if err := r.Get(ctx, secretKey, &secret); err != nil {
		return "", 0, "", "", fmt.Errorf("failed to get CNPG secret %s: %w", secretKey, err)
	}

	if uriBytes, ok := secret.Data["uri"]; ok {
		return r.parseURIConnection(string(uriBytes), pgConn)
	}

	usernameBytes, ok := secret.Data["username"]
	if !ok {
		return "", 0, "", "", fmt.Errorf("username not found in CNPG secret %s", secretKey)
	}
	username = string(usernameBytes)

	passwordBytes, ok := secret.Data["password"]
	if !ok {
		return "", 0, "", "", fmt.Errorf("password not found in CNPG secret %s", secretKey)
	}
	password = string(passwordBytes)

	return host, port, username, password, nil
}

func (r *PostGresConnectionReconciler) parseURIConnection(uri string, pgConn *postgresv1.PostGresConnection) (string, int32, string, string, error) {
	return "", 0, "", "", fmt.Errorf("URI parsing not yet implemented, falling back to individual fields")
}

func (r *PostGresConnectionReconciler) updateStatus(ctx context.Context, pgConn *postgresv1.PostGresConnection, ready bool, message string) (ctrl.Result, error) {
	now := metav1.NewTime(time.Now())
	pgConn.Status.Ready = ready
	pgConn.Status.Message = message
	pgConn.Status.LastChecked = &now

	conditionType := "Ready"
	conditionStatus := metav1.ConditionFalse
	if ready {
		conditionStatus = metav1.ConditionTrue
	}

	condition := metav1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		LastTransitionTime: now,
		Reason:             "ConnectionValidated",
		Message:            message,
	}

	pgConn.Status.Conditions = []metav1.Condition{condition}

	if err := r.Status().Update(ctx, pgConn); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
	}

	requeueAfter := time.Minute * 5
	if !ready {
		requeueAfter = time.Minute
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostGresConnectionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&postgresv1.PostGresConnection{}).
		Named("postgresconnection").
		Complete(r)
}
