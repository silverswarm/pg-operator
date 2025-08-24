---
name: Bug report
about: Create a report to help us improve
title: '[BUG] '
labels: bug
assignees: ''

---

**Describe the bug**
A clear and concise description of what the bug is.

**To Reproduce**
Steps to reproduce the behavior:
1. Deploy the operator with '...'
2. Create a PostGresConnection '...'
3. Create a Database '...'
4. See error

**Expected behavior**
A clear and concise description of what you expected to happen.

**Environment:**
 - Kubernetes version: [e.g. 1.28.0]
 - CloudNativePG version: [e.g. 1.21.0]
 - PostgreSQL Operator version: [e.g. 1.0.0]
 - PostgreSQL version: [e.g. 15.4]

**YAML manifests:**
```yaml
# PostGresConnection
apiVersion: postgres.silverswarm.io/v1
kind: PostGresConnection
metadata:
  name: example
spec:
  # your configuration

---
# Database
apiVersion: postgres.silverswarm.io/v1
kind: Database
metadata:
  name: example
spec:
  # your configuration
```

**Logs:**
```
# Operator logs
kubectl logs -n pg-operator-system deploy/pg-operator-controller-manager

# PostgreSQL logs (if relevant)
kubectl logs -n postgres-system postgres-cluster-1
```

**Additional context**
Add any other context about the problem here.