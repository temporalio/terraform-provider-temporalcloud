package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"
	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/validation"
)

type namespaceAccessModel struct {
	NamespaceID types.String                             `tfsdk:"namespace_id"`
	Permission  internaltypes.CaseInsensitiveStringValue `tfsdk:"permission"`
}

type projectAccessModel struct {
	ProjectID types.String                             `tfsdk:"project_id"`
	Role      internaltypes.CaseInsensitiveStringValue `tfsdk:"role"`
}

const accountAccessCustomRolesDescription = "The set of custom role IDs assigned within account_access in addition to the built-in account_access role. Empty sets are not allowed, omit the attribute instead."

const projectAccessesDescription = "The set of project accesses. Empty sets are not allowed, omit the attribute instead. Roles inherited from an account_access role are not included in this set and cannot be managed."

const projectAccessRoleDescription = "The role to assign. Must be one of `admin`, `write`, `read`, `list`, `contribute`, or `member` (case-insensitive)."

var namespaceAccessAttrs = map[string]attr.Type{
	"namespace_id": types.StringType,
	"permission":   internaltypes.CaseInsensitiveStringType{},
}

var projectAccessAttrs = map[string]attr.Type{
	"project_id": types.StringType,
	"role":       internaltypes.CaseInsensitiveStringType{},
}

// projectAccessesSchema returns the project_accesses attribute, shared by the resources that build
// their access schema inline and the ones using addAccessSchemaAttrs.
func projectAccessesSchema(descriptionSuffix string, extraValidators ...validator.Set) schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Description: projectAccessesDescription + descriptionSuffix,
		Optional:    true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"project_id": schema.StringAttribute{
					Description: "The project to assign a role to.",
					Required:    true,
				},
				"role": schema.StringAttribute{
					CustomType:  internaltypes.CaseInsensitiveStringType{},
					Description: projectAccessRoleDescription,
					Required:    true,
					Validators: []validator.String{
						stringvalidator.OneOfCaseInsensitive(enums.AllowedProjectAccessRoles()...),
					},
				},
			},
		},
		Validators: append([]validator.Set{
			setvalidator.SizeAtLeast(1),
			validation.SetNestedAttributeMustBeUnique("project_id"),
		}, extraValidators...),
	}
}

// projectAccessesDataSourceSchema is the read-only equivalent of projectAccessesSchema. identityType
// names the identity in the description, as namespace_accesses does ("user", "service account").
func projectAccessesDataSourceSchema(identityType string) dsschema.SetNestedAttribute {
	return dsschema.SetNestedAttribute{
		Description: fmt.Sprintf("The set of project roles for this %s, including each project and its role.", identityType),
		Computed:    true,
		NestedObject: dsschema.NestedAttributeObject{
			Attributes: map[string]dsschema.Attribute{
				"project_id": dsschema.StringAttribute{
					Description: "The project to assign a role to.",
					Computed:    true,
				},
				"role": dsschema.StringAttribute{
					CustomType:  internaltypes.CaseInsensitiveStringType{},
					Description: projectAccessRoleDescription,
					Computed:    true,
				},
			},
		},
	}
}

func addAccessSchemaAttrs(s *schema.Schema, accountDescriptionSuffix string) {
	s.Attributes["account_access"] = schema.StringAttribute{
		CustomType:  internaltypes.CaseInsensitiveStringType{},
		Description: "The role on the account. Must be one of `owner`, `admin`, `developer`, `read`, `financeadmin`, or `none` (case-insensitive). `owner` is only valid for import and cannot be created, updated or deleted without Temporal support." + accountDescriptionSuffix,
		Required:    true,
		Validators: []validator.String{
			stringvalidator.OneOfCaseInsensitive(enums.AllowedAccountAccessRolesForUsersAndGroups()...),
		},
	}

	s.Attributes["account_access_custom_roles"] = schema.SetAttribute{
		Description: accountAccessCustomRolesDescription,
		Optional:    true,
		ElementType: types.StringType,
		Validators: []validator.Set{
			setvalidator.SizeAtLeast(1),
		},
	}

	s.Attributes["namespace_accesses"] = schema.SetNestedAttribute{
		Description: "The set of namespace accesses. Empty sets are not allowed, omit the attribute instead. Users with account_access roles of owner or admin cannot be assigned explicit permissions to namespaces. They implicitly receive access to all Namespaces.",
		Optional:    true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"namespace_id": schema.StringAttribute{
					Description: "The namespace to assign permissions to.",
					Required:    true,
				},
				"permission": schema.StringAttribute{
					CustomType:  internaltypes.CaseInsensitiveStringType{},
					Description: "The permission to assign. Must be one of admin, write, or read (case-insensitive)",
					Required:    true,
					Validators: []validator.String{
						stringvalidator.OneOfCaseInsensitive("admin", "write", "read"),
					},
				},
			},
		},
		Validators: []validator.Set{
			setvalidator.SizeAtLeast(1),
			validation.SetNestedAttributeMustBeUnique("namespace_id"),
			validation.SetMustBeEmptyWhen(
				path.Root("account_access"),
				[]string{"owner", "admin"},
			),
		},
	}

	s.Attributes["project_accesses"] = projectAccessesSchema("")
}

