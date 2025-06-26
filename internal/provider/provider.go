package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"go.temporal.io/cloud-sdk/api/cloudservice/v1"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
)

// Ensure TerraformCloudProvider satisfies various provider interfaces.
var _ provider.Provider = &TerraformCloudProvider{}

// TerraformCloudProvider defines the provider implementation.
type TerraformCloudProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// TerraformCloudProviderModel describes the provider data model.
type TerraformCloudProviderModel struct {
	APIKey           types.String `tfsdk:"api_key"`
	Endpoint         types.String `tfsdk:"endpoint"`
	AllowInsecure    types.Bool   `tfsdk:"allow_insecure"`
	AllowedAccountID types.String `tfsdk:"allowed_account_id"`
}

func (p *TerraformCloudProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "temporalcloud"
	resp.Version = p.version
}

func (p *TerraformCloudProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Use the ` + "`" + `temporalcloud` + "`" + ` provider to interact with resources supported by [Temporal Cloud](https://temporal.io/cloud).

Use the navigation to the left to learn about the available resources supported by this provider.

~> This provider is in Public Preview, is under active development, and is subject to change. We reserve the right to make breaking changes during this pre-GA period, though we will do our best to maintain compatibility wherever possible.

## Provider Configuration

Credentials for Temporal Cloud can be provided by adding an ` + "`" + `api_key` + "`" + ` property or by setting the environment variable ` + "`" + `TEMPORAL_CLOUD_API_KEY` + "`" + `.
You can generate an API key for Temporal Cloud by following [these instructions](https://docs.temporal.io/cloud/api-keys).

!> Hard-coded credentials are not recommended in any Terraform configuration and should not be committed
in version control. We recommend passing credentials to this provider via environment variables.


`,
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "The API key for Temporal Cloud. See [this documentation](https://docs.temporal.io/cloud/api-keys) for information on how to obtain an API key.",
				Optional:            true,
				Sensitive:           true,
			},
			"endpoint": schema.StringAttribute{
				Description: "The endpoint for the Temporal Cloud API. Defaults to `saas-api.tmprl.cloud:443`.",
				Optional:    true,
			},
			"allow_insecure": schema.BoolAttribute{
				Description: "If set to True, it allows for an insecure connection to the Temporal Cloud API. This should never be set to 'true' in production and defaults to false.",
				Optional:    true,
			},
			"allowed_account_id": schema.StringAttribute{
				Description: "The ID of the account to operate on. Prevents accidental mutation of accounts other than that provided.",
				Optional:    true,
			},
		},
	}
}

func (p *TerraformCloudProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data TerraformCloudProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.APIKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Unknown Terraform Cloud API Key",
			"The provider cannot create a Terraform Cloud API client as there is an unknown configuration value for the Temporal Cloud API Key."+
				" Either apply the source of the value first, or statically set the API Key via environment variable or in configuration.")
		return
	}

	if data.Endpoint.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Unknown Terraform Cloud Endpoint",
			"The provider cannot create a Terraform Cloud API client as there is an unknown configuration value for the Temporal Cloud API Endpoint."+
				" Either apply the source of the value first, or statically set the Endpoint via environment variable or in configuration.")
	}

	if data.AllowInsecure.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("allow_insecure"),
			"Unknown Terraform Cloud Endpoint",
			"The provider cannot create a Terraform Cloud API client as there is an unknown configuration value for `allow_insecure`."+
				" Either apply the source of the value first, or statically set the allow_insecure flag via environment variable or in configuration.")
	}

	if data.AllowedAccountID.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("allowed_account_id"),
			"Unknown Terraform Cloud Allowed Account ID Value",
			"The provider cannot create a Terraform Cloud API client as there is an unknown configuration value for `allowed_account_id`."+
				" Either apply the source of the value first, or statically set the Allowed Account ID value via environment variable or in configuration.")
	}

	apiKey := os.Getenv("TEMPORAL_CLOUD_API_KEY")
	if !data.APIKey.IsNull() {
		apiKey = data.APIKey.ValueString()
	}

	endpoint := "saas-api.tmprl.cloud:443"
	if os.Getenv("TEMPORAL_CLOUD_ENDPOINT") != "" {
		endpoint = os.Getenv("TEMPORAL_CLOUD_ENDPOINT")
	}
	if !data.Endpoint.IsNull() {
		endpoint = data.Endpoint.ValueString()
	}

	allowInsecure := os.Getenv("TEMPORAL_CLOUD_ALLOW_INSECURE") == "true"
	if !data.AllowInsecure.IsNull() {
		allowInsecure = data.AllowInsecure.ValueBool()
	}

	allowedAccountID := os.Getenv("TEMPORAL_CLOUD_ALLOWED_ACCOUNT_ID")
	if !data.AllowedAccountID.IsNull() {
		allowedAccountID = data.AllowedAccountID.ValueString()
	}

	cc, err := client.NewConnectionWithAPIKey(endpoint, allowInsecure, apiKey, p.version)
	if err != nil {
		resp.Diagnostics.AddError("Failed to connect to Temporal Cloud API", err.Error())
		return
	}

	if allowedAccountID != "" {
		accountResp, err := cc.CloudService().GetAccount(ctx, &cloudservice.GetAccountRequest{})
		if err != nil {
			resp.Diagnostics.AddError("Failed to validate allowed account ID", fmt.Sprintf("failed to get account details: %s", err.Error()))
			return
		}
		currentAccountID := accountResp.GetAccount().GetId()
		if currentAccountID != allowedAccountID {
			resp.Diagnostics.AddError("Failed to validate allowed account ID", fmt.Sprintf("current account ID '%s' does not match allowed account ID '%s'", currentAccountID, allowedAccountID))
			return
		}
	}

	resp.DataSourceData = cc
	resp.ResourceData = cc
}

func (p *TerraformCloudProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewNamespaceResource,
		NewNamespaceSearchAttributeResource,
		NewNamespaceTagsResource,
		NewUserResource,
		NewServiceAccountResource,
		NewApiKeyResource,
		NewMetricsEndpointResource,
		NewNexusEndpointResource,
		NewNamespaceExportSinkResource,
		NewGroupAccessResource,
	}
}

func (p *TerraformCloudProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewRegionsDataSource,
		NewNamespacesDataSource,
		NewServiceAccountsDataSource,
		NewNamespaceDataSource,
		NewServiceAccountDataSource,
		NewSCIMGroupDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &TerraformCloudProvider{
			version: version,
		}
	}
}
