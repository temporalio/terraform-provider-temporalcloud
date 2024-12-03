# Contributing to Temporal Cloud Terraform Provider

Thank you for your interest in contributing to the Temporal Cloud Terraform Provider! This document provides guidelines for contributions, including setting up your development environment, contributing to the codebase, and expectations for community contributions.

## Encouraging Contributions

We actively encourage contributions in the form of issues and pull requests (PRs). Whether you've found a bug, want to request a feature, or contribute new functionality, we welcome all participation. Your input helps improve the quality and usability of the Temporal Cloud Terraform Provider for everyone.

For those looking to get started for the first time pls see the issues labeled [good first issue](https://github.com/temporalio/terraform-provider-temporalcloud/labels/good%20first%20issue) 

### Opening Issues

If you encounter a bug, have a feature request, or need clarification on some part of the codebase, please open an issue. When doing so, ensure that:

- You provide a clear description of the issue.
- Include any relevant logs, error messages, or reproduction steps (if applicable).
- If possible, offer suggestions on how the problem might be solved.
- Be sure not to post any company sensitive data in your issue. 

### Submitting Pull Requests (PRs)

We welcome all PRs. Before submitting a PR, please ensure that:

- Your code changes are well-tested.
- The PR includes a clear description of what you're addressing and why.
- If the PR introduces new functionality, appropriate tests are added.
- Run Acceptance Tests locally before submitted a non-Draft PR to ensure they pass.  See the Running Tests section located below for details.

Feel free to submit a draft PR early if you need feedback or assistance during the development process. This can help identify potential improvements or issues early on.

Note: When you submit your first PR, you will be asked to sign the [Temporal Contributor License Agreement (CLA)](https://gist.github.com/samarabbas/7dcd41eb1d847e12263cc961ccfdb197) before we merge your PR.

Note 2: The Temporal Terraform Provider repo uses an automated test suite to run Acceptance Tests using an API Key. For security reasons, PR contributed by non Temporal team members aren't able to run the automated test suite. Before your PR is merged, a Temporal team member will likely run Acceptance Tests locally and may fork your PR in the process and resubmit the PR using the Temporal created fork. 


## Issues and Pull Requests Lifecycle

- **Issues**: This is an open source project. Issues can be resolved by any community member. The maintainers of this project do triage issues regularly to ensure the issue is clear and tagged appropriately. If more information is needed, we will ask for further clarification. We encourage discussion to clarify the problem or refine solutions. 
  
- **Pull Requests**: Once a PR is submitted, it will be reviewed by the maintainers. Feedback will be provided, and the contributor may be asked to make changes. Once all feedback is addressed and the PR passes the necessary tests, it will be merged. Please note that complex changes or large PRs may take longer to review.

## Expectations for Reviews and Issue Triage

While we strive to review issues/PRs and merge contributions as quickly as possible, we operate on a **best-effort basis**. There are no guarantees on review times, and no service level agreements (SLAs) are provided. We appreciate your patience and understanding as maintainers work through the queue of issues and pull requests.

## Prerequisites

Before contributing, please ensure you have the following prerequisites installed:

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.19
- [Temporal Go SDK](https://github.com/temporalio/sdk-go) >= 1.26.0
- [Docker](https://docs.docker.com/get-docker/), for running acceptance tests

## Development Setup

### Cloning the repository

To begin working on the provider, clone the repository into your development environment:

```sh
git clone https://github.com/temporalio/terraform-provider-temporalcloud.git  
cd terraform-provider-temporalcloud
```

### Building the provider

To build the provider, use the Go `install` command:

```sh
go install
```

This will install the provider binary in your `$GOPATH/bin` directory.

### Running Tests

The following is guidance for running tests with the TF Provider. 

#### Unit Tests

Unit tests should be executed frequently to ensure the quality of your contributions. Run the following command to execute the tests:

```sh
make test
```

#### Acceptance Tests

Running acceptance tests locally will create real resources in Temporal Cloud, and running them may incur costs. Ensure you have a Temporal Cloud API key configured as described in the [README](https://registry.terraform.io/providers/temporalio/temporalcloud/latest/docs) before running these tests:

```sh
make testacc
```

#### Testing with Terraform

To test your code locally with Terraform, build the provider locally:

```sh
go build -o terraform-provider-temporalcloud  
mkdir -p ~/.terraform.d/plugins/temporal.io/provider-temporalcloud/temporalcloud/1.0.0/darwin_arm64/  
mv terraform-provider-temporalcloud ~/.terraform.d/plugins/temporal.io/provider-temporalcloud/temporalcloud/1.0.0/darwin_arm64/
```

In your Terraform configuration, specify the local provider binary:

```terraform
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

Documentation for resources and data sources is defined in the code (see `MarkdownDescription`) and generated programmatically. To update the documentation:

1. Make code changes.
2. Run the following commands to format the code and generate documentation:

```sh
gofmt -s -w .  
go generate ./...
```

## Adding Dependencies

This provider uses Go modules for dependency management. To add a new dependency, run the following commands:

```sh
go get github.com/author/dependency  
go mod tidy
```

Make sure to commit the changes to both `go.mod` and `go.sum`.

## Code Style

All code contributions should follow the established code style. Use `gofmt` to automatically format your code:

```sh
gofmt -s -w .
```

## Commit Messages and Pull Requests

We follow the [Chris Beams](https://chris.beams.io/posts/git-commit/) style for writing commit messages. Please ensure:

- Commit message titles are capitalized and concise.
- Pull request titles follow the same guidelines and avoid generic phrases like "bug fixes."
