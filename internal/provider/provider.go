package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

// TerraformCloudProvider describes the provider data model.
type TerraformCloudProviderModel struct {
	APIKey    types.String `tfsdk:"api_key"`
	AccountID types.String `tfsdk:"account_id"`
}

func (p *TerraformCloudProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "temporalcloud"
	resp.Version = p.version
}

func (p *TerraformCloudProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
			},
			"account_id": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
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

	if data.AccountID.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("account_id"),
			"Unknown Terraform Cloud Account ID",
			"The provider cannot create a Terraform Cloud API client as there is an unknown configuration value for the Temporal Cloud Account ID."+
				" Either apply the source of the value first, or statically set the Account ID via environment variable or in configuration.")
		return
	}

	apiKey := os.Getenv("TEMPORAL_CLOUD_API_KEY")
	accountID := os.Getenv("TEMPORAL_CLOUD_ACCOUNT_ID")
	if !data.APIKey.IsNull() {
		apiKey = data.APIKey.ValueString()
	}
	if !data.AccountID.IsNull() {
		accountID = data.AccountID.ValueString()
	}

	if accountID == "" {
		resp.Diagnostics.AddError("Failed to connect to Temporal Cloud API", "An Account ID is required")
		return
	}

	client, err := NewClient(apiKey, accountID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to connect to Temporal Cloud API", err.Error())
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *TerraformCloudProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewExampleResource,
		NewNamespaceResource,
	}
}

func (p *TerraformCloudProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &TerraformCloudProvider{
			version: version,
		}
	}
}
