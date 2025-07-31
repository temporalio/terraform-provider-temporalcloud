// The MIT License
//
// Copyright (c) 2023 Temporal Technologies Inc.  All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	namespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
)

const (
	defaultCreateTimeout time.Duration = 5 * time.Minute
	defaultDeleteTimeout time.Duration = 5 * time.Minute
)

type (
	namespaceResource struct {
		// client cloudservicev1.CloudServiceClient
		client *client.Client
	}

	namespaceResourceModel struct {
		ID                  types.String                           `tfsdk:"id"`
		Name                types.String                           `tfsdk:"name"`
		Regions             internaltypes.UnorderedStringListValue `tfsdk:"regions"`
		AcceptedClientCA    internaltypes.EncodedCAValue           `tfsdk:"accepted_client_ca"`
		RetentionDays       types.Int64                            `tfsdk:"retention_days"`
		CertificateFilters  types.List                             `tfsdk:"certificate_filters"`
		ApiKeyAuth          types.Bool                             `tfsdk:"api_key_auth"`
		CodecServer         types.Object                           `tfsdk:"codec_server"`
		Endpoints           types.Object                           `tfsdk:"endpoints"`
		ConnectivityRuleIds internaltypes.UnorderedStringListValue `tfsdk:"connectivity_rule_ids"`

		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}

	namespaceCertificateFilterModel struct {
		CommonName             types.String `tfsdk:"common_name"`
		Organization           types.String `tfsdk:"organization"`
		OrganizationalUnit     types.String `tfsdk:"organizational_unit"`
		SubjectAlternativeName types.String `tfsdk:"subject_alternative_name"`
	}

	codecServerModel struct {
		Endpoint                      types.String `tfsdk:"endpoint"`
		PassAccessToken               types.Bool   `tfsdk:"pass_access_token"`
		IncludeCrossOriginCredentials types.Bool   `tfsdk:"include_cross_origin_credentials"`
	}

	endpointsModel struct {
		WebAddress      types.String `tfsdk:"web_address"`
		GrpcAddress     types.String `tfsdk:"grpc_address"`
		MtlsGrpcAddress types.String `tfsdk:"mtls_grpc_address"`
	}
)

var (
	_ resource.Resource                = (*namespaceResource)(nil)
	_ resource.ResourceWithConfigure   = (*namespaceResource)(nil)
	_ resource.ResourceWithImportState = (*namespaceResource)(nil)

	namespaceCertificateFilterAttrs = map[string]attr.Type{
		"common_name":              types.StringType,
		"organization":             types.StringType,
		"organizational_unit":      types.StringType,
		"subject_alternative_name": types.StringType,
	}

	codecServerAttrs = map[string]attr.Type{
		"endpoint":                         types.StringType,
		"pass_access_token":                types.BoolType,
		"include_cross_origin_credentials": types.BoolType,
	}

	endpointsAttrs = map[string]attr.Type{
		"web_address":       types.StringType,
		"grpc_address":      types.StringType,
		"mtls_grpc_address": types.StringType,
	}
)

func NewNamespaceResource() resource.Resource {
	return &namespaceResource{}
}

