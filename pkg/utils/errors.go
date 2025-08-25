package utils

import (
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

func HandleReconcileError(err error, msg string, logger logr.Logger) (ctrl.Result, error) {
	if err == nil {
		return ctrl.Result{}, nil
	}

	if errors.IsNotFound(err) {
		logger.Info(fmt.Sprintf("Resource not found: %s", msg))
		return ctrl.Result{}, nil
	}

	if errors.IsConflict(err) {
		logger.Info(fmt.Sprintf("Conflict occurred, requeuing: %s", msg))
		return ctrl.Result{Requeue: true}, nil
	}

	logger.Error(err, msg)
	return ctrl.Result{}, fmt.Errorf("%s: %w", msg, err)
}

func IsRetryableError(err error) bool {
	return errors.IsServiceUnavailable(err) ||
		errors.IsTimeout(err) ||
		errors.IsServerTimeout(err) ||
		errors.IsTooManyRequests(err)
}
