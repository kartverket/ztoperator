package envoy_test

import (
	"testing"

	httpconnectionmanagerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/kartverket/ztoperator/pkg/envoy"
	"github.com/kartverket/ztoperator/pkg/envoy/httpfilter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHttpFilterToEnvoyFilterConfigPatch_SelectsConfigPatchByHttpFilterName(t *testing.T) {
	tests := []struct {
		httpFilterName        string
		expectedSubFilterName string
	}{
		{
			httpFilterName:        httpfilter.EnvoyExtensionHttpFilterLua,
			expectedSubFilterName: "",
		},
		{
			httpFilterName:        httpfilter.EnvoyExtensionHttpFilterOAuth2,
			expectedSubFilterName: httpfilter.EnvoyExtensionHttpFilterJwtAuthentication,
		},
	}

	for _, tt := range tests {
		t.Run(tt.httpFilterName, func(t *testing.T) {
			configPatch, err := envoy.HttpFilterToEnvoyFilterConfigPatch(
				&httpconnectionmanagerv3.HttpFilter{Name: tt.httpFilterName},
			)
			require.NoError(t, err)
			require.NotNil(t, configPatch)

			assert.Equal(t,
				tt.expectedSubFilterName,
				configPatch.GetMatch().GetListener().GetFilterChain().GetFilter().GetSubFilter().GetName(),
			)
			assert.Equal(t, tt.httpFilterName, configPatch.GetPatch().GetValue().GetFields()["name"].GetStringValue())
		})
	}
}

func TestHttpFilterToEnvoyFilterConfigPatch_ReturnsErrorForUnsupportedHttpFilterName(t *testing.T) {
	for _, httpFilterName := range []string{
		httpfilter.EnvoyExtensionHttpFilterJwtAuthentication,
		httpfilter.EnvoyExtensionHttpFilterRouter,
		"envoy.filters.http.unknown",
		"",
	} {
		t.Run(httpFilterName, func(t *testing.T) {
			configPatch, err := envoy.HttpFilterToEnvoyFilterConfigPatch(
				&httpconnectionmanagerv3.HttpFilter{Name: httpFilterName},
			)
			require.Error(t, err)
			assert.Nil(t, configPatch)
		})
	}
}
