package provider

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	projectv1 "go.temporal.io/cloud-sdk/api/project/v1"
)

func TestProjectSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the resource.Resource and call its Schema method
	NewProjectResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestAccBasicProject(t *testing.T) {
	config := func(displayName string, extraAttrs string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_project" "terraform" {
  display_name = "%s"
  %s
}`, displayName, extraAttrs)
	}

	name := createRandomName()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_project.terraform", "display_name", name),
					resource.TestCheckResourceAttr("temporalcloud_project.terraform", "description", ""),
					resource.TestCheckResourceAttrSet("temporalcloud_project.terraform", "id"),
					resource.TestCheckResourceAttrSet("temporalcloud_project.terraform", "state"),
				),
			},
			{
				// display_name is mutable and must not force replacement.
				Config: config(name+"-renamed", `description = "This is a test description"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_project.terraform", "display_name", name+"-renamed"),
					resource.TestCheckResourceAttr("temporalcloud_project.terraform", "description", "This is a test description"),
				),
			},
			{
				// Dropping description falls back to the "" default.
				Config: config(name+"-renamed", ""),
				Check: resource.TestCheckResourceAttr(
					"temporalcloud_project.terraform", "description", ""),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_project.terraform",
			},
		},
	})
}

func TestAccProject_DeleteProtection(t *testing.T) {
	config := func(displayName string, enableDeleteProtection bool) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_project" "terraform" {
  display_name = "%s"
  project_lifecycle = {
    enable_delete_protection = %t
  }
}`, displayName, enableDeleteProtection)
	}

	name := createRandomName()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name, true),
				Check: resource.TestCheckResourceAttr(
					"temporalcloud_project.terraform", "project_lifecycle.enable_delete_protection", "true"),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_project.terraform",
			},
			{
				// Protection must be lifted before the test framework can destroy the Project.
				Config: config(name, false),
				Check: resource.TestCheckResourceAttr(
					"temporalcloud_project.terraform", "project_lifecycle.enable_delete_protection", "false"),
			},
		},
	})
}

func testProjectWaitConfig() waitForProjectAvailableConfig {
	return waitForProjectAvailableConfig{
		retryInterval: 10 * time.Millisecond,
		maxAttempts:   5,
	}
}

func testProjectResponse(id string) *cloudservicev1.GetProjectResponse {
	return &cloudservicev1.GetProjectResponse{
		Project: &projectv1.Project{
			Id:   id,
			Spec: &projectv1.ProjectSpec{DisplayName: "test-project"},
		},
	}
}

func TestWaitForProjectAvailableSuccess(t *testing.T) {
	callCount := 0
	getProjectFunc := func(ctx context.Context, req *cloudservicev1.GetProjectRequest) (*cloudservicev1.GetProjectResponse, error) {
		callCount++
		return testProjectResponse(req.GetProjectId()), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	project, err := waitForProjectAvailableWithConfig(ctx, getProjectFunc, "test-project-id", testProjectWaitConfig())
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if project == nil {
		t.Fatal("Expected project, got nil")
	}
	if project.GetId() != "test-project-id" {
		t.Errorf("Expected project ID 'test-project-id', got '%s'", project.GetId())
	}
	if callCount != 1 {
		t.Errorf("Expected 1 API call, got %d", callCount)
	}
}

// PermissionDenied is the code the API returns while a freshly created Project's access
// propagates to the caller, so it must be retried rather than surfaced.
func TestWaitForProjectAvailableRetriesOnPermissionDenied(t *testing.T) {
	attempts := 0
	getProjectFunc := func(ctx context.Context, req *cloudservicev1.GetProjectRequest) (*cloudservicev1.GetProjectResponse, error) {
		attempts++
		if attempts < 3 {
			return nil, status.Error(codes.PermissionDenied, "permission denied")
		}
		return testProjectResponse(req.GetProjectId()), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	project, err := waitForProjectAvailableWithConfig(ctx, getProjectFunc, "test-project-id", testProjectWaitConfig())
	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}
	if project == nil {
		t.Fatal("Expected project, got nil")
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestWaitForProjectAvailableRetriesOnNotFound(t *testing.T) {
	attempts := 0
	getProjectFunc := func(ctx context.Context, req *cloudservicev1.GetProjectRequest) (*cloudservicev1.GetProjectResponse, error) {
		attempts++
		if attempts < 2 {
			return nil, status.Error(codes.NotFound, "not found")
		}
		return testProjectResponse(req.GetProjectId()), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	project, err := waitForProjectAvailableWithConfig(ctx, getProjectFunc, "test-project-id", testProjectWaitConfig())
	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}
	if project == nil {
		t.Fatal("Expected project, got nil")
	}
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestWaitForProjectAvailableFailsOnNonRetryableError(t *testing.T) {
	attempts := 0
	getProjectFunc := func(ctx context.Context, req *cloudservicev1.GetProjectRequest) (*cloudservicev1.GetProjectResponse, error) {
		attempts++
		return nil, status.Error(codes.InvalidArgument, "bad request")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	project, err := waitForProjectAvailableWithConfig(ctx, getProjectFunc, "test-project-id", testProjectWaitConfig())
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if project != nil {
		t.Error("Expected nil project on error")
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt for a non-retryable error, got %d", attempts)
	}
}

func TestWaitForProjectAvailableMaxAttemptsReached(t *testing.T) {
	attempts := 0
	getProjectFunc := func(ctx context.Context, req *cloudservicev1.GetProjectRequest) (*cloudservicev1.GetProjectResponse, error) {
		attempts++
		return nil, status.Error(codes.PermissionDenied, "permission denied")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config := testProjectWaitConfig()
	_, err := waitForProjectAvailableWithConfig(ctx, getProjectFunc, "test-project-id", config)
	if err == nil {
		t.Fatal("Expected error after exhausting attempts, got nil")
	}
	if attempts != config.maxAttempts {
		t.Errorf("Expected %d attempts, got %d", config.maxAttempts, attempts)
	}
}

func TestWaitForProjectAvailableContextTimeout(t *testing.T) {
	getProjectFunc := func(ctx context.Context, req *cloudservicev1.GetProjectRequest) (*cloudservicev1.GetProjectResponse, error) {
		return nil, status.Error(codes.PermissionDenied, "permission denied")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()

	_, err := waitForProjectAvailableWithConfig(ctx, getProjectFunc, "test-project-id", waitForProjectAvailableConfig{
		retryInterval: 50 * time.Millisecond,
		maxAttempts:   100,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Expected context.DeadlineExceeded, got %v", err)
	}
}
