package resolver

import (
	"fmt"
	"net/netip"

	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/model"
	envoymodel "github.com/kartverket/ztoperator/pkg/model/envoy"
	appsv1 "k8s.io/api/apps/v1"
)

const (
	loopbackAddress = "127.0.0.1"
	anyAddress      = "0.0.0.0"
)

func ResolveEnvoyBootstrap(
	authPolicy v1alpha1.AuthPolicy,
	autoLoginConfig model.AutoLoginConfig,
	oAuthCredentials model.OAuthCredentials,
	identityProviderUris model.IdentityProviderUris,
	audiences []string,
	deployment appsv1.Deployment,
) (*bootstrapv3.Bootstrap, error) {
	skiperatorAppPort, getSkiperatorAppPortErr := helperfunctions.GetSkiperatorAppPort(deployment)
	if getSkiperatorAppPortErr != nil {
		return nil, fmt.Errorf(
			"failed to get Skiperator app port from deployment %s/%s: %w",
			deployment.Namespace,
			deployment.Name,
			getSkiperatorAppPortErr,
		)
	}

	return envoymodel.ToEnvoyBootstrap(
		authPolicy,
		autoLoginConfig,
		identityProviderUris,
		audiences,
		oAuthCredentials.ClientID,
		fmt.Sprintf("http://%s:%d", loopbackAddress, skiperatorAppPort),
		envoymodel.BindAddresses{
			Ingress: netip.AddrPortFrom(
				netip.MustParseAddr(anyAddress),
				uint16(config.Get().EnvoyPort),
			),
			Probe: netip.AddrPortFrom(
				netip.MustParseAddr(anyAddress),
				uint16(config.Get().EnvoyProbePort),
			),
			Admin: netip.AddrPortFrom(
				netip.MustParseAddr(loopbackAddress),
				uint16(config.Get().EnvoyAdminPort),
			),
		},
	)
}
