default: testacc

# Run unit tests
.PHONY: test
test:
	TF_ACC="" go test ./... -v $(TESTARGS)

# Run acceptance tests
.PHONY: testacc
testacc:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

# Example: Run specific namespace export sink tests
.PHONY: test-namespace-export-sink
test-namespace-export-sink:
	TF_ACC=1 go test ./internal/provider -run TestAccNamespaceExportSink_GCS -v $(TESTARGS) -timeout 120m

# Run the Project resource acceptance tests. Requires an account with Projects enabled.
.PHONY: test-project
test-project:
	TF_ACC=1 go test ./internal/provider -run 'TestAccBasicProject|TestAccProject_DeleteProtection' -v $(TESTARGS) -timeout 30m

# Run the project_accesses acceptance tests across users, service accounts, and groups.
.PHONY: test-project-access
test-project-access:
	TF_ACC=1 go test ./internal/provider -run 'TestAccBasicUserWithProjectAccesses|TestAccServiceAccountWithProjectAccesses|TestAccGroupAccess_WithProjectAccesses' -v $(TESTARGS) -timeout 30m
