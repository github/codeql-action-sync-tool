package actionconfiguration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStandardConfiguration(t *testing.T) {
	result, err := Parse("{\"bundleVersion\": \"test\"}")
	require.NoError(t, err)
	require.Equal(t, "test", result.BundleVersion)
}

func TestExtraPropertiesIgnored(t *testing.T) {
	result, err := Parse("{\"bundleVersion\": \"test\", \"someOtherThing\": \"blah\"}")
	require.NoError(t, err)
	require.Equal(t, "test", result.BundleVersion)
}

func TestErrorIfInvalidJSON(t *testing.T) {
	_, err := Parse("{")
	require.Error(t, err)
}

func TestErrorIfMissingProperties(t *testing.T) {
	_, err := Parse("{\"someOtherThing\": \"blah\"}")
	require.EqualError(t, err, errorBundleVersionNotSet)
}
