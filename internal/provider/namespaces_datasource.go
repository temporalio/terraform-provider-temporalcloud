package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"

	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"
	namespacev1 "go.temporal.io/api/cloud/namespace/v1"
)

var (
	_ datasource.DataSource              = &namespacesDataSource{}
	_ datasource.DataSourceWithConfigure = &namespacesDataSource{}
)

func NewNamespacesDataSource() datasource.DataSource {
	return &namespacesDataSource{}
}

type (
	namespacesDataSource struct {
		client *client.Client
	}

	namespacesDataModel struct {
		ID         types.String         `tfsdk:"id"`
		Namespaces []namespaceDataModel `tfsdk:"namespaces"`
	}

	namespaceDataModel struct {
		ID                     types.String `tfsdk:"id"`
		Name                   types.String `tfsdk:"name"`
		State                  types.String `tfsdk:"state"`
		ActiveRegion           types.String `tfsdk:"active_region"`
		Regions                types.List   `tfsdk:"regions"`
		AcceptedClientCA       types.String `tfsdk:"accepted_client_ca"`
		RetentionDays          types.Int64  `tfsdk:"retention_days"`
		CertificateFilters     types.List   `tfsdk:"certificate_filters"`
		ApiKeyAuth             types.Bool   `tfsdk:"api_key_auth"`
		CodecServer            types.Object `tfsdk:"codec_server"`
		Endpoints              types.Object `tfsdk:"endpoints"`
		PrivateConnectivities  types.List   `tfsdk:"private_connectivities"`
		CustomSearchAttributes types.Map    `tfsdk:"custom_search_attributes"`
		Limits                 types.Object `tfsdk:"limits"`
		CreatedTime            types.String `tfsdk:"created_time"`
		LastModifiedTime       types.String `tfsdk:"last_modified_time"`
	}

	endpointsDataModel struct {
		WebAddress  types.String `tfsdk:"web_address"`
		GrpcAddress types.String `tfsdk:"grpc_address"`
	}

	privateConnectivityDataModel struct {
		Region             types.String `tfsdk:"region"`
		AwsPrivateLinkInfo types.Object `tfsdk:"aws_private_link_info"`
	}

	awsPrivateLinkInfoDataModel struct {
		AllowedPrincipalArns    types.List `tfsdk:"allowed_principal_arns"`
		VpcEndpointServiceNames types.List `tfsdk:"vpc_endpoint_service_names"`
	}

	limitsDataModel struct {
		ActionsPerSecondLimit types.Int64 `tfsdk:"actions_per_second_limit"`
	}
)

var (
	endpointDataModelAttrs = map[string]attr.Type{
		"web_address":  types.StringType,
		"grpc_address": types.StringType,
	}

	privateConnectivityDataModelAttrs = map[string]attr.Type{
		"region":                types.StringType,
		"aws_private_link_info": types.ObjectType{AttrTypes: awsPrivateLinkAttrs},
	}

	awsPrivateLinkAttrs = map[string]attr.Type{
		"allowed_principal_arns":     types.ListType{ElemType: types.StringType},
		"vpc_endpoint_service_names": types.ListType{ElemType: types.StringType},
	}

	limitsAttrs = map[string]attr.Type{
		"actions_per_second_limit": types.Int64Type,
	}
)

func (d *namespacesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *namespacesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespaces"
}

func (d *namespacesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"namespaces": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
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
				},
			},
		},
	}
}

