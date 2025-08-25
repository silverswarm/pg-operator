package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	postgresv1 "github.com/silverswarm/pg-operator/api/v1"
)

type SecretService struct {
	client client.Client
	scheme *runtime.Scheme
}

func NewSecretService(client client.Client, scheme *runtime.Scheme) *SecretService {
	return &SecretService{
		client: client,
		scheme: scheme,
	}
}

func (s *SecretService) CreateUserSecret(ctx context.Context, database *postgresv1.Database, user postgresv1.DatabaseUser, password string) error {
	secretName := user.SecretName
	if secretName == "" {
		secretName = fmt.Sprintf("%s-%s", database.Name, user.Name)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: database.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"username": []byte(user.Name),
			"password": []byte(password),
		},
	}

	if err := controllerutil.SetControllerReference(database, secret, s.scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	if err := s.client.Create(ctx, secret); err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("failed to create secret: %w", err)
	}

	return nil
}

func (s *SecretService) GetSecret(ctx context.Context, name, namespace string) (*corev1.Secret, error) {
	var secret corev1.Secret
	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	if err := s.client.Get(ctx, key, &secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", key, err)
	}

	return &secret, nil
}
