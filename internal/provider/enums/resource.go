package enums

import (
	"errors"
	"fmt"

	"go.temporal.io/cloud-sdk/api/resource/v1"
)

var (
	ErrInvalidResourceState = errors.New("invalid resource state")
)

func FromResourceState(p resource.ResourceState) (string, error) {
	switch p {
	case resource.ResourceState_RESOURCE_STATE_ACTIVATING:
		return "activating", nil
	case resource.ResourceState_RESOURCE_STATE_ACTIVATION_FAILED:
		return "activationfailed", nil
	case resource.ResourceState_RESOURCE_STATE_ACTIVE:
		return "active", nil
	case resource.ResourceState_RESOURCE_STATE_UPDATING:
		return "updating", nil
	case resource.ResourceState_RESOURCE_STATE_UPDATE_FAILED:
		return "updatefailed", nil
	case resource.ResourceState_RESOURCE_STATE_DELETING:
		return "deleting", nil
	case resource.ResourceState_RESOURCE_STATE_DELETE_FAILED:
		return "deletefailed", nil
	case resource.ResourceState_RESOURCE_STATE_DELETED:
		return "deleted", nil
	case resource.ResourceState_RESOURCE_STATE_SUSPENDED:
		return "suspended", nil
	case resource.ResourceState_RESOURCE_STATE_EXPIRED:
		return "expired", nil
	default:
		return "", fmt.Errorf("%w: %v", ErrInvalidResourceState, p)
	}
}
