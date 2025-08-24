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

// DatabaseSpec defines the desired state of Database
type DatabaseSpec struct {
	// ConnectionRef references a PostGresConnection resource
	// +kubebuilder:validation:Required
	ConnectionRef ConnectionReference `json:"connectionRef"`

	// DatabaseName is the name of the database to create
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=^[a-zA-Z][a-zA-Z0-9_]*$
	DatabaseName string `json:"databaseName"`

	// Users defines the users/roles to create for this database
	// +optional
	Users []DatabaseUser `json:"users,omitempty"`

	// Owner is the owner of the database (defaults to superuser if not specified)
	// +optional
	Owner string `json:"owner,omitempty"`

	// Encoding for the database
	// +kubebuilder:default="UTF8"
	// +optional
	Encoding string `json:"encoding,omitempty"`
}

// ConnectionReference represents a reference to a PostGresConnection
type ConnectionReference struct {
	// Name of the PostGresConnection resource
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the PostGresConnection (defaults to same namespace as Database)
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// DatabaseUser defines a user/role with permissions for the database
type DatabaseUser struct {
	// Name of the user/role to create
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=^[a-zA-Z][a-zA-Z0-9_]*$
	Name string `json:"name"`

	// Permissions for this user on the database
	// +kubebuilder:validation:Required
	Permissions []Permission `json:"permissions"`

	// CreateSecret determines if a secret should be created with user credentials
	// +kubebuilder:default=true
	// +optional
	CreateSecret *bool `json:"createSecret,omitempty"`

	// SecretName is the name of the secret to create (defaults to <database>-<user>)
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// Permission defines database permissions
type Permission string

const (
	// PermissionConnect allows connecting to the database
	PermissionConnect Permission = "CONNECT"
	// PermissionCreate allows creating schemas and tables
	PermissionCreate Permission = "CREATE"
	// PermissionUsage allows using schemas
	PermissionUsage Permission = "USAGE"
	// PermissionSelect allows SELECT operations
	PermissionSelect Permission = "SELECT"
	// PermissionInsert allows INSERT operations
	PermissionInsert Permission = "INSERT"
	// PermissionUpdate allows UPDATE operations
	PermissionUpdate Permission = "UPDATE"
	// PermissionDelete allows DELETE operations
	PermissionDelete Permission = "DELETE"
	// PermissionAll grants all privileges
	PermissionAll Permission = "ALL"
)

// DatabaseStatus defines the observed state of Database.
type DatabaseStatus struct {
	// Ready indicates if the database and users are ready
	// +optional
	Ready bool `json:"ready,omitempty"`

	// DatabaseCreated indicates if the database has been created
	// +optional
	DatabaseCreated bool `json:"databaseCreated,omitempty"`

	// UsersCreated tracks which users have been created
	// +optional
	UsersCreated []string `json:"usersCreated,omitempty"`

	// Message provides human readable status information
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Database is the Schema for the databases API
type Database struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Database
	// +required
	Spec DatabaseSpec `json:"spec"`

	// status defines the observed state of Database
	// +optional
	Status DatabaseStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// DatabaseList contains a list of Database
type DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Database `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Database{}, &DatabaseList{})
}
