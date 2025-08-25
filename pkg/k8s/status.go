package k8s

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	postgresv1 "github.com/silverswarm/pg-operator/api/v1"
)

type StatusService struct {
	client client.Client
}

func NewStatusService(client client.Client) *StatusService {
	return &StatusService{
		client: client,
	}
}

func (s *StatusService) UpdateDatabaseStatus(ctx context.Context, database *postgresv1.Database, ready, databaseCreated bool, usersCreated []string, message string) (ctrl.Result, error) {
	database.Status.Ready = ready
	database.Status.DatabaseCreated = databaseCreated
	database.Status.UsersCreated = usersCreated
	database.Status.Message = message

	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "Reconciling",
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	if ready {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "Ready"
		condition.Message = "Database and users are ready"
	}

	meta.SetStatusCondition(&database.Status.Conditions, condition)

	if err := s.client.Status().Update(ctx, database); err != nil {
		return ctrl.Result{}, err
	}

	if !ready {
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	return ctrl.Result{}, nil
}

func (s *StatusService) UpdatePostGresConnectionStatus(ctx context.Context, pgConn *postgresv1.PostGresConnection, ready bool, message string) (ctrl.Result, error) {
	pgConn.Status.Ready = ready
	pgConn.Status.Message = message

	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "Reconciling",
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	if ready {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "Ready"
		condition.Message = "Connection is ready"
	}

	meta.SetStatusCondition(&pgConn.Status.Conditions, condition)

	if err := s.client.Status().Update(ctx, pgConn); err != nil {
		return ctrl.Result{}, err
	}

	if !ready {
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	return ctrl.Result{}, nil
}
