package validation

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"go.temporal.io/cloud-sdk/api/identity/v1"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/types"
)

type namespaceAccessValidator struct {
	accountAccessAttrName string
}

func NewNamespaceAccessValidator(accountAccessAttrName string) validator.Set {
	return namespaceAccessValidator{
		accountAccessAttrName: accountAccessAttrName,
	}
}

func (v namespaceAccessValidator) Description(_ context.Context) string {
	return "Validates that the namespace accesses are valid for the user."
}

func (v namespaceAccessValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v namespaceAccessValidator) ValidateSet(ctx context.Context, req validator.SetRequest, resp *validator.SetResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	var accountAccess types.CaseInsensitiveStringValue
	accountAccessAttrPath := req.Path.ParentPath().AtName(v.accountAccessAttrName)
	diags := req.Config.GetAttribute(ctx, accountAccessAttrPath, &accountAccess)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	accountRole, err := enums.ToAccountAccessRole(accountAccess.ValueString())
	if err != nil {
		// leave it upto the account_access validator to handle the error
		return
	}
	if accountRole == identity.AccountAccess_ROLE_OWNER || accountRole == identity.AccountAccess_ROLE_ADMIN {
		// Users with account_access roles of owner or admin cannot be assigned explicit permissions to namespaces.
		// They implicitly receive access to all Namespaces.
		if len(req.ConfigValue.Elements()) > 0 {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid Namespace Accesses",
				"Users with account_access roles of owner or admin cannot have namespace accesses. Remove the namespace_accesses attribute.",
			)
		}
	}
}
