package actionconfiguration

import (
	"encoding/json"

	"github.com/pkg/errors"
)

const errorBundleVersionNotSet = "The property \"bundleVersion\" was not set in the Action default configuration."

type ActionConfiguration struct {
	BundleVersion string
}

func Parse(contents string) (*ActionConfiguration, error) {
	var result ActionConfiguration
	err := json.Unmarshal([]byte(contents), &result)
	if err != nil {
		return nil, errors.Wrap(err, "Error decoding Action default configuration.")
	}
	if result.BundleVersion == "" {
		return nil, errors.New(errorBundleVersionNotSet)
	}
	return &result, nil
}
