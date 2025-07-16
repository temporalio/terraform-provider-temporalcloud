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

test-connectivity-rule:
	TF_ACC=1 go test ./internal/provider -run TestAccNamespaceWithCodecServer -v $(TESTARGS) -timeout 120m