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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PostGresConnectionSpec defines the desired state of PostGresConnection
type PostGresConnectionSpec struct {
	// ClusterName references the CNPG cluster name
	// +kubebuilder:validation:Required
	ClusterName string `json:"clusterName"`

	// ClusterNamespace is the namespace where the CNPG cluster is located
	// Defaults to the same namespace as the PostGresConnection if not specified
	// +optional
	ClusterNamespace string `json:"clusterNamespace,omitempty"`

	// SuperUserSecret references the secret containing superuser credentials
	// Defaults to {clusterName}-superuser if not specified
	// +optional
	SuperUserSecret *SecretReference `json:"superUserSecret,omitempty"`

	// UseAppSecret determines whether to use the app user secret instead of superuser
	// When true, uses {clusterName}-app secret for connection
	// +kubebuilder:default=false
	// +optional
	UseAppSecret *bool `json:"useAppSecret,omitempty"`

	// Host is the PostgreSQL host (if not using CNPG service discovery)
	// Defaults to {clusterName}-rw service if not specified
	// +optional
	Host string `json:"host,omitempty"`

	// Port is the PostgreSQL port
	// +kubebuilder:default=5432
	// +optional
	Port int32 `json:"port,omitempty"`

	// SSLMode specifies the SSL mode for the connection
	// +kubebuilder:default="require"
	// +kubebuilder:validation:Enum=disable;allow;prefer;require;verify-ca;verify-full
	// +optional
	SSLMode string `json:"sslMode,omitempty"`
}

// SecretReference represents a reference to a secret
type SecretReference struct {
	// Name of the secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the secret (defaults to same namespace as PostGresConnection)
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// PostGresConnectionStatus defines the observed state of PostGresConnection.
type PostGresConnectionStatus struct {
	// Ready indicates if the connection is ready to be used
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Message provides human readable status information
	// +optional
	Message string `json:"message,omitempty"`

	// LastChecked is the last time the connection was verified
	// +optional
	LastChecked *metav1.Time `json:"lastChecked,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PostGresConnection is the Schema for the postgresconnections API
type PostGresConnection struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of PostGresConnection
	// +required
	Spec PostGresConnectionSpec `json:"spec"`

	// status defines the observed state of PostGresConnection
	// +optional
	Status PostGresConnectionStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// PostGresConnectionList contains a list of PostGresConnection
type PostGresConnectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostGresConnection `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostGresConnection{}, &PostGresConnectionList{})
}
