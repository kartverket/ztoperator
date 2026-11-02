package resolver

import (
	"fmt"

	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/constants"
	"github.com/kartverket/ztoperator/pkg/envoy/cluster"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/model"
	envoymodel "github.com/kartverket/ztoperator/pkg/model/envoy"
	appsv1 "k8s.io/api/apps/v1"
)

func ResolveEnvoyBootstrap(
	authPolicy v1alpha1.AuthPolicy,
	autoLoginConfig model.AutoLoginConfig,
	oAuthCredentials model.OAuthCredentials,
	identityProviderUris model.IdentityProviderUris,
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

	upstreamCluster, err := cluster.FromURI(
		constants.EnvoyBootstrapUpstreamClusterName,
		fmt.Sprintf("http://%s:%d", constants.LoopbackAddress, skiperatorAppPort),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream cluster: %w", err)
	}

	return envoymodel.ToEnvoyBootstrap(
		authPolicy,
		autoLoginConfig,
		identityProviderUris,
		upstreamCluster,
		oAuthCredentials.ClientID,
	)
}
