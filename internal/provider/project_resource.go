package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	projectv1 "go.temporal.io/cloud-sdk/api/project/v1"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"
	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
)

type (
	projectResource struct {
		client *client.Client
	}

	projectResourceModel struct {
		ID          types.String                  `tfsdk:"id"`
		State       types.String                  `tfsdk:"state"`
		DisplayName types.String                  `tfsdk:"display_name"`
		Description types.String                  `tfsdk:"description"`
		Lifecycle   internaltypes.ZeroObjectValue `tfsdk:"project_lifecycle"`

		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}

	projectLifecycleModel struct {
		EnableDeleteProtection types.Bool `tfsdk:"enable_delete_protection"`
	}
)

var (
	_ resource.Resource                = (*projectResource)(nil)
	_ resource.ResourceWithConfigure   = (*projectResource)(nil)
	_ resource.ResourceWithImportState = (*projectResource)(nil)

	projectLifecycleAttrs = map[string]attr.Type{
		"enable_delete_protection": types.BoolType,
	}
)

func NewProjectResource() resource.Resource {
	return &projectResource{}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *projectResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provisions a Temporal Cloud Project. Projects are an upcoming feature and may not be enabled for your account; contact Temporal support to request access.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the Project.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Description: "The current state of the Project.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"display_name": schema.StringAttribute{
				Description: "The display name of the Project.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"description": schema.StringAttribute{
				Description: "The description of the Project.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"project_lifecycle": schema.SingleNestedAttribute{
				Description: "The lifecycle configuration for the Project. Note that this is different from the Terraform resource lifecycle. This controls settings like delete protection within Temporal Cloud.",
				CustomType: internaltypes.ZeroObjectType{
					ObjectType: basetypes.ObjectType{
						AttrTypes: projectLifecycleAttrs,
					},
				},
				Attributes: map[string]schema.Attribute{
					"enable_delete_protection": schema.BoolAttribute{
						Description: "If true, the Project cannot be deleted. This is a safeguard against accidental deletion. To delete a Project with this option enabled, you must first set it to false.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
				},
				Optional: true,
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, defaultCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	spec, d := getProjectSpecFromModel(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	svcResp, err := r.client.CloudService().CreateProject(ctx, &cloudservicev1.CreateProjectRequest{
		Spec:             spec,
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Project", err.Error())
		return
	}
	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to create Project", err.Error())
		return
	}

	project, err := waitForProjectAvailable(ctx, r.client, svcResp.GetProjectId())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get Project after creation", err.Error())
		return
	}

	resp.Diagnostics.Append(updateProjectModelFromSpec(ctx, &plan, project)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, err := r.client.CloudService().GetProject(ctx, &cloudservicev1.GetProjectRequest{
		ProjectId: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Project Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to get Project", err.Error())
		return
	}

	resp.Diagnostics.Append(updateProjectModelFromSpec(ctx, &state, project.GetProject())...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, defaultCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	currentProject, err := r.client.CloudService().GetProject(ctx, &cloudservicev1.GetProjectRequest{
		ProjectId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current Project status", err.Error())
		return
	}

	spec, d := getProjectSpecFromModel(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	svcResp, err := r.client.CloudService().UpdateProject(ctx, &cloudservicev1.UpdateProjectRequest{
		ProjectId:        plan.ID.ValueString(),
		Spec:             spec,
		ResourceVersion:  currentProject.GetProject().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update Project", err.Error())
		return
	}
	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to update Project", err.Error())
		return
	}

	project, err := r.client.CloudService().GetProject(ctx, &cloudservicev1.GetProjectRequest{
		ProjectId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get Project after update", err.Error())
		return
	}

	resp.Diagnostics.Append(updateProjectModelFromSpec(ctx, &plan, project.GetProject())...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	currentProject, err := r.client.CloudService().GetProject(ctx, &cloudservicev1.GetProjectRequest{
		ProjectId: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Project Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to get current Project status", err.Error())
		return
	}

	// Delete protection is enforced server-side, but the resulting error does not explain how to
	// proceed. Check against the live spec rather than Terraform state so that protection toggled
	// outside of Terraform is still respected.
	if currentProject.GetProject().GetSpec().GetLifecycle().GetEnableDeleteProtection() {
		resp.Diagnostics.AddError(
			"Failed to delete Project",
			fmt.Sprintf("Project %s has delete protection enabled. Set project_lifecycle.enable_delete_protection to false and apply before destroying the Project.", state.ID.ValueString()),
		)
		return
	}

	svcResp, err := r.client.CloudService().DeleteProject(ctx, &cloudservicev1.DeleteProjectRequest{
		ProjectId:        state.ID.ValueString(),
		ResourceVersion:  currentProject.GetProject().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Project Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to delete Project", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to delete Project", err.Error())
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// waitForProjectAvailableConfig contains configuration for polling behavior.
type waitForProjectAvailableConfig struct {
	retryInterval time.Duration
	maxAttempts   int
}

// defaultWaitForProjectAvailableConfig returns the default polling configuration.
func defaultWaitForProjectAvailableConfig() waitForProjectAvailableConfig {
	return waitForProjectAvailableConfig{
		retryInterval: 10 * time.Second,
		maxAttempts:   12,
	}
}

func waitForProjectAvailable(ctx context.Context, c *client.Client, projectID string) (*projectv1.Project, error) {
	getProjectFunc := func(ctx context.Context, req *cloudservicev1.GetProjectRequest) (*cloudservicev1.GetProjectResponse, error) {
		return c.CloudService().GetProject(ctx, req)
	}
	return waitForProjectAvailableWithConfig(ctx, getProjectFunc, projectID, defaultWaitForProjectAvailableConfig())
}

// waitForProjectAvailableWithConfig polls GetProject until the Project is readable. A freshly
// created Project is not immediately visible to the caller: the API reports PermissionDenied (not
// only NotFound) while the caller's access to it propagates, even after the create async operation
// reports fulfilled. This mirrors waitForNamespaceAvailable.
func waitForProjectAvailableWithConfig(ctx context.Context, getProjectFunc func(context.Context, *cloudservicev1.GetProjectRequest) (*cloudservicev1.GetProjectResponse, error), projectID string, config waitForProjectAvailableConfig) (*projectv1.Project, error) {
	ctx = tflog.SetField(ctx, "project_id", projectID)

	retryInterval := config.retryInterval
	maxAttempts := config.maxAttempts

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		tflog.Debug(ctx, "attempting to get project", map[string]any{
			"attempt": attempt,
		})

		project, err := getProjectFunc(ctx, &cloudservicev1.GetProjectRequest{
			ProjectId: projectID,
		})

		if err == nil {
			tflog.Debug(ctx, "project successfully retrieved")
			return project.GetProject(), nil
		}

		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.NotFound {
			tflog.Debug(ctx, "project not yet accessible, retrying", map[string]any{
				"attempt":  attempt,
				"retry_in": retryInterval.String(),
				"error":    err.Error(),
			})
		} else {
			tflog.Error(ctx, "failed to get project with non-retryable error", map[string]any{
				"attempt": attempt,
				"error":   err.Error(),
			})
			return nil, fmt.Errorf("failed to get project: %w", err)
		}

		if attempt >= maxAttempts {
			break
		}

		// Wait before next retry, respecting context cancellation
		select {
		case <-time.After(retryInterval):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("project %s not available after %d attempts", projectID, maxAttempts)
}

func getProjectSpecFromModel(ctx context.Context, plan *projectResourceModel) (*projectv1.ProjectSpec, diag.Diagnostics) {
	var diags diag.Diagnostics

	spec := &projectv1.ProjectSpec{
		DisplayName: plan.DisplayName.ValueString(),
		Description: plan.Description.ValueString(),
	}

	if !plan.Lifecycle.IsNull() && !plan.Lifecycle.IsZero(ctx) {
		lifecycle, d := getProjectLifecycleFromModel(ctx, plan)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		spec.Lifecycle = lifecycle
	}

	return spec, diags
}

func getProjectLifecycleFromModel(ctx context.Context, model *projectResourceModel) (*projectv1.LifecycleSpec, diag.Diagnostics) {
	var diags diag.Diagnostics
	var lifecycle projectLifecycleModel
	diags.Append(model.Lifecycle.As(ctx, &lifecycle, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}
	return &projectv1.LifecycleSpec{
		EnableDeleteProtection: lifecycle.EnableDeleteProtection.ValueBool(),
	}, diags
}

func updateProjectModelFromSpec(ctx context.Context, state *projectResourceModel, project *projectv1.Project) diag.Diagnostics {
	var diags diag.Diagnostics

	stateStr, err := enums.FromResourceState(project.GetState())
	if err != nil {
		diags.AddError("Failed to convert resource state", err.Error())
		return diags
	}

	state.ID = types.StringValue(project.GetId())
	state.State = types.StringValue(stateStr)
	state.DisplayName = types.StringValue(project.GetSpec().GetDisplayName())
	state.Description = types.StringValue(project.GetSpec().GetDescription())

	if lifecycleSpec := project.GetSpec().GetLifecycle(); lifecycleSpec != nil && !proto.Equal(lifecycleSpec, &projectv1.LifecycleSpec{}) {
		lifecycle := &projectLifecycleModel{
			EnableDeleteProtection: types.BoolValue(lifecycleSpec.GetEnableDeleteProtection()),
		}
		obj, objectDiags := types.ObjectValueFrom(ctx, projectLifecycleAttrs, lifecycle)
		diags.Append(objectDiags...)
		if diags.HasError() {
			return diags
		}
		state.Lifecycle = internaltypes.ZeroObjectValue{ObjectValue: obj}
	} else if !state.Lifecycle.IsZero(ctx) {
		// only update the lifecycle if it's not already set to zero
		state.Lifecycle = internaltypes.ZeroObjectValue{ObjectValue: types.ObjectNull(projectLifecycleAttrs)}
	}

	return diags
}
