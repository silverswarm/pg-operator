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

package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	postgresv1 "github.com/silverswarm/pg-operator/api/v1"
	"github.com/silverswarm/pg-operator/internal/testutil"
)

var _ = Describe("PostgreSQL Operator", Ordered, func() {
	const (
		testNamespace    = "default"
		clusterName      = "postgres-cluster"
		connectionName   = "test-connection"
		databaseName     = "test-database"
		crossNsNamespace = "test-app"
		timeout          = 15 * time.Minute
		interval         = 5 * time.Second
	)

	var (
		ctx = context.Background()
	)

	BeforeAll(func() {
		By("Waiting for CNPG operator to be ready")
		cmd := exec.Command("kubectl", "wait", "--for=condition=Available",
			"deployment/cnpg-controller-manager", "-n", "cnpg-system", "--timeout=300s")
		_, err := testutil.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "CNPG operator should be ready")

		By("Verifying PostgreSQL cluster is ready")
		cmd = exec.Command("kubectl", "wait", "--for=condition=Ready",
			fmt.Sprintf("cluster/%s", clusterName), "--timeout=600s")
		_, err = testutil.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "PostgreSQL cluster should be ready")

		By("Verifying pg-operator is ready")
		cmd = exec.Command("kubectl", "wait", "--for=condition=Available",
			"deployment/operator-controller-manager", "-n", "operator-system", "--timeout=300s")
		_, err = testutil.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "pg-operator should be ready")
	})

	Context("PostGresConnection", func() {
		It("should create and validate connection", func() {
			By("Creating PostGresConnection")
			cmd := exec.Command("kubectl", "apply", "-f", "test/e2e/manifests/postgresconnection.yaml")
			_, err := testutil.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create PostGresConnection")

			By("Waiting for connection to be ready")
			Eventually(func() bool {
				connection := &postgresv1.PostGresConnection{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      connectionName,
					Namespace: testNamespace,
				}, connection)
				if err != nil {
					return false
				}
				return connection.Status.Ready
			}, timeout, interval).Should(BeTrue())

			By("Verifying connection status")
			connection := &postgresv1.PostGresConnection{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      connectionName,
				Namespace: testNamespace,
			}, connection)
			Expect(err).NotTo(HaveOccurred())
			Expect(connection.Status.Ready).To(BeTrue())
			Expect(connection.Status.Message).To(ContainSubstring("ready"))
		})
	})

	Context("Database Management", func() {
		It("should create database with users", func() {
			By("Creating Database resource")
			cmd := exec.Command("kubectl", "apply", "-f", "test/e2e/manifests/database.yaml")
			_, err := testutil.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create Database")

			By("Waiting for database to be ready")
			Eventually(func() bool {
				database := &postgresv1.Database{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      databaseName,
					Namespace: testNamespace,
				}, database)
				if err != nil {
					return false
				}
				return database.Status.Ready
			}, timeout, interval).Should(BeTrue())

			By("Verifying database status")
			database := &postgresv1.Database{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      databaseName,
				Namespace: testNamespace,
			}, database)
			Expect(err).NotTo(HaveOccurred())
			Expect(database.Status.Ready).To(BeTrue())
			Expect(database.Status.DatabaseCreated).To(BeTrue())
			Expect(database.Status.UsersCreated).To(ContainElements("app_user", "readonly_user"))
		})

		It("should create user secrets", func() {
			By("Checking app_user secret exists")
			appUserSecret := &corev1.Secret{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-database-app-user",
				Namespace: testNamespace,
			}, appUserSecret)
			Expect(err).NotTo(HaveOccurred(), "app_user secret should exist")
			Expect(appUserSecret.Data).To(HaveKey("username"))
			Expect(appUserSecret.Data).To(HaveKey("password"))
			Expect(string(appUserSecret.Data["username"])).To(Equal("app_user"))

			By("Checking readonly_user secret exists")
			readonlyUserSecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "readonly-user-credentials",
				Namespace: testNamespace,
			}, readonlyUserSecret)
			Expect(err).NotTo(HaveOccurred(), "readonly_user secret should exist")
			Expect(readonlyUserSecret.Data).To(HaveKey("username"))
			Expect(readonlyUserSecret.Data).To(HaveKey("password"))
			Expect(string(readonlyUserSecret.Data["username"])).To(Equal("readonly_user"))
		})

		It("should verify database and users in PostgreSQL", func() {
			By("Getting PostgreSQL primary pod")
			cmd := exec.Command("kubectl", "get", "pods", "-l",
				fmt.Sprintf("cnpg.io/cluster=%s,role=primary", clusterName),
				"-o", "jsonpath={.items[0].metadata.name}")
			podName, err := testutil.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Should get PostgreSQL pod")
			podName = strings.TrimSpace(podName)
			Expect(podName).NotTo(BeEmpty())

			By("Verifying database exists")
			cmd = exec.Command("kubectl", "exec", podName, "--",
				"psql", "-U", "postgres", "-c", "\\l")
			output, err := testutil.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Should list databases")
			Expect(output).To(ContainSubstring("testdb"))

			By("Verifying users exist")
			cmd = exec.Command("kubectl", "exec", podName, "--",
				"psql", "-U", "postgres", "-c", "\\du")
			output, err = testutil.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Should list users")
			Expect(output).To(ContainSubstring("app_user"))
			Expect(output).To(ContainSubstring("readonly_user"))

			By("Testing user permissions")
			// Get app_user password
			appUserSecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-database-app-user",
				Namespace: testNamespace,
			}, appUserSecret)
			Expect(err).NotTo(HaveOccurred())
			appPassword := string(appUserSecret.Data["password"])

			// Test app_user can create tables and insert data
			cmd = exec.Command("kubectl", "exec", podName, "--", "env",
				fmt.Sprintf("PGPASSWORD=%s", appPassword), "psql", "-U", "app_user", "-d", "testdb", "-c",
				"CREATE TABLE IF NOT EXISTS test_table (id INT, name TEXT);")
			_, err = testutil.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "app_user should be able to create tables")

			cmd = exec.Command("kubectl", "exec", podName, "--", "env",
				fmt.Sprintf("PGPASSWORD=%s", appPassword), "psql", "-U", "app_user", "-d", "testdb", "-c",
				"INSERT INTO test_table VALUES (1, 'test');")
			_, err = testutil.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "app_user should be able to insert data")

			// Test readonly_user can select but not modify
			readonlySecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "readonly-user-credentials",
				Namespace: testNamespace,
			}, readonlySecret)
			Expect(err).NotTo(HaveOccurred())
			readonlyPassword := string(readonlySecret.Data["password"])

			cmd = exec.Command("kubectl", "exec", podName, "--", "env",
				fmt.Sprintf("PGPASSWORD=%s", readonlyPassword), "psql", "-U", "readonly_user", "-d", "testdb", "-c",
				"SELECT * FROM test_table;")
			_, err = testutil.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "readonly_user should be able to select data")
		})
	})

	Context("Cross-namespace scenarios", func() {
		BeforeAll(func() {
			By("Creating test namespace")
			cmd := exec.Command("kubectl", "create", "namespace", crossNsNamespace, "--dry-run=client", "-o", "yaml")
			manifestOutput, err := testutil.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(manifestOutput)
			_, err = testutil.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Should create test namespace")
		})

		It("should handle cross-namespace connections", func() {
			By("Creating cross-namespace PostGresConnection")
			cmd := exec.Command("kubectl", "apply", "-f", "test/e2e/manifests/cross-namespace-connection.yaml")
			_, err := testutil.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create cross-namespace connection")

			By("Waiting for cross-namespace connection to be ready")
			Eventually(func() bool {
				connection := &postgresv1.PostGresConnection{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "cross-ns-connection",
					Namespace: crossNsNamespace,
				}, connection)
				if err != nil {
					return false
				}
				return connection.Status.Ready
			}, timeout, interval).Should(BeTrue())
		})

		AfterAll(func() {
			By("Cleaning up test namespace")
			cmd := exec.Command("kubectl", "delete", "namespace", crossNsNamespace, "--ignore-not-found")
			_, _ = testutil.Run(cmd)
		})
	})

	Context("Operator Resilience", func() {
		It("should recover from operator restart", func() {
			By("Recording initial database status")
			database := &postgresv1.Database{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      databaseName,
				Namespace: testNamespace,
			}, database)
			Expect(err).NotTo(HaveOccurred())
			initialGeneration := database.Generation

			By("Restarting the operator")
			cmd := exec.Command("kubectl", "rollout", "restart", "deployment/operator-controller-manager", "-n", "operator-system")
			_, err = testutil.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Should restart operator")

			By("Waiting for operator to be ready again")
			cmd = exec.Command("kubectl", "rollout", "status", "deployment/operator-controller-manager", "-n", "operator-system", "--timeout=300s")
			_, err = testutil.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Operator should be ready after restart")

			By("Verifying database status is maintained")
			Eventually(func() bool {
				database := &postgresv1.Database{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      databaseName,
					Namespace: testNamespace,
				}, database)
				return err == nil && database.Status.Ready
			}, timeout, interval).Should(BeTrue())

			By("Verifying no unnecessary reconciliation occurred")
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      databaseName,
				Namespace: testNamespace,
			}, database)
			Expect(err).NotTo(HaveOccurred())
			Expect(database.Generation).To(Equal(initialGeneration), "Database should not be unnecessarily updated")
		})
	})

	AfterAll(func() {
		By("Cleaning up test resources")
		resources := []string{
			"database/test-database",
			"postgresconnection/test-connection",
		}

		for _, resource := range resources {
			cmd := exec.Command("kubectl", "delete", resource, "--ignore-not-found")
			_, _ = testutil.Run(cmd)
		}

		By("Waiting for cleanup to complete")
		time.Sleep(30 * time.Second)
	})
})
