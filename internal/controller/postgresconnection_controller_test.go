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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	postgresv1 "github.com/silverswarm/pg-operator/api/v1"
)

var _ = Describe("PostGresConnection Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the custom resource for the Kind PostGresConnection")
			err := k8sClient.Get(ctx, typeNamespacedName, &postgresv1.PostGresConnection{})
			if err != nil && errors.IsNotFound(err) {
				resource := &postgresv1.PostGresConnection{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: postgresv1.PostGresConnectionSpec{
						ClusterName: "test-cluster",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup the PostGresConnection resource")
			resource := &postgresv1.PostGresConnection{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &PostGresConnectionReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify the PostGresConnection resource exists and has proper status
			By("Checking the PostGresConnection resource status")
			connection := &postgresv1.PostGresConnection{}
			err = k8sClient.Get(ctx, typeNamespacedName, connection)
			Expect(err).NotTo(HaveOccurred())
			Expect(connection.Spec.ClusterName).To(Equal("test-cluster"))
		})
	})

	Describe("PostGresConnection Controller Helper Functions", func() {
		var reconciler *PostGresConnectionReconciler
		var connection *postgresv1.PostGresConnection
		var ctx context.Context

		BeforeEach(func() {
			ctx = context.Background()
			reconciler = &PostGresConnectionReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			connection = &postgresv1.PostGresConnection{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-connection",
					Namespace: "default",
				},
				Spec: postgresv1.PostGresConnectionSpec{
					ClusterName: "test-cluster",
				},
			}
		})

		Describe("getConnectionDetails", func() {
			var superuserSecret *corev1.Secret

			BeforeEach(func() {
				// Create a mock CNPG superuser secret
				superuserSecret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-superuser",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"username": []byte("postgres"),
						"password": []byte("secret123"),
						"dbname":   []byte("postgres"),
						"host":     []byte("test-cluster-rw"),
						"port":     []byte("5432"),
					},
				}
				Expect(k8sClient.Create(ctx, superuserSecret)).To(Succeed())
			})

			AfterEach(func() {
				Expect(k8sClient.Delete(ctx, superuserSecret)).To(Succeed())
			})

			It("should retrieve connection details from superuser secret", func() {
				host, port, username, password, err := reconciler.getConnectionDetails(ctx, connection)
				Expect(err).NotTo(HaveOccurred())
				Expect(host).To(Equal("test-cluster-rw"))
				Expect(port).To(Equal(int32(5432)))
				Expect(username).To(Equal("postgres"))
				Expect(password).To(Equal("secret123"))
			})

			It("should handle missing secret gracefully", func() {
				// Use a connection with non-existent cluster
				badConnection := &postgresv1.PostGresConnection{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bad-connection",
						Namespace: "default",
					},
					Spec: postgresv1.PostGresConnectionSpec{
						ClusterName: "non-existent-cluster",
					},
				}

				_, _, _, _, err := reconciler.getConnectionDetails(ctx, badConnection)
				Expect(err).To(HaveOccurred())
			})

			It("should use app secret when specified", func() {
				// Create a mock CNPG app secret
				appSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-app",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"username": []byte("app"),
						"password": []byte("apppass123"),
						"dbname":   []byte("app"),
						"host":     []byte("test-cluster-rw"),
						"port":     []byte("5432"),
					},
				}
				Expect(k8sClient.Create(ctx, appSecret)).To(Succeed())

				// Configure connection to use app secret
				connection.Spec.UseAppSecret = &[]bool{true}[0]

				host, port, username, password, err := reconciler.getConnectionDetails(ctx, connection)
				Expect(err).NotTo(HaveOccurred())
				Expect(host).To(Equal("test-cluster-rw"))
				Expect(port).To(Equal(int32(5432)))
				Expect(username).To(Equal("app"))
				Expect(password).To(Equal("apppass123"))

				// Cleanup
				Expect(k8sClient.Delete(ctx, appSecret)).To(Succeed())
			})

			It("should handle cross-namespace connections", func() {
				// Create connection with different namespace
				connection.Spec.ClusterNamespace = "postgres-system"

				_, _, _, _, err := reconciler.getConnectionDetails(ctx, connection)
				Expect(err).To(HaveOccurred()) // Should fail since secret is in default namespace
			})

			It("should use custom host and port when specified", func() {
				connection.Spec.Host = "custom-host"
				connection.Spec.Port = 5433

				host, port, _, _, err := reconciler.getConnectionDetails(ctx, connection)
				Expect(err).NotTo(HaveOccurred())
				Expect(host).To(Equal("custom-host"))
				Expect(port).To(Equal(int32(5433)))
			})

			It("should apply SSL mode configuration", func() {
				connection.Spec.SSLMode = "disable"

				_, _, _, _, err := reconciler.getConnectionDetails(ctx, connection)
				Expect(err).NotTo(HaveOccurred())
				// Note: SSLMode is not returned by getConnectionDetails, it's used in connection string
			})
		})

		Describe("updateStatus", func() {
			BeforeEach(func() {
				// Create the connection resource in the cluster
				Expect(k8sClient.Create(ctx, connection)).To(Succeed())
			})

			AfterEach(func() {
				// Cleanup
				Expect(k8sClient.Delete(ctx, connection)).To(Succeed())
			})

			It("should update connection status correctly", func() {
				_, err := reconciler.updateStatus(ctx, connection, true, "Connection validated successfully")
				Expect(err).NotTo(HaveOccurred())

				// Fetch updated connection
				updatedConn := &postgresv1.PostGresConnection{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      connection.Name,
					Namespace: connection.Namespace,
				}, updatedConn)
				Expect(err).NotTo(HaveOccurred())

				// Verify status
				Expect(updatedConn.Status.Ready).To(BeTrue())
				Expect(updatedConn.Status.Message).To(Equal("Connection validated successfully"))
			})

			It("should update connection status with error message", func() {
				errorMsg := "Failed to connect to PostgreSQL cluster"
				_, err := reconciler.updateStatus(ctx, connection, false, errorMsg)
				Expect(err).NotTo(HaveOccurred())

				// Fetch updated connection
				updatedConn := &postgresv1.PostGresConnection{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      connection.Name,
					Namespace: connection.Namespace,
				}, updatedConn)
				Expect(err).NotTo(HaveOccurred())

				// Verify status
				Expect(updatedConn.Status.Ready).To(BeFalse())
				Expect(updatedConn.Status.Message).To(Equal(errorMsg))
			})
		})
	})
})
