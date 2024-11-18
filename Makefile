default: testacc

# Run unit tests
.PHONY: test
test:
	TF_ACC=0 go test ./... -v $(TESTARGS)

# Run acceptance tests
.PHONY: testacc
testacc:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m
