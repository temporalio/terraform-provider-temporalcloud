package enums

import (
	"errors"
	"fmt"
	"strings"

	"go.temporal.io/api/cloud/namespace/v1"
)

var (
	ErrInvalidNamespaceSearchAttribute = errors.New("invalid namespace search attribute")
)

func ToNamespaceSearchAttribute(s string) (namespace.NamespaceSpec_SearchAttributeType, error) {
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
	case "keyword_list", "keywordlist":
		return namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD_LIST, nil
	default:
		return namespace.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_UNSPECIFIED, fmt.Errorf("%w: %s", ErrInvalidNamespaceSearchAttribute, s)
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