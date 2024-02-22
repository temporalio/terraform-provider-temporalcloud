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

// TerraformCloudProvider describes the provider data model.
type TerraformCloudProviderModel struct {
	APIKey        types.String `tfsdk:"api_key"`
	Endpoint      types.String `tfsdk:"endpoint"`
	AllowInsecure types.Bool   `tfsdk:"allow_insecure"`
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
			"endpoint": schema.StringAttribute{
				Optional: true,
			},
			"allow_insecure": schema.BoolAttribute{
				Optional: true,
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
				" Either apply the source of the value first, or statically set the API Key via environment variable or in configuration.")
	}

	if data.AllowInsecure.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("allow_insecure"),
			"Unknown Terraform Cloud Endpoint",
			"The provider cannot create a Terraform Cloud API client as there is an unknown configuration value for `allow_insecure`."+
				" Either apply the source of the value first, or statically set the API Key via environment variable or in configuration.")
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

	client, err := client.NewConnectionWithAPIKey(endpoint, allowInsecure, apiKey)
	if err != nil {
		resp.Diagnostics.AddError("Failed to connect to Temporal Cloud API", err.Error())
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *TerraformCloudProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewNamespaceResource,
		NewNamespaceSearchAttributeResource,
	}
}

func (p *TerraformCloudProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewRegionsDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &TerraformCloudProvider{
			version: version,
		}
	}
}
