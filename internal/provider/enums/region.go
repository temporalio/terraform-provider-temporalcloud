package enums

import (
	"errors"
	"fmt"

	"go.temporal.io/cloud-sdk/api/region/v1"
)

var (
	ErrInvalidRegionCloudProvider = errors.New("invalid region cloud provider")
)

func FromRegionCloudProvider(p region.Region_CloudProvider) (string, error) {
	switch p {
	case region.Region_CLOUD_PROVIDER_AWS:
		return "aws", nil
	case region.Region_CLOUD_PROVIDER_GCP:
		return "gcp", nil
	default:
		return "", fmt.Errorf("%w: %v", ErrInvalidRegionCloudProvider, p)
	}
}