func (d *namespacesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state namespacesDataModel

	var namespaces []*namespacev1.Namespace
	pageToken := ""
	for {
		r, err := d.client.CloudService().GetNamespaces(ctx, &cloudservicev1.GetNamespacesRequest{PageToken: pageToken})
		if err != nil {
			resp.Diagnostics.AddError("Unable to fetch namespaces", err.Error())
			return
		}

		namespaces = append(namespaces, r.GetNamespaces()...)

		if r.GetNextPageToken() == "" {
			break
		}

		pageToken = r.GetNextPageToken()
	}

	for _, ns := range namespaces {
		namespaceModel := namespaceDataModel{
			ID:               types.StringValue(ns.Namespace),
			Name:             types.StringValue(ns.GetSpec().GetName()),
			State:            types.StringValue(ns.State),
			ActiveRegion:     types.StringValue(ns.ActiveRegion),
			AcceptedClientCA: types.StringValue(ns.GetSpec().GetMtlsAuth().GetAcceptedClientCa()),
			ApiKeyAuth:       types.BoolValue(ns.GetSpec().GetApiKeyAuth().GetEnabled()),
			RetentionDays:    types.Int64Value(int64(ns.GetSpec().GetRetentionDays())),
			CreatedTime:      types.StringValue(ns.GetCreatedTime().AsTime().Format(time.RFC3339)),
		}
		if ns.GetLastModifiedTime().String() != "" {
			namespaceModel.LastModifiedTime = types.StringValue(ns.GetLastModifiedTime().AsTime().Format(time.RFC3339))
		} else {
			namespaceModel.LastModifiedTime = types.StringNull()
		}

		regions, listDiags := types.ListValueFrom(ctx, types.StringType, ns.GetSpec().GetRegions())
		resp.Diagnostics.Append(listDiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		namespaceModel.Regions = regions

		certificateFilters := types.ListNull(types.ObjectType{AttrTypes: namespaceCertificateFilterAttrs})
		if len(ns.GetSpec().GetMtlsAuth().GetCertificateFilters()) > 0 {
			certificateFilterObjects := make([]types.Object, len(ns.GetSpec().GetMtlsAuth().GetCertificateFilters()))
			for i, certFilter := range ns.GetSpec().GetMtlsAuth().GetCertificateFilters() {
				model := namespaceCertificateFilterModel{
					CommonName:             stringOrNull(certFilter.GetCommonName()),
					Organization:           stringOrNull(certFilter.GetOrganization()),
					OrganizationalUnit:     stringOrNull(certFilter.GetOrganizationalUnit()),
					SubjectAlternativeName: stringOrNull(certFilter.GetSubjectAlternativeName()),
				}
				obj, diag := types.ObjectValueFrom(ctx, namespaceCertificateFilterAttrs, model)
				resp.Diagnostics.Append(diag...)
				if resp.Diagnostics.HasError() {
					return
				}
				certificateFilterObjects[i] = obj
			}
			filters, diag := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: namespaceCertificateFilterAttrs}, certificateFilterObjects)
			resp.Diagnostics.Append(diag...)
			if resp.Diagnostics.HasError() {
				return
			}

			certificateFilters = filters
		}
		namespaceModel.CertificateFilters = certificateFilters

		var codecServer basetypes.ObjectValue
		if ns.GetSpec().GetCodecServer().GetEndpoint() != "" {
			csModel := &codecServerModel{
				Endpoint:                      stringOrNull(ns.GetSpec().GetCodecServer().GetEndpoint()),
				PassAccessToken:               types.BoolValue(ns.GetSpec().GetCodecServer().GetPassAccessToken()),
				IncludeCrossOriginCredentials: types.BoolValue(ns.GetSpec().GetCodecServer().GetIncludeCrossOriginCredentials()),
			}
			s, objectDiags := types.ObjectValueFrom(ctx, codecServerAttrs, csModel)
			resp.Diagnostics.Append(objectDiags...)
			codecServer = s
		} else {
			codecServer = types.ObjectNull(codecServerAttrs)
		}

		namespaceModel.CodecServer = codecServer

		endpointModel := &endpointsDataModel{
			GrpcAddress: types.StringValue(ns.GetEndpoints().GetGrpcAddress()),
			WebAddress:  types.StringValue(ns.GetEndpoints().GetWebAddress()),
		}
		endpointState, endpointDiags := types.ObjectValueFrom(ctx, endpointDataModelAttrs, endpointModel)
		resp.Diagnostics.Append(endpointDiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		namespaceModel.Endpoints = endpointState

		privateConnectivites := types.ListNull(types.ObjectType{AttrTypes: privateConnectivityDataModelAttrs})
		if len(ns.GetPrivateConnectivities()) > 0 {
			privateConnectivityObjects := make([]types.Object, len(ns.GetPrivateConnectivities()))
			for i, privateConn := range ns.GetPrivateConnectivities() {
				var awsPrivLinkModel awsPrivateLinkInfoDataModel
				principals, listDiags := types.ListValueFrom(ctx, types.StringType, privateConn.GetAwsPrivateLink().GetAllowedPrincipalArns())
				resp.Diagnostics.Append(listDiags...)
				if resp.Diagnostics.HasError() {
					return
				}
				awsPrivLinkModel.AllowedPrincipalArns = principals

				serviceNames, listDiags := types.ListValueFrom(ctx, types.StringType, privateConn.GetAwsPrivateLink().GetAllowedPrincipalArns())
				resp.Diagnostics.Append(listDiags...)
				if resp.Diagnostics.HasError() {
					return
				}
				awsPrivLinkModel.VpcEndpointServiceNames = serviceNames
				privLinkObj, diag := types.ObjectValueFrom(ctx, awsPrivateLinkAttrs, awsPrivLinkModel)
				resp.Diagnostics.Append(diag...)
				if resp.Diagnostics.HasError() {
					return
				}
				model := privateConnectivityDataModel{
					Region:             types.StringValue(privateConn.GetRegion()),
					AwsPrivateLinkInfo: privLinkObj,
				}
				obj, diag := types.ObjectValueFrom(ctx, privateConnectivityDataModelAttrs, model)
				resp.Diagnostics.Append(diag...)
				if resp.Diagnostics.HasError() {
					return
				}
				privateConnectivityObjects[i] = obj
			}
			privateConns, diag := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: privateConnectivityDataModelAttrs}, privateConnectivityObjects)
			resp.Diagnostics.Append(diag...)
			if resp.Diagnostics.HasError() {
				return
			}
			privateConnectivites = privateConns
		}
		namespaceModel.PrivateConnectivities = privateConnectivites

		searchAttributes := types.MapNull(types.StringType)
		if len(ns.GetSpec().GetCustomSearchAttributes()) > 0 {
			sa, diag := types.MapValueFrom(ctx, types.StringType, ns.GetSpec().GetCustomSearchAttributes())
			resp.Diagnostics.Append(diag...)
			if resp.Diagnostics.HasError() {
				return
			}
			searchAttributes = sa
		}
		namespaceModel.CustomSearchAttributes = searchAttributes

		limitModel := &limitsDataModel{
			ActionsPerSecondLimit: types.Int64Value(int64(ns.GetLimits().GetActionsPerSecondLimit())),
		}
		limits, diag := types.ObjectValueFrom(ctx, limitsAttrs, limitModel)
		resp.Diagnostics.Append(diag...)
		if resp.Diagnostics.HasError() {
			return
		}
		namespaceModel.Limits = limits

		state.Namespaces = append(state.Namespaces, namespaceModel)
	}

	state.ID = types.StringValue("terraform")
	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}
