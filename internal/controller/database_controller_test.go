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
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	postgresv1 "github.com/silverswarm/pg-operator/api/v1"
)

var _ = Describe("Database Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		const connectionName = "test-connection"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		
		connectionNamespacedName := types.NamespacedName{
			Name:      connectionName,
			Namespace: "default",
		}

		BeforeEach(func() {
			// Create a PostGresConnection first
			By("creating the PostGresConnection resource")
			connectionResource := &postgresv1.PostGresConnection{
				ObjectMeta: metav1.ObjectMeta{
					Name:      connectionName,
					Namespace: "default",
				},
				Spec: postgresv1.PostGresConnectionSpec{
					ClusterName: "test-cluster",
				},
			}
			err := k8sClient.Get(ctx, connectionNamespacedName, &postgresv1.PostGresConnection{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, connectionResource)).To(Succeed())
			}

			// Create the Database resource
			By("creating the custom resource for the Kind Database")
			resource := &postgresv1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: postgresv1.DatabaseSpec{
					ConnectionRef: postgresv1.ConnectionReference{
						Name: connectionName,
					},
					DatabaseName: "testdb",
					Users: []postgresv1.DatabaseUser{
						{
							Name: "testuser",
							Permissions: []postgresv1.Permission{
								"CONNECT",
								"CREATE",
							},
							CreateSecret: &[]bool{true}[0],
						},
					},
				},
			}
			err = k8sClient.Get(ctx, typeNamespacedName, &postgresv1.Database{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// Cleanup Database resource
			By("Cleanup the Database resource")
			resource := &postgresv1.Database{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			// Cleanup PostGresConnection resource
			By("Cleanup the PostGresConnection resource")
			connectionResource := &postgresv1.PostGresConnection{}
			err = k8sClient.Get(ctx, connectionNamespacedName, connectionResource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, connectionResource)).To(Succeed())
			}

			// Cleanup any created secrets
			secretList := &corev1.SecretList{}
			err = k8sClient.List(ctx, secretList, client.InNamespace("default"))
			if err == nil {
				for _, secret := range secretList.Items {
					if secret.Name == fmt.Sprintf("%s-%s", resourceName, "testuser") {
						Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())
					}
				}
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &DatabaseReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify the Database resource exists and has proper status
			By("Checking the Database resource status")
			database := &postgresv1.Database{}
			err = k8sClient.Get(ctx, typeNamespacedName, database)
			Expect(err).NotTo(HaveOccurred())
			Expect(database.Spec.DatabaseName).To(Equal("testdb"))
			Expect(database.Spec.Users).To(HaveLen(1))
			Expect(database.Spec.Users[0].Name).To(Equal("testuser"))
		})

		It("should handle missing connection reference gracefully", func() {
			By("Creating a Database with non-existent connection")
			badResourceName := "bad-database"
			badTypeNamespacedName := types.NamespacedName{
				Name:      badResourceName,
				Namespace: "default",
			}

			badResource := &postgresv1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:      badResourceName,
					Namespace: "default",
				},
				Spec: postgresv1.DatabaseSpec{
					ConnectionRef: postgresv1.ConnectionReference{
						Name: "non-existent-connection",
					},
					DatabaseName: "testdb",
				},
			}
			Expect(k8sClient.Create(ctx, badResource)).To(Succeed())

			By("Reconciling the resource with bad connection")
			controllerReconciler := &DatabaseReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: badTypeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred()) // Should handle gracefully

			// Cleanup
			Expect(k8sClient.Delete(ctx, badResource)).To(Succeed())
		})
	})

	Describe("Database Controller Helper Functions", func() {
		var reconciler *DatabaseReconciler
		var database *postgresv1.Database
		var ctx context.Context

		BeforeEach(func() {
			ctx = context.Background()
			reconciler = &DatabaseReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			database = &postgresv1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "helper-test-db",
					Namespace: "default",
				},
				Spec: postgresv1.DatabaseSpec{
					ConnectionRef: postgresv1.ConnectionReference{
						Name: "test-connection",
					},
					DatabaseName: "testdb",
					Users: []postgresv1.DatabaseUser{
						{
							Name:        "testuser",
							Permissions: []postgresv1.Permission{"CONNECT", "CREATE"},
							CreateSecret: &[]bool{true}[0],
						},
					},
				},
			}
		})

		Describe("generatePassword", func() {
			It("should generate a password of correct length", func() {
				password, err := generatePassword()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(password)).To(Equal(24)) // base64 encoding of 16 bytes = 24 chars
			})

			It("should generate different passwords each time", func() {
				password1, err1 := generatePassword()
				password2, err2 := generatePassword()
				
				Expect(err1).NotTo(HaveOccurred())
				Expect(err2).NotTo(HaveOccurred())
				Expect(password1).NotTo(Equal(password2))
			})

			It("should generate passwords with valid characters", func() {
				password, err := generatePassword()
				Expect(err).NotTo(HaveOccurred())
				
				// Should contain URL-safe base64 characters (A-Z, a-z, 0-9, -, _, =)
				matched, _ := regexp.MatchString("^[A-Za-z0-9_-]+=*$", password)
				Expect(matched).To(BeTrue())
			})
		})

		Describe("createUserSecret", func() {
			BeforeEach(func() {
				// Create the database resource in the cluster to get a proper UID
				Expect(k8sClient.Create(ctx, database)).To(Succeed())
			})

			AfterEach(func() {
				// Cleanup database
				Expect(k8sClient.Delete(ctx, database)).To(Succeed())
			})

			It("should create a secret with correct data", func() {
				err := reconciler.createUserSecret(ctx, database, database.Spec.Users[0])
				Expect(err).NotTo(HaveOccurred())

				// Check that the secret was created
				secretName := fmt.Sprintf("%s-%s", database.Spec.DatabaseName, database.Spec.Users[0].Name)
				secret := &corev1.Secret{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      secretName,
					Namespace: database.Namespace,
				}, secret)
				Expect(err).NotTo(HaveOccurred())

				// Verify secret data
				Expect(secret.Data).To(HaveKey("username"))
				Expect(secret.Data).To(HaveKey("password"))
				Expect(string(secret.Data["username"])).To(Equal("testuser"))
				Expect(len(secret.Data["password"])).To(BeNumerically(">", 0))

				// Cleanup
				Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			})

			It("should create a secret with custom name when specified", func() {
				user := database.Spec.Users[0]
				customSecretName := "custom-secret-name"
				user.SecretName = customSecretName

				err := reconciler.createUserSecret(ctx, database, user)
				Expect(err).NotTo(HaveOccurred())

				// Check that the secret was created with custom name
				secret := &corev1.Secret{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      user.SecretName,
					Namespace: database.Namespace,
				}, secret)
				Expect(err).NotTo(HaveOccurred())

				// Cleanup
				Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			})
		})

		Describe("updateStatus", func() {
			BeforeEach(func() {
				// Create the database resource in the cluster
				Expect(k8sClient.Create(ctx, database)).To(Succeed())
			})

			AfterEach(func() {
				// Cleanup
				Expect(k8sClient.Delete(ctx, database)).To(Succeed())
			})

			It("should update database status correctly", func() {
				users := []string{"user1", "user2"}
				_, err := reconciler.updateStatus(ctx, database, true, true, users, "")
				Expect(err).NotTo(HaveOccurred())

				// Fetch updated database
				updatedDB := &postgresv1.Database{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      database.Name,
					Namespace: database.Namespace,
				}, updatedDB)
				Expect(err).NotTo(HaveOccurred())

				// Verify status
				Expect(updatedDB.Status.Ready).To(BeTrue())
				Expect(updatedDB.Status.Message).To(BeEmpty())
				Expect(updatedDB.Status.UsersCreated).To(Equal(users))
			})

			It("should update database status with error message", func() {
				errorMsg := "Failed to connect to database"
				_, err := reconciler.updateStatus(ctx, database, false, false, nil, errorMsg)
				Expect(err).NotTo(HaveOccurred())

				// Fetch updated database
				updatedDB := &postgresv1.Database{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      database.Name,
					Namespace: database.Namespace,
				}, updatedDB)
				Expect(err).NotTo(HaveOccurred())

				// Verify status
				Expect(updatedDB.Status.Ready).To(BeFalse())
				Expect(updatedDB.Status.Message).To(Equal(errorMsg))
			})
		})

		Describe("getPostGresConnection", func() {
			var connection *postgresv1.PostGresConnection

			BeforeEach(func() {
				connection = &postgresv1.PostGresConnection{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-connection",
						Namespace: "default",
					},
					Spec: postgresv1.PostGresConnectionSpec{
						ClusterName: "test-cluster",
					},
				}
				Expect(k8sClient.Create(ctx, connection)).To(Succeed())
			})

			AfterEach(func() {
				Expect(k8sClient.Delete(ctx, connection)).To(Succeed())
			})

			It("should retrieve existing PostGresConnection", func() {
				retrieved, err := reconciler.getPostGresConnection(ctx, database)
				Expect(err).NotTo(HaveOccurred())
				Expect(retrieved).NotTo(BeNil())
				Expect(retrieved.Name).To(Equal("test-connection"))
				Expect(retrieved.Spec.ClusterName).To(Equal("test-cluster"))
			})

			It("should return error for non-existent connection", func() {
				database.Spec.ConnectionRef.Name = "non-existent"
				retrieved, err := reconciler.getPostGresConnection(ctx, database)
				Expect(err).To(HaveOccurred())
				Expect(retrieved).To(BeNil())
			})

			It("should handle cross-namespace connection reference", func() {
				database.Spec.ConnectionRef.Namespace = "other-namespace"
				retrieved, err := reconciler.getPostGresConnection(ctx, database)
				Expect(err).To(HaveOccurred()) // Should fail since connection is in default namespace
				Expect(retrieved).To(BeNil())
			})
		})
	})
})
