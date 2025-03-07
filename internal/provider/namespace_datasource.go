package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"
)

var (
	_ datasource.DataSource              = &namespaceDataSource{}
	_ datasource.DataSourceWithConfigure = &namespaceDataSource{}
)

func NewNamespaceDataSource() datasource.DataSource {
	return &namespaceDataSource{}
}

type namespaceDataSource struct {
	client *client.Client
}

func (d *namespaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = client
}

func (d *namespaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace"
}

func (d *namespaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "The unique identifier of the namespace across all Temporal Cloud tenants.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "The name of the namespace.",
			},
			"state": schema.StringAttribute{
				Computed:    true,
				Description: "The current state of the namespace.",
			},
			"active_region": schema.StringAttribute{
				Computed:    true,
				Description: "The currently active region for the namespace.",
			},
			"regions": schema.ListAttribute{
				Computed:    true,
				Description: "The list of regions that this namespace is available in. If more than one region is specified, this namespace is a Multi-region Namespace, which is currently unsupported by the Terraform provider.",
				ElementType: types.StringType,
			},
			"accepted_client_ca": schema.StringAttribute{
				Computed:    true,
				Description: "The Base64-encoded CA cert in PEM format that clients use when authenticating with Temporal Cloud.",
			},
			"retention_days": schema.Int64Attribute{
				Computed:    true,
				Description: "The number of days to retain workflow history. Any changes to the retention period will be applied to all new running workflows.",
			},
			"certificate_filters": schema.ListNestedAttribute{
				Computed:    true,
				Optional:    true,
				Description: "A list of filters to apply to client certificates when initiating a connection Temporal Cloud. If present, connections will only be allowed from client certificates whose distinguished name properties match at least one of the filters.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"common_name": schema.StringAttribute{
							Computed:    true,
							Description: "The certificate's common name.",
						},
						"organization": schema.StringAttribute{
							Computed:    true,
							Description: "The certificate's organization.",
						},
						"organizational_unit": schema.StringAttribute{
							Computed:    true,
							Description: "The certificate's organizational unit.",
						},
						"subject_alternative_name": schema.StringAttribute{
							Computed:    true,
							Description: "The certificate's subject alternative name (or SAN).",
						},
					},
				},
			},
			"api_key_auth": schema.BoolAttribute{
				Description: "If true, Temporal Cloud will use API key authentication for this namespace. If false, mutual TLS (mTLS) authentication will be used.",
				Optional:    true,
			},
			"codec_server": schema.SingleNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "A codec server is used by the Temporal Cloud UI to decode payloads for all users interacting with this namespace, even if the workflow history itself is encrypted.",
				Attributes: map[string]schema.Attribute{
					"endpoint": schema.StringAttribute{
						Computed:    true,
						Description: "The endpoint of the codec server.",
					},
					"pass_access_token": schema.BoolAttribute{
						Computed:    true,
						Description: "If true, Temporal Cloud will pass the access token to the codec server upon each request.",
					},
					"include_cross_origin_credentials": schema.BoolAttribute{
						Computed:    true,
						Description: "If true, Temporal Cloud will include cross-origin credentials in requests to the codec server.",
					},
				},
			},
			"endpoints": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "The endpoints for the namespace.",
				Attributes: map[string]schema.Attribute{
					"web_address": schema.StringAttribute{
						Description: "The web UI address.",
						Computed:    true,
					},
					"grpc_address": schema.StringAttribute{
						Computed:    true,
						Description: "The gRPC hostport address that the temporal workers, clients and tctl connect to.",
					},
				},
			},
			"private_connectivities": schema.ListNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The private connectivities for the namespace, if any.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"region": schema.StringAttribute{
							Computed:    true,
							Description: "The id of the region where the private connectivity applies.",
						},
						"aws_private_link_info": schema.SingleNestedAttribute{
							Computed:    true,
							Optional:    true,
							Description: "The AWS PrivateLink info. This will only be set for namespaces whose cloud provider is AWS.",
							Attributes: map[string]schema.Attribute{
								"allowed_principal_arns": schema.ListAttribute{
									Computed:    true,
									ElementType: types.StringType,
									Description: "The list of principal arns that are allowed to access the namespace on the private link.",
								},
								"vpc_endpoint_service_names": schema.ListAttribute{
									Computed:    true,
									ElementType: types.StringType,
									Description: "The list of vpc endpoint service names that are associated with the namespace.",
								},
							},
						},
					},
				},
			},
			"custom_search_attributes": schema.MapAttribute{
				Computed:    true,
				Optional:    true,
				ElementType: types.StringType,
				Description: "The custom search attributes to use for the namespace.",
			},
			"limits": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "The limits set on the namespace currently.",
				Attributes: map[string]schema.Attribute{
					"actions_per_second_limit": schema.Int64Attribute{
						Computed:    true,
						Description: "The number of actions per second (APS) that is currently allowed for the namespace. The namespace may be throttled if its APS exceeds the limit.",
					},
				},
			},
			"created_time": schema.StringAttribute{
				Computed:    true,
				Description: "The date and time when the namespace was created.",
			},
			"last_modified_time": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				Description: "The date and time when the namespace was last modified. Will not be set if the namespace has never been modified.",
			},
		},
	}
}

func (d *namespaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var input namespaceDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(input.ID.ValueString()) == 0 {
		resp.Diagnostics.AddError("invalid namespace id", "namespace id is required")
		return
	}

	namespaceResp, err := d.client.CloudService().GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: input.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch namespaces", err.Error())
		return
	}

	namespaceModel, diags := namespaceToNamespaceDataModel(ctx, namespaceResp.GetNamespace())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, namespaceModel)
	resp.Diagnostics.Append(diags...)
}
