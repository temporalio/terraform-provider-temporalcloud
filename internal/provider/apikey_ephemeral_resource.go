package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type (
	apiKeyEphemeralResource struct {
		client *client.Client
	}

	apiKeyEphemeralResourceModel struct {
		OwnerType   types.String `tfsdk:"owner_type"`
		OwnerID     types.String `tfsdk:"owner_id"`
		DisplayName types.String `tfsdk:"display_name"`
		Description types.String `tfsdk:"description"`
		ExpiryTime  types.String `tfsdk:"expiry_time"`
		Disabled    types.Bool   `tfsdk:"disabled"`
		// Computed outputs
		ID    types.String `tfsdk:"id"`
		State types.String `tfsdk:"state"`
		Token types.String `tfsdk:"token"`
	}

	// privateData stores API key info for cleanup in Close
	apiKeyPrivateData struct {
		KeyID           string `json:"key_id"`
		ResourceVersion string `json:"resource_version"`
	}
)

var (
	_ ephemeral.EphemeralResource              = (*apiKeyEphemeralResource)(nil)
	_ ephemeral.EphemeralResourceWithConfigure = (*apiKeyEphemeralResource)(nil)
	_ ephemeral.EphemeralResourceWithClose     = (*apiKeyEphemeralResource)(nil)
)

func NewApiKeyEphemeralResource() ephemeral.EphemeralResource {
	return &apiKeyEphemeralResource{}
}

func (r *apiKeyEphemeralResource) Configure(_ context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *apiKeyEphemeralResource) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_apikey"
}

func (r *apiKeyEphemeralResource) Schema(ctx context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Creates a temporary Temporal Cloud API key that is not stored in Terraform state. The API key is created when the resource is opened and deleted when the Terraform operation completes.",
		Attributes: map[string]schema.Attribute{
			"owner_type": schema.StringAttribute{
				Description: "The type of the owner to create the API key for. Must be either 'user' or 'service-account'.",
				Required:    true,
			},
			"owner_id": schema.StringAttribute{
				Description: "The ID of the owner to create the API key for.",
				Required:    true,
			},
			"display_name": schema.StringAttribute{
				Description: "The display name for the API key.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "The description for the API key.",
				Optional:    true,
			},
			"expiry_time": schema.StringAttribute{
				Description: "The expiry time for the API key in ISO 8601 format (RFC3339).",
				Required:    true,
			},
			"disabled": schema.BoolAttribute{
				Description: "Whether the API key is disabled. Defaults to false.",
				Optional:    true,
			},
			// Computed outputs
			"id": schema.StringAttribute{
				Description: "The unique identifier of the API key.",
				Computed:    true,
			},
			"state": schema.StringAttribute{
				Description: "The current state of the API key.",
				Computed:    true,
			},
			"token": schema.StringAttribute{
				Description: "The secret token for the API key. This value is only available when the key is created and is never stored in Terraform state.",
				Computed:    true,
				Sensitive:   true,
			},
		},
	}
}

func (r *apiKeyEphemeralResource) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var config apiKeyEphemeralResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse the expiry time
	expiryTimeString := config.ExpiryTime.ValueString()
	expiryTime, err := time.Parse(time.RFC3339, expiryTimeString)
	if err != nil {
		resp.Diagnostics.AddError("Invalid ExpiryTime", "Could not parse ExpiryTime: "+err.Error())
		return
	}
	expiryTimestamp := timestamppb.New(expiryTime)

	description := ""
	if !config.Description.IsNull() {
		description = config.Description.ValueString()
	}

	disabled := false
	if !config.Disabled.IsNull() {
		disabled = config.Disabled.ValueBool()
	}

	ownerType, err := enums.ToOwnerType(config.OwnerType.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid owner_type", err.Error())
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider has not been configured. Please ensure the provider block is properly configured with credentials.",
		)
		return
	}

	tflog.Debug(ctx, "Creating ephemeral API key", map[string]interface{}{
		"owner_type":   config.OwnerType.ValueString(),
		"owner_id":     config.OwnerID.ValueString(),
		"display_name": config.DisplayName.ValueString(),
	})

	svcResp, err := r.client.CloudService().CreateApiKey(ctx, &cloudservicev1.CreateApiKeyRequest{
		Spec: &identityv1.ApiKeySpec{
			OwnerId:     config.OwnerID.ValueString(),
			OwnerType:   ownerType,
			DisplayName: config.DisplayName.ValueString(),
			Description: description,
			ExpiryTime:  expiryTimestamp,
			Disabled:    disabled,
		},
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create API key", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to create API key", err.Error())
		return
	}

	apiKey, err := r.client.CloudService().GetApiKey(ctx, &cloudservicev1.GetApiKeyRequest{
		KeyId: svcResp.GetKeyId(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get API key after creation", err.Error())
		return
	}

	// Update the model with computed values
	stateStr, err := enums.FromResourceState(apiKey.ApiKey.GetState())
	if err != nil {
		resp.Diagnostics.AddError("Failed to convert API key state", err.Error())
		return
	}

	config.ID = types.StringValue(apiKey.ApiKey.GetId())
	config.State = types.StringValue(stateStr)
	config.Token = types.StringValue(svcResp.Token)

	resp.Diagnostics.Append(resp.Result.Set(ctx, &config)...)

	// Store private data for cleanup in Close
	privateData := apiKeyPrivateData{
		KeyID:           apiKey.ApiKey.GetId(),
		ResourceVersion: apiKey.ApiKey.GetResourceVersion(),
	}
	privateDataBytes, err := json.Marshal(privateData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to marshal private data", err.Error())
		return
	}
	resp.Private.SetKey(ctx, "apikey", privateDataBytes)

	tflog.Info(ctx, "Created ephemeral API key", map[string]interface{}{
		"id": apiKey.ApiKey.GetId(),
	})
}

func (r *apiKeyEphemeralResource) Close(ctx context.Context, req ephemeral.CloseRequest, resp *ephemeral.CloseResponse) {
	// Retrieve private data
	privateDataBytes, diags := req.Private.GetKey(ctx, "apikey")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || len(privateDataBytes) == 0 {
		return
	}

	var privateData apiKeyPrivateData
	if err := json.Unmarshal(privateDataBytes, &privateData); err != nil {
		resp.Diagnostics.AddError("Failed to unmarshal private data", err.Error())
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"Cannot clean up ephemeral API key: provider client is not available.",
		)
		return
	}

	tflog.Debug(ctx, "Deleting ephemeral API key", map[string]interface{}{
		"id": privateData.KeyID,
	})

	// Get current resource version in case it changed
	apiKey, err := r.client.CloudService().GetApiKey(ctx, &cloudservicev1.GetApiKeyRequest{
		KeyId: privateData.KeyID,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			tflog.Warn(ctx, "Ephemeral API key already deleted", map[string]interface{}{
				"id": privateData.KeyID,
			})
			return
		}
		resp.Diagnostics.AddError("Failed to get API key for deletion", err.Error())
		return
	}

	svcResp, err := r.client.CloudService().DeleteApiKey(ctx, &cloudservicev1.DeleteApiKeyRequest{
		KeyId:            privateData.KeyID,
		ResourceVersion:  apiKey.ApiKey.GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			tflog.Warn(ctx, "Ephemeral API key already deleted", map[string]interface{}{
				"id": privateData.KeyID,
			})
			return
		}
		resp.Diagnostics.AddError("Failed to delete ephemeral API key", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to delete ephemeral API key", err.Error())
		return
	}

	tflog.Info(ctx, "Deleted ephemeral API key", map[string]interface{}{
		"id": privateData.KeyID,
	})
}