func getProjectAccessesFromSet(ctx context.Context, set types.Set) (map[string]*identityv1.ProjectAccess, diag.Diagnostics) {
	var diags diag.Diagnostics

	elements := make([]types.Object, 0, len(set.Elements()))
	diags.Append(set.ElementsAs(ctx, &elements, false)...)
	if diags.HasError() {
		return nil, diags
	}

	if len(elements) == 0 {
		return nil, diags
	}

	projectAccesses := make(map[string]*identityv1.ProjectAccess, len(elements))
	for _, access := range elements {
		var model projectAccessModel
		diags.Append(access.As(ctx, &model, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}
		role, err := enums.ToProjectAccessRole(model.Role.ValueString())
		if err != nil {
			diags.AddError("Failed to convert project access role", err.Error())
			return nil, diags
		}
		projectAccesses[model.ProjectID.ValueString()] = &identityv1.ProjectAccess{
			Role: role,
		}
	}

	return projectAccesses, diags
}

func getProjectSetFromSpec(ctx context.Context, spec *identityv1.Access) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics

	projectAccesses := types.SetNull(types.ObjectType{AttrTypes: projectAccessAttrs})
	if len(spec.GetProjectAccesses()) > 0 {
		projectAccessObjects := make([]types.Object, 0)
		for projectID, projectAccess := range spec.GetProjectAccesses() {
			role, err := enums.FromProjectAccessRole(projectAccess.GetRole())
			if err != nil {
				diags.AddError("Failed to convert project access role", err.Error())
				return projectAccesses, diags
			}
			model := projectAccessModel{
				ProjectID: types.StringValue(projectID),
				Role:      internaltypes.CaseInsensitiveString(role),
			}
			obj, d := types.ObjectValueFrom(ctx, projectAccessAttrs, model)
			diags.Append(d...)
			if diags.HasError() {
				return projectAccesses, diags
			}
			projectAccessObjects = append(projectAccessObjects, obj)
		}

		accesses, d := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: projectAccessAttrs}, projectAccessObjects)
		diags.Append(d...)
		if diags.HasError() {
			return projectAccesses, diags
		}

		projectAccesses = accesses
	}

	return projectAccesses, diags
}

func getNamespaceAccessesFromSet(ctx context.Context, set types.Set) (map[string]*identityv1.NamespaceAccess, diag.Diagnostics) {
	var diags diag.Diagnostics

	elements := make([]types.Object, 0, len(set.Elements()))
	diags.Append(set.ElementsAs(ctx, &elements, false)...)
	if diags.HasError() {
		return nil, diags
	}

	if len(elements) == 0 {
		return nil, diags
	}

	namespaceAccesses := make(map[string]*identityv1.NamespaceAccess, len(elements))
	for _, access := range elements {
		var model namespaceAccessModel
		diags.Append(access.As(ctx, &model, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}
		permission, err := enums.ToNamespaceAccessPermission(model.Permission.ValueString())
		if err != nil {
			diags.AddError("Failed to convert namespace permission", err.Error())
			return nil, diags
		}
		namespaceAccesses[model.NamespaceID.ValueString()] = &identityv1.NamespaceAccess{
			Permission: permission,
		}
	}

	return namespaceAccesses, diags
}

func getCustomRolesFromSet(ctx context.Context, set types.Set) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics

	if set.IsNull() || set.IsUnknown() {
		return nil, diags
	}

	var customRoles []string
	diags.Append(set.ElementsAs(ctx, &customRoles, false)...)
	if diags.HasError() {
		return nil, diags
	}

	if len(customRoles) == 0 {
		return nil, diags
	}

	return customRoles, diags
}

func getCustomRolesSet(ctx context.Context, customRoles []string) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics

	if len(customRoles) == 0 {
		return types.SetNull(types.StringType), diags
	}

	customRolesSet, d := types.SetValueFrom(ctx, types.StringType, customRoles)
	diags.Append(d...)
	if diags.HasError() {
		return types.SetNull(types.StringType), diags
	}

	return customRolesSet, diags
}

func getAccountAccessFromModel(ctx context.Context, accountAccess string, accountAccessCustomRolesSet types.Set) (*identityv1.AccountAccess, diag.Diagnostics) {
	var diags diag.Diagnostics

	role, err := enums.ToAccountAccessRole(accountAccess)
	if err != nil {
		diags.AddError("Failed to convert account access role", err.Error())
		return nil, diags
	}

	accountAccessCustomRoles, d := getCustomRolesFromSet(ctx, accountAccessCustomRolesSet)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	if role == identityv1.AccountAccess_ROLE_UNSPECIFIED && len(accountAccessCustomRoles) == 0 {
		return nil, diags
	}

	return &identityv1.AccountAccess{
		Role:        role,
		CustomRoles: accountAccessCustomRoles,
	}, diags
}

func getNamespaceSetFromSpec(ctx context.Context, spec *identityv1.Access) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics

	namespaceAccesses := types.SetNull(types.ObjectType{AttrTypes: namespaceAccessAttrs})
	if len(spec.GetNamespaceAccesses()) > 0 {
		namespaceAccessObjects := make([]types.Object, 0)
		for ns, namespaceAccess := range spec.GetNamespaceAccesses() {
			permission, err := enums.FromNamespaceAccessPermission(namespaceAccess.GetPermission())
			if err != nil {
				diags.AddError("Failed to convert namespace access permission", err.Error())
				return namespaceAccesses, diags
			}
			model := namespaceAccessModel{
				NamespaceID: types.StringValue(ns),
				Permission:  internaltypes.CaseInsensitiveString(permission),
			}
			obj, d := types.ObjectValueFrom(ctx, namespaceAccessAttrs, model)
			diags.Append(d...)
			if diags.HasError() {
				return namespaceAccesses, diags
			}
			namespaceAccessObjects = append(namespaceAccessObjects, obj)
		}

		accesses, d := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: namespaceAccessAttrs}, namespaceAccessObjects)
		diags.Append(d...)
		if diags.HasError() {
			return namespaceAccesses, diags
		}

		namespaceAccesses = accesses
	}

	return namespaceAccesses, diags
}