func (r *namespaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Metadata returns the resource type name.
func (r *namespaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace"
}

// Schema defines the schema for the resource.
func (r *namespaceResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provisions a Temporal Cloud namespace. \n\nRegions available in Temporal Cloud: https://docs.temporal.io/cloud/regions. \n\nNote that regions are prefixed with the cloud provider (aws-us-east-1, not us-east-1)",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the namespace.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				Description: "The unique identifier of the namespace across all Temporal Cloud tenants.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"regions": schema.ListAttribute{
				Description: "The list of regions where this namespace is available. Must be one or two regions. See https://docs.temporal.io/cloud/regions for a list of available regions and HA options. Note that regions are prefixed with the cloud provider (aws-us-east-1, not us-east-1). If two regions are specified, the namespace will be replicated across them in a high availability (HA) configuration. Same-region, multi-region, and multi-cloud HA namespaces are supported. Please note that changing, adding, or removing regions for an existing namespace is not currently supported and the provider will throw an error. For HA namespaces the provider will ignore order changes on regions, which can happen if the namespace fails over.",
				ElementType: types.StringType,
				Required:    true,
				CustomType: internaltypes.UnorderedStringListType{
					ListType: basetypes.ListType{ElemType: basetypes.StringType{}},
				},
			},
			"accepted_client_ca": schema.StringAttribute{
				CustomType:  internaltypes.EncodedCAType{},
				Description: "The Base64-encoded CA cert in PEM format that clients use when authenticating with Temporal Cloud. This is a required field when a Namespace uses mTLS authentication.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"retention_days": schema.Int64Attribute{
				Description: "The number of days to retain workflow history. Any changes to the retention period will be applied to all new running workflows.",
				Required:    true,
			},
			"certificate_filters": schema.ListNestedAttribute{
				Description: "A list of filters to apply to client certificates when initiating a connection Temporal Cloud. If present, connections will only be allowed from client certificates whose distinguished name properties match at least one of the filters. Empty lists are not allowed, omit the attribute instead.",
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"common_name": schema.StringAttribute{
							Description: "The certificate's common name.",
							Optional:    true,
						},
						"organization": schema.StringAttribute{
							Description: "The certificate's organization.",
							Optional:    true,
						},
						"organizational_unit": schema.StringAttribute{
							Description: "The certificate's organizational unit.",
							Optional:    true,
						},
						"subject_alternative_name": schema.StringAttribute{
							Description: "The certificate's subject alternative name (or SAN).",
							Optional:    true,
						},
					},
				},
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"api_key_auth": schema.BoolAttribute{
				Description: "If true, Temporal Cloud will enable API key authentication for this namespace.",
				Optional:    true,
				Computed:    true,
			},
			"codec_server": schema.SingleNestedAttribute{
				Description: "A codec server is used by the Temporal Cloud UI to decode payloads for all users interacting with this namespace, even if the workflow history itself is encrypted.",
				Attributes: map[string]schema.Attribute{
					"endpoint": schema.StringAttribute{
						Description: "The endpoint of the codec server. Must begin with \"https\".",
						Required:    true,
					},
					"pass_access_token": schema.BoolAttribute{
						Description: "If true, Temporal Cloud will pass the access token to the codec server upon each request.",
						Computed:    true,
						Default:     booldefault.StaticBool(false),
						Optional:    true,
					},
					"include_cross_origin_credentials": schema.BoolAttribute{
						Description: "If true, Temporal Cloud will include cross-origin credentials in requests to the codec server.",
						Computed:    true,
						Default:     booldefault.StaticBool(false),
						Optional:    true,
					},
				},
				Optional: true,
			},
			"endpoints": schema.SingleNestedAttribute{
				Description: "The endpoints for the namespace.",
				Attributes: map[string]schema.Attribute{
					"grpc_address": schema.StringAttribute{
						Description: "The gRPC address for API key client connections (may be empty if API keys are disabled).",
						Computed:    true,
					},
					"mtls_grpc_address": schema.StringAttribute{
						Description: "The gRPC address for mTLS client connections (may be empty if mTLS is disabled).",
						Computed:    true,
					},
					"web_address": schema.StringAttribute{
						Description: "The address in the Temporal Cloud Web UI for the namespace",
						Computed:    true,
					},
				},
				Computed: true,
			},
			"connectivity_rule_ids": schema.ListAttribute{
				Description: "The IDs of the connectivity rules for this namespace.",
				Optional:    true,
				ElementType: types.StringType,
				CustomType: internaltypes.UnorderedStringListType{
					ListType: basetypes.ListType{ElemType: basetypes.StringType{}},
				},
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Delete: true,
			}),
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *namespaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan namespaceResourceModel
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

	regions, d := getRegionsFromModel(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	certFilters, d := getCertFiltersFromModel(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	var codecServer *namespacev1.CodecServerSpec
	if !plan.CodecServer.IsNull() {
		var d diag.Diagnostics
		codecServer, d = getCodecServerFromModel(ctx, &plan)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	connectivityRuleIds, d := getConnectivityRuleIdsFromModel(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	spec := &namespacev1.NamespaceSpec{
		Name:                plan.Name.ValueString(),
		Regions:             regions,
		RetentionDays:       int32(plan.RetentionDays.ValueInt64()),
		CodecServer:         codecServer,
		ConnectivityRuleIds: connectivityRuleIds,
	}

	if !plan.ApiKeyAuth.ValueBool() && plan.AcceptedClientCA.IsNull() {
		resp.Diagnostics.AddError("Namespace not configured with authentication", "accepted_client_ca or api_key_auth must be set")
		return
	}

	if plan.ApiKeyAuth.ValueBool() {
		spec.ApiKeyAuth = &namespacev1.ApiKeyAuthSpec{Enabled: true}
	}

	if !plan.AcceptedClientCA.IsNull() {
		mtls := &namespacev1.MtlsAuthSpec{}
		if plan.AcceptedClientCA.ValueString() != "" {
			certs, err := base64.StdEncoding.DecodeString(plan.AcceptedClientCA.ValueString())
			if err != nil {
				resp.Diagnostics.AddError("Invalid (base64 encoded) accepted_client_ca", err.Error())
				return
			}
			mtls.Enabled = true
			mtls.AcceptedClientCa = certs
			mtls.CertificateFilters = certFilters
		}

		spec.MtlsAuth = mtls
	}

	svcResp, err := r.client.CloudService().CreateNamespace(ctx, &cloudservicev1.CreateNamespaceRequest{
		Spec:             spec,
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create namespace", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to create namespace", err.Error())
		return
	}

	ns, err := r.client.CloudService().GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: svcResp.Namespace,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace after creation", err.Error())
		return
	}

	resp.Diagnostics.Append(updateModelFromSpec(ctx, &plan, ns.Namespace)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *namespaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state namespaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	model, err := r.client.CloudService().GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Namespace Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to get namespace", err.Error())
		return
	}

	resp.Diagnostics.Append(updateModelFromSpec(ctx, &state, model.Namespace)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func areRegionsEqual(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	// Create copies to avoid modifying the original slices
	sortedS1 := make([]string, len(s1))
	copy(sortedS1, s1)
	sortedS2 := make([]string, len(s2))
	copy(sortedS2, s2)

	// Sort both slices
	sort.Strings(sortedS1)
	sort.Strings(sortedS2)

	return slices.Compare(sortedS1, sortedS2) == 0
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *namespaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan namespaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	regions, d := getRegionsFromModel(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	certFilters, d := getCertFiltersFromModel(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	connectivityRuleIds, d := getConnectivityRuleIdsFromModel(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentNs, err := r.client.CloudService().GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current namespace status", err.Error())
		return
	}

	var codecServer *namespacev1.CodecServerSpec
	if !plan.CodecServer.IsNull() {
		var d diag.Diagnostics
		codecServer, d = getCodecServerFromModel(ctx, &plan)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	spec := &namespacev1.NamespaceSpec{
		Name:                plan.Name.ValueString(),
		Regions:             regions,
		RetentionDays:       int32(plan.RetentionDays.ValueInt64()),
		CodecServer:         codecServer,
		SearchAttributes:    currentNs.GetNamespace().GetSpec().GetSearchAttributes(),
		ConnectivityRuleIds: connectivityRuleIds,
	}

	if !plan.ApiKeyAuth.ValueBool() && plan.AcceptedClientCA.IsNull() {
		resp.Diagnostics.AddError("Namespace not configured with authentication", "accepted_client_ca or api_key_auth must be set")
		return
	}

	if plan.ApiKeyAuth.ValueBool() {
		spec.ApiKeyAuth = &namespacev1.ApiKeyAuthSpec{Enabled: true}
	}

	if !plan.AcceptedClientCA.IsNull() {
		mtls := &namespacev1.MtlsAuthSpec{}

		if plan.AcceptedClientCA.ValueString() != "" {
			certs, err := base64.StdEncoding.DecodeString(plan.AcceptedClientCA.ValueString())
			if err != nil {
				resp.Diagnostics.AddError("Invalid (base64 encoded) accepted_client_ca", err.Error())
				return
			}
			mtls.Enabled = true
			mtls.AcceptedClientCa = certs
			mtls.CertificateFilters = certFilters
		}

		spec.MtlsAuth = mtls
	}

	if !areRegionsEqual(currentNs.GetNamespace().GetSpec().GetRegions(), spec.Regions) {
		resp.Diagnostics.AddError("Namespace regions cannot be changed", "Changing the regions of a namespace is not supported currently via terraform.")
		return
	}

	svcResp, err := r.client.CloudService().UpdateNamespace(ctx, &cloudservicev1.UpdateNamespaceRequest{
		Namespace:        plan.ID.ValueString(),
		Spec:             spec,
		ResourceVersion:  currentNs.GetNamespace().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update namespace", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to update namespace", err.Error())
		return
	}

	ns, err := r.client.CloudService().GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace after update", err.Error())
		return
	}

	resp.Diagnostics.Append(updateModelFromSpec(ctx, &plan, ns.Namespace)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *namespaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state namespaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentNs, err := r.client.CloudService().GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Namespace Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to get current namespace status", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()
	svcResp, err := r.client.CloudService().DeleteNamespace(ctx, &cloudservicev1.DeleteNamespaceRequest{
		Namespace:        state.ID.ValueString(),
		ResourceVersion:  currentNs.GetNamespace().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Namespace Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to delete namespace", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to delete namespace", err.Error())
	}
}

func (r *namespaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func getRegionsFromModel(ctx context.Context, plan *namespaceResourceModel) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	regions := make([]types.String, 0, len(plan.Regions.Elements()))
	diags.Append(plan.Regions.ElementsAs(ctx, &regions, false)...)
	if diags.HasError() {
		return nil, diags
	}

	requestRegions := make([]string, len(regions))
	for i, region := range regions {
		requestRegions[i] = region.ValueString()
	}

	return requestRegions, diags
}

func getConnectivityRuleIdsFromModel(ctx context.Context, plan *namespaceResourceModel) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics

	connectivityRuleIds := make([]types.String, 0, len(plan.ConnectivityRuleIds.Elements()))
	diags.Append(plan.ConnectivityRuleIds.ElementsAs(ctx, &connectivityRuleIds, false)...)
	if diags.HasError() {
		return nil, diags
	}

	if len(connectivityRuleIds) == 0 {
		return nil, diags
	}

	requestConnectivityRuleIds := make([]string, len(connectivityRuleIds))
	for i, connectivityRuleId := range connectivityRuleIds {
		requestConnectivityRuleIds[i] = connectivityRuleId.ValueString()
	}
	return requestConnectivityRuleIds, diags
}

func updateModelFromSpec(ctx context.Context, state *namespaceResourceModel, ns *namespacev1.Namespace) diag.Diagnostics {
	var diags diag.Diagnostics

	state.ID = types.StringValue(ns.GetNamespace())
	state.Name = types.StringValue(ns.GetSpec().GetName())
	planRegions, listDiags := types.ListValueFrom(ctx, types.StringType, ns.GetSpec().GetRegions())
	diags.Append(listDiags...)
	if diags.HasError() {
		return diags
	}
	planRegionsUnordered := internaltypes.UnorderedStringListValue{
		ListValue: planRegions,
	}

	certificateFilter := types.ListNull(types.ObjectType{AttrTypes: namespaceCertificateFilterAttrs})
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
			diags.Append(diag...)
			if diags.HasError() {
				return diags
			}
			certificateFilterObjects[i] = obj
		}

		filters, diag := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: namespaceCertificateFilterAttrs}, certificateFilterObjects)
		diags.Append(diag...)
		if diags.HasError() {
			return diags
		}

		certificateFilter = filters
	}

	if len(ns.GetSpec().GetMtlsAuth().GetAcceptedClientCa()) > 0 {
		state.AcceptedClientCA = internaltypes.EncodedCA(
			base64.StdEncoding.EncodeToString(ns.GetSpec().GetMtlsAuth().GetAcceptedClientCa()),
		)
	}

	if ns.GetSpec().GetApiKeyAuth() != nil {
		state.ApiKeyAuth = types.BoolValue(ns.GetSpec().GetApiKeyAuth().GetEnabled())
	}

	var codecServerState basetypes.ObjectValue
	// The API always returns a non-empty CodecServerSpec, even if it wasn't specified on object creation. We explicitly
	// map an endpoint whose value is the empty string to `null`, since an empty endpoint implies that the codec server
	// was not set via config.
	if ns.GetSpec().GetCodecServer().GetEndpoint() != "" {
		codecServer := &codecServerModel{
			Endpoint:                      stringOrNull(ns.GetSpec().GetCodecServer().GetEndpoint()),
			PassAccessToken:               types.BoolValue(ns.GetSpec().GetCodecServer().GetPassAccessToken()),
			IncludeCrossOriginCredentials: types.BoolValue(ns.GetSpec().GetCodecServer().GetIncludeCrossOriginCredentials()),
		}

		state, objectDiags := types.ObjectValueFrom(ctx, codecServerAttrs, codecServer)
		diags.Append(objectDiags...)
		if diags.HasError() {
			return diags
		}

		codecServerState = state
	} else {
		codecServerState = types.ObjectNull(codecServerAttrs)
	}
	state.CodecServer = codecServerState

	endpoints := &endpointsModel{
		GrpcAddress:     stringOrNull(ns.GetEndpoints().GetGrpcAddress()),
		WebAddress:      stringOrNull(ns.GetEndpoints().GetWebAddress()),
		MtlsGrpcAddress: stringOrNull(ns.GetEndpoints().GetMtlsGrpcAddress()),
	}
	endpointsState, objectDiags := types.ObjectValueFrom(ctx, endpointsAttrs, endpoints)
	diags.Append(objectDiags...)
	if diags.HasError() {
		return diags
	}

	// Handle connectivity rule IDs - preserve the intent from the plan

	connectivityRuleIdsState := internaltypes.UnorderedStringListValue{
		ListValue: types.ListNull(types.StringType),
	}
	connectivityRuleIds := ns.GetSpec().GetConnectivityRuleIds()
	if len(connectivityRuleIds) > 0 {
		// Use API response values
		planConnectivityRuleIds, listDiags := types.ListValueFrom(ctx, types.StringType, connectivityRuleIds)
		diags.Append(listDiags...)
		if diags.HasError() {
			return diags
		}
		connectivityRuleIdsState = internaltypes.UnorderedStringListValue{
			ListValue: planConnectivityRuleIds,
		}

	}
	state.ConnectivityRuleIds = connectivityRuleIdsState
	state.Endpoints = endpointsState
	state.Regions = planRegionsUnordered
	state.CertificateFilters = certificateFilter
	state.RetentionDays = types.Int64Value(int64(ns.GetSpec().GetRetentionDays()))

	return diags
}

func getCertFiltersFromModel(ctx context.Context, model *namespaceResourceModel) ([]*namespacev1.CertificateFilterSpec, diag.Diagnostics) {
	var diags diag.Diagnostics

	elements := make([]types.Object, 0, len(model.CertificateFilters.Elements()))
	diags.Append(model.CertificateFilters.ElementsAs(ctx, &elements, false)...)
	if diags.HasError() {
		return nil, diags
	}

	if len(elements) == 0 {
		return nil, diags
	}

	certificateFilters := make([]*namespacev1.CertificateFilterSpec, len(elements))
	for i, filter := range elements {
		var model namespaceCertificateFilterModel
		diags.Append(filter.As(ctx, &model, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}

		certificateFilters[i] = &namespacev1.CertificateFilterSpec{
			CommonName:             model.CommonName.ValueString(),
			Organization:           model.Organization.ValueString(),
			OrganizationalUnit:     model.OrganizationalUnit.ValueString(),
			SubjectAlternativeName: model.SubjectAlternativeName.ValueString(),
		}
	}

	return certificateFilters, diags
}

func getCodecServerFromModel(ctx context.Context, model *namespaceResourceModel) (*namespacev1.CodecServerSpec, diag.Diagnostics) {
	var diags diag.Diagnostics
	var codecServer codecServerModel
	diags.Append(model.CodecServer.As(ctx, &codecServer, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}
	return &namespacev1.CodecServerSpec{
		Endpoint:                      codecServer.Endpoint.ValueString(),
		PassAccessToken:               codecServer.PassAccessToken.ValueBool(),
		IncludeCrossOriginCredentials: codecServer.IncludeCrossOriginCredentials.ValueBool(),
	}, diags
}

func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
