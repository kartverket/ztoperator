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
		"required": {"wellKnownURI": "` + webhookAllowedURI + `", "requireAudience": true},
		"optional": {"wellKnownURI": "` + webhookOptionalURI + `", "requireAudience": false}
	}`
	Expect(os.Setenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS", providers)).To(Succeed())
	Expect(config.Load()).To(Succeed())
})
