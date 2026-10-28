package authpolicy_test

import (
	"os"
	"testing"

	"github.com/kartverket/ztoperator/pkg/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAuthPolicy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AuthPolicy webhook suite")
}

var _ = BeforeSuite(func() {
	providers := `{
		"required": {
			"wellKnownURI": "` + webhookAllowedURI + `",
			"requireAudience": true,
			"issuer": "https://required.example.com",
			"authorizationEndpoint": "https://required.example.com/authorize",
			"tokenEndpoint": "https://required.example.com/token",
			"jwksURI": "https://required.example.com/jwks",
			"endSessionEndpoint": "https://required.example.com/logout"
		},
		"optional": {
			"wellKnownURI": "` + webhookOptionalURI + `",
			"requireAudience": false,
			"issuer": "https://optional.example.com",
			"tokenEndpoint": "https://optional.example.com/token",
			"jwksURI": "https://optional.example.com/jwks"
		}
	}`
	Expect(os.Setenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS", providers)).To(Succeed())
	Expect(config.Load()).To(Succeed())
})
