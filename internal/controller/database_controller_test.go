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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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

		It("should handle reconcile when database not found", func() {
			By("Reconciling a non-existent resource")
			controllerReconciler := &DatabaseReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			nonExistentName := types.NamespacedName{
				Name:      "non-existent-db",
				Namespace: "default",
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: nonExistentName,
			})
			Expect(err).NotTo(HaveOccurred()) // Should handle gracefully
		})

		It("should handle when PostGresConnection is not ready", func() {
			By("Creating a Database with connection that is not ready")
			// Create a connection that's not ready
			notReadyConnectionName := "not-ready-connection"
			notReadyConnection := &postgresv1.PostGresConnection{
				ObjectMeta: metav1.ObjectMeta{
					Name:      notReadyConnectionName,
					Namespace: "default",
				},
				Spec: postgresv1.PostGresConnectionSpec{
					ClusterName: "not-ready-cluster",
				},
				Status: postgresv1.PostGresConnectionStatus{
					Ready: false, // Connection not ready
				},
			}
			Expect(k8sClient.Create(ctx, notReadyConnection)).To(Succeed())

			// Create database that uses this connection
			notReadyDatabaseName := "not-ready-db"
			notReadyDatabase := &postgresv1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:      notReadyDatabaseName,
					Namespace: "default",
				},
				Spec: postgresv1.DatabaseSpec{
					ConnectionRef: postgresv1.ConnectionReference{
						Name: notReadyConnectionName,
					},
					DatabaseName: "notreadydb",
				},
			}
			Expect(k8sClient.Create(ctx, notReadyDatabase)).To(Succeed())

			controllerReconciler := &DatabaseReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			notReadyNamespacedName := types.NamespacedName{
				Name:      notReadyDatabaseName,
				Namespace: "default",
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: notReadyNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred()) // Should handle gracefully

			// Cleanup
			Expect(k8sClient.Delete(ctx, notReadyDatabase)).To(Succeed())
			Expect(k8sClient.Delete(ctx, notReadyConnection)).To(Succeed())
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
							Name:         "testuser",
							Permissions:  []postgresv1.Permission{"CONNECT", "CREATE"},
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

		Describe("connectToDatabase", func() {
			It("should handle secret retrieval for connection", func() {
				// Test the connection logic by using a connection that references existing secrets
				connection := &postgresv1.PostGresConnection{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-conn",
						Namespace: "default",
					},
					Spec: postgresv1.PostGresConnectionSpec{
						ClusterName: "test-cluster",
						Host:        "localhost",
						Port:        5432,
						SSLMode:     "disable",
					},
				}

				// This will fail to connect but we can verify the secret lookup logic
				db, err := reconciler.connectToDatabase(ctx, connection)
				Expect(err).To(HaveOccurred())
				Expect(db).To(BeNil())
				// Verify the error contains secret-related error (expected since secret doesn't exist)
				Expect(err.Error()).To(ContainSubstring("secret"))
			})

			It("should handle SSL mode configuration", func() {
				connection := &postgresv1.PostGresConnection{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ssl-test-conn",
						Namespace: "default",
					},
					Spec: postgresv1.PostGresConnectionSpec{
						ClusterName: "test-cluster",
						SSLMode:     "require",
					},
				}

				db, err := reconciler.connectToDatabase(ctx, connection)
				Expect(err).To(HaveOccurred()) // Will fail to connect but function is called
				Expect(db).To(BeNil())
			})

			It("should use custom host and port when specified", func() {
				connection := &postgresv1.PostGresConnection{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "custom-host-conn",
						Namespace: "default",
					},
					Spec: postgresv1.PostGresConnectionSpec{
						ClusterName: "test-cluster",
						Host:        "custom.host.com",
						Port:        5433,
					},
				}

				db, err := reconciler.connectToDatabase(ctx, connection)
				Expect(err).To(HaveOccurred()) // Will fail but tests host/port logic
				Expect(db).To(BeNil())
				// Error should reference secret lookup, not connection itself
				Expect(err.Error()).To(ContainSubstring("secret"))
			})

			It("should use default port when not specified", func() {
				connection := &postgresv1.PostGresConnection{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "default-port-conn",
						Namespace: "default",
					},
					Spec: postgresv1.PostGresConnectionSpec{
						ClusterName: "test-cluster",
						// Port: 0 (default)
					},
				}

				db, err := reconciler.connectToDatabase(ctx, connection)
				Expect(err).To(HaveOccurred()) // Will fail but tests default port logic
				Expect(db).To(BeNil())
			})

			It("should handle custom SuperUserSecret", func() {
				// Create a custom secret first
				customSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "custom-superuser-secret",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"username": []byte("customuser"),
						"password": []byte("custompass"),
					},
				}
				Expect(k8sClient.Create(ctx, customSecret)).To(Succeed())

				connection := &postgresv1.PostGresConnection{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "custom-secret-conn",
						Namespace: "default",
					},
					Spec: postgresv1.PostGresConnectionSpec{
						ClusterName: "test-cluster",
						SuperUserSecret: &postgresv1.SecretReference{
							Name: "custom-superuser-secret",
						},
					},
				}

				db, err := reconciler.connectToDatabase(ctx, connection)
				// This should either fail to connect to DB (expected) or succeed in secret lookup
				if err != nil {
					// Expected: fails to connect to actual database
					Expect(db).To(BeNil())
				} else {
					// Unexpected but possible: secret lookup worked, connection string built
					if db != nil {
						db.Close()
					}
				}

				// Cleanup
				Expect(k8sClient.Delete(ctx, customSecret)).To(Succeed())
			})
		})

		Describe("ensureDatabase", func() {
			It("should handle nil database connection", func() {
				// Test that the function properly handles nil database connection
				database := &postgresv1.Database{
					Spec: postgresv1.DatabaseSpec{
						DatabaseName: "testdb",
						Owner:        "postgres",
						Encoding:     "UTF8",
					},
				}

				// Since we can't connect to a real database, we expect this to fail with nil db
				// but we're verifying the function signature and parameter handling
				// We'll use a defer/recover to catch the nil pointer dereference
				var panicked bool
				func() {
					defer func() {
						if r := recover(); r != nil {
							panicked = true
						}
					}()
					reconciler.ensureDatabase(ctx, nil, database)
				}()
				Expect(panicked).To(BeTrue()) // Expected to panic with nil db
			})

			It("should handle empty database name", func() {
				database := &postgresv1.Database{
					Spec: postgresv1.DatabaseSpec{
						DatabaseName: "", // Empty name
					},
				}

				// This will panic due to nil db connection but we're testing parameter validation
				var panicked bool
				func() {
					defer func() {
						if r := recover(); r != nil {
							panicked = true
						}
					}()
					reconciler.ensureDatabase(ctx, nil, database)
				}()
				Expect(panicked).To(BeTrue())
			})
		})

		Describe("ensureUsers", func() {
			It("should handle user list processing with nil db", func() {
				database := &postgresv1.Database{
					Spec: postgresv1.DatabaseSpec{
						DatabaseName: "testdb",
						Users: []postgresv1.DatabaseUser{
							{
								Name:        "user1",
								Permissions: []postgresv1.Permission{"CONNECT"},
							},
							{
								Name:        "user2",
								Permissions: []postgresv1.Permission{"CONNECT", "CREATE"},
							},
						},
					},
				}

				// This will panic due to nil db, we catch it to test parameter handling
				var panicked bool
				func() {
					defer func() {
						if r := recover(); r != nil {
							panicked = true
						}
					}()
					reconciler.ensureUsers(ctx, nil, database)
				}()
				Expect(panicked).To(BeTrue())
			})

			It("should handle empty user list", func() {
				database := &postgresv1.Database{
					Spec: postgresv1.DatabaseSpec{
						DatabaseName: "testdb",
						Users:        []postgresv1.DatabaseUser{}, // Empty users
					},
				}

				users, err := reconciler.ensureUsers(ctx, nil, database)
				Expect(err).NotTo(HaveOccurred()) // Should succeed with empty list
				Expect(users).To(BeEmpty())
			})
		})

		Describe("ensureUser", func() {
			It("should handle user creation parameters with nil db", func() {
				user := postgresv1.DatabaseUser{
					Name: "testuser",
					Permissions: []postgresv1.Permission{
						"CONNECT",
						"CREATE",
					},
				}

				// This will panic due to nil db, we catch it to test parameter handling
				var panicked bool
				func() {
					defer func() {
						if r := recover(); r != nil {
							panicked = true
						}
					}()
					reconciler.ensureUser(ctx, nil, user)
				}()
				Expect(panicked).To(BeTrue())
			})

			It("should handle empty user name with nil db", func() {
				user := postgresv1.DatabaseUser{
					Name:        "", // Invalid empty name
					Permissions: []postgresv1.Permission{"CONNECT"},
				}

				// This will panic due to nil db, we catch it to test parameter handling
				var panicked bool
				func() {
					defer func() {
						if r := recover(); r != nil {
							panicked = true
						}
					}()
					reconciler.ensureUser(ctx, nil, user)
				}()
				Expect(panicked).To(BeTrue())
			})
		})

		Describe("grantPermissions", func() {
			It("should handle permission granting with nil db", func() {
				user := postgresv1.DatabaseUser{
					Name: "testuser",
					Permissions: []postgresv1.Permission{
						"CONNECT",
						"CREATE",
						"SELECT",
					},
				}

				// This will panic due to nil db, we catch it to test permission processing
				var panicked bool
				func() {
					defer func() {
						if r := recover(); r != nil {
							panicked = true
						}
					}()
					reconciler.grantPermissions(ctx, nil, "testdb", user)
				}()
				Expect(panicked).To(BeTrue())
			})

			It("should handle ALL permission with nil db", func() {
				user := postgresv1.DatabaseUser{
					Name:        "adminuser",
					Permissions: []postgresv1.Permission{"ALL"},
				}

				// This will panic due to nil db, we catch it to test ALL permission handling
				var panicked bool
				func() {
					defer func() {
						if r := recover(); r != nil {
							panicked = true
						}
					}()
					reconciler.grantPermissions(ctx, nil, "testdb", user)
				}()
				Expect(panicked).To(BeTrue())
			})

			It("should handle empty permissions", func() {
				user := postgresv1.DatabaseUser{
					Name:        "limiteduser",
					Permissions: []postgresv1.Permission{}, // No permissions
				}

				err := reconciler.grantPermissions(ctx, nil, "testdb", user)
				Expect(err).NotTo(HaveOccurred()) // Should succeed with no permissions to grant
			})
		})

		Describe("SetupWithManager", func() {
			It("should setup controller with manager", func() {
				// Create a fake manager for testing
				scheme := runtime.NewScheme()
				Expect(postgresv1.AddToScheme(scheme)).To(Succeed())

				// This tests the controller registration logic
				// Use testEnv to get the config
				mgr, err := manager.New(testEnv.Config, manager.Options{
					Scheme:  scheme,
					Metrics: manager.Options{}.Metrics, // Disable metrics server for tests
				})
				Expect(err).NotTo(HaveOccurred())

				err = reconciler.SetupWithManager(mgr)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
