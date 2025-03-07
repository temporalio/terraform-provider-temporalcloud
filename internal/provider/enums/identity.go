package enums

import (
	"errors"
	"fmt"
	"strings"

	"go.temporal.io/api/cloud/identity/v1"
)

var (
	ErrInvalidOwnerType                 = errors.New("invalid owner type")
	ErrInvalidAccountAccessRole         = errors.New("invalid account access role")
	ErrInvalidNamespaceAccessPermission = errors.New("invalid namespace access permission")
)

func ToOwnerType(s string) (identity.OwnerType, error) {
	switch strings.ToLower(s) {
	case "user":
		return identity.OWNER_TYPE_USER, nil
	case "service-account":
		return identity.OWNER_TYPE_SERVICE_ACCOUNT, nil
	default:
		return identity.OWNER_TYPE_UNSPECIFIED, fmt.Errorf("%w: %s", ErrInvalidOwnerType, s)
	}
}

func FromOwnerType(t identity.OwnerType) (string, error) {
	switch t {
	case identity.OWNER_TYPE_USER:
		return "user", nil
	case identity.OWNER_TYPE_SERVICE_ACCOUNT:
		return "service-account", nil
	default:
		return "", fmt.Errorf("%w: %v", ErrInvalidOwnerType, t)
	}
}

func ToAccountAccessRole(s string) (identity.AccountAccess_Role, error) {
	switch strings.ToLower(s) {
	case "owner":
		return identity.AccountAccess_ROLE_OWNER, nil
	case "admin":
		return identity.AccountAccess_ROLE_ADMIN, nil
	case "developer":
		return identity.AccountAccess_ROLE_DEVELOPER, nil
	case "read":
		return identity.AccountAccess_ROLE_READ, nil
	case "financeadmin":
		return identity.AccountAccess_ROLE_FINANCE_ADMIN, nil
	case "none":
		return identity.AccountAccess_ROLE_UNSPECIFIED, nil
	default:
		return identity.AccountAccess_ROLE_UNSPECIFIED, fmt.Errorf("%w: %s", ErrInvalidAccountAccessRole, s)
	}
}

func FromAccountAccessRole(r identity.AccountAccess_Role) (string, error) {
	switch r {
	case identity.AccountAccess_ROLE_OWNER:
		return "owner", nil
	case identity.AccountAccess_ROLE_ADMIN:
		return "admin", nil
	case identity.AccountAccess_ROLE_DEVELOPER:
		return "developer", nil
	case identity.AccountAccess_ROLE_READ:
		return "read", nil
	case identity.AccountAccess_ROLE_FINANCE_ADMIN:
		return "financeadmin", nil
	case identity.AccountAccess_ROLE_UNSPECIFIED:
		return "none", nil
	default:
		return "", fmt.Errorf("%w: %v", ErrInvalidAccountAccessRole, r)
	}
}

func ToNamespaceAccessPermission(s string) (identity.NamespaceAccess_Permission, error) {
	switch strings.ToLower(s) {
	case "admin":
		return identity.NamespaceAccess_PERMISSION_ADMIN, nil
	case "write":
		return identity.NamespaceAccess_PERMISSION_WRITE, nil
	case "read":
		return identity.NamespaceAccess_PERMISSION_READ, nil
	default:
		return identity.NamespaceAccess_PERMISSION_UNSPECIFIED, fmt.Errorf("%w: %s", ErrInvalidNamespaceAccessPermission, s)
	}
}

func FromNamespaceAccessPermission(p identity.NamespaceAccess_Permission) (string, error) {
	switch p {
	case identity.NamespaceAccess_PERMISSION_ADMIN:
		return "admin", nil
	case identity.NamespaceAccess_PERMISSION_WRITE:
		return "write", nil
	case identity.NamespaceAccess_PERMISSION_READ:
		return "read", nil
	default:
		return "", fmt.Errorf("%w: %v", ErrInvalidNamespaceAccessPermission, p)
	}
}
