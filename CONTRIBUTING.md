# Contributing to Temporal Cloud Terraform Provider
This doc is for contributors to Temporal Cloud Terraform Provider (hopefully that's you!)

Note: All contributors also need to fill out the [Temporal Contributor License Agreement (CLA)](https://gist.github.com/samarabbas/7dcd41eb1d847e12263cc961ccfdb197) before we can merge in any of your changes.

We appreciate your interest in contributing to the Temporal Cloud Terraform Provider! This document provides guidelines for contributing, including setting up your development environment, running tests, and adhering to coding standards.

## Prerequisites

Before contributing, please ensure you have the following prerequisites installed:

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.19
- [Temporal Go SDK](https://github.com/temporalio/sdk-go) >= 1.26.0
- [Docker](https://docs.docker.com/get-docker/), for running acceptance tests


## Development Setup

### Cloning the repository

To begin working on the provider, clone the repository into your development environment:

```bash
git clone https://github.com/temporalio/terraform-provider-temporalcloud.git
cd terraform-provider-temporalcloud
```

### Building the provider

To build the provider, use the Go `install` command:

```bash
go install
```

This will install the provider binary in your `$GOPATH/bin` directory.

### Running Tests

#### Unit Tests

Unit tests should be executed frequently to ensure the quality of your contributions. Run the following command to execute the tests:

```bash
make test
```

#### Acceptance Tests

Running acceptance tests locally will create real resources in Temporal Cloud, and running them may incur costs. Ensure you have a Temporal Cloud API key configured as described in the [README](https://registry.terraform.io/providers/temporalio/temporalcloud/latest/docs) before running these tests:

```bash
make testacc
```

#### Testing with Terraform

To test your code locally with Terraform, build the provider locally:

```bash
go build -o terraform-provider-temporalcloud
mkdir -p ~/.terraform.d/plugins/temporal.io/provider-temporalcloud/temporalcloud/1.0.0/darwin_arm64/
mv terraform-provider-temporalcloud ~/.terraform.d/plugins/temporal.io/provider-temporalcloud/temporalcloud/1.0.0/darwin_arm64/
```

In your Terraform configuration, specify the local provider binary:

```hcl
provider "temporalcloud" {
  api_key = "<API_KEY>"
}

terraform {
  required_providers {
    temporalcloud = {
      version = "~> 1.0.0"
      source  = "temporal.io/provider-temporalcloud/temporalcloud"
    }
  }
}
```

## Documentation

Documentation for resources and data sources is defined in the code (see `MarkdownDescription`) and generated programatically. To update the documentation:

1. Make code changes.
2. Run the following commands to format the code and generate documentation:

```bash
gofmt -s -w .
go generate ./...
```

## Adding Dependencies

This provider uses Go modules for dependency management. To add a new dependency, run the following commands:

```bash
go get github.com/author/dependency
go mod tidy
```

Make sure to commit the changes to both `go.mod` and `go.sum`.

## Code Style

All code contributions should follow the established code style. Use `gofmt` to automatically format your code:

```bash
gofmt -s -w .
```

## Commit Messages and Pull Requests

We follow the [Chris Beams](https://chris.beams.io/posts/git-commit/) style for writing commit messages. Please ensure:

- Commit message titles are capitalized and concise.
- Pull request titles follow the same guidelines and avoid generic phrases like "bug fixes."