package authpolicy_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAuthPolicy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AuthPolicy webhook suite")
}
