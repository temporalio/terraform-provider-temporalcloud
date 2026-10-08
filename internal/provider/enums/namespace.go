package enums

import (
	"errors"
	"fmt"
	"strings"

	"go.temporal.io/cloud-sdk/api/namespace/v1"
)

var (
	ErrInvalidNamespaceSearchAttribute = errors.New("invalid namespace search attribute")
	ErrInvalidEncryptionValidationMode = errors.New("invalid encryption validation mode")
)

func ToEncryptionValidationMode(value string) namespace.EncryptionValidationSpec_EncryptionValidationMode {
	switch strings.ToLower(value) {
	case "disabled":
		return namespace.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DISABLED
	case "warn":
		return namespace.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_WARN
	case "deny":
		return namespace.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DENY
	default:
		// An omitted mode is represented by "". Non-empty invalid values
		// are rejected by the mode attribute validator before this conversion.
		return namespace.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_UNSPECIFIED
	}
}

func FromEncryptionValidationMode(value namespace.EncryptionValidationSpec_EncryptionValidationMode) (string, error) {
	switch value {
	case namespace.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DISABLED:
		return "disabled", nil
	case namespace.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_WARN:
		return "warn", nil
	case namespace.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DENY:
		return "deny", nil
	default:
		return "", fmt.Errorf("%w: %v", ErrInvalidEncryptionValidationMode, value)
	}
}

func ToNamespaceSearchAttribute(s string, strict bool) (namespace.NamespaceSpec_SearchAttributeType, error) {
	switch strings.ToLower(s) {
	case "text":
		return namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_TEXT, nil
	case "keyword":
		return namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD, nil
	case "int":
		return namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_INT, nil
	case "double":
		return namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DOUBLE, nil
	case "bool":
		return namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_BOOL, nil
	case "datetime":
		return namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DATETIME, nil
	case "keyword_list":
		return namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD_LIST, nil
	case "keywordlist", "keyword-list":
		if !strict {
			return namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD_LIST, nil
		}
		fallthrough
	default:
		return namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_UNSPECIFIED, fmt.Errorf("%w: '%s', must be one of: text, keyword, int, double, bool, datetime, keyword_list", ErrInvalidNamespaceSearchAttribute, s)
	}
}

func FromNamespaceSearchAttribute(r namespace.NamespaceSpec_SearchAttributeType) (string, error) {
	switch r {
	case namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_TEXT:
		return "text", nil
	case namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD:
		return "keyword", nil
	case namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_INT:
		return "int", nil
	case namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DOUBLE:
		return "double", nil
	case namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_BOOL:
		return "bool", nil
	case namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DATETIME:
		return "datetime", nil
	case namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD_LIST:
		return "keyword_list", nil
	default:
		return "", fmt.Errorf("%w: %v", ErrInvalidNamespaceSearchAttribute, r)
	}
}
