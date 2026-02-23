package authorizationpolicytest_test

import (
	"reflect"
	"testing"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy"
)

func TestConstructAcceptedResourcesWithAudienceNil(t *testing.T) {
	scope := state.Scope{
		Audiences: nil,
	}
	actualValues := authorizationpolicy.ConstructAcceptedResources(scope)

	if len(actualValues) != 0 {
		t.Fatalf("expecting an empty array, but got an array with %d", len(actualValues))
	}
}

func TestConstructAcceptedResourcesWithAudience(t *testing.T) {
	expectedAudiences := []string{
		"audience1",
		"audience2",
		"audience3",
	}
	scope := state.Scope{
		Audiences: expectedAudiences,
	}
	actualValues := authorizationpolicy.ConstructAcceptedResources(scope)

	if !reflect.DeepEqual(expectedAudiences, actualValues) {
		t.Fatalf("expected != actual, expected: %v\n, actual: %v\n", expectedAudiences, actualValues)
	}
}

func TestConstructAcceptedResourcesWithAudienceAndAcceptedResources(t *testing.T) {
	expectedAudiences := []string{
		"audience1",
		"audience2",
		"audience3",
	}
	expectedAcceptedResources := []string{
		"https://audience1.com",
		"https://audience2.com",
		"https://audience3.com",
	}

	expectedValues := make([]string, 0, len(expectedAudiences)+len(expectedAcceptedResources))
	expectedValues = append(expectedValues, expectedAudiences...)
	expectedValues = append(expectedValues, expectedAcceptedResources...)

	scope := state.Scope{
		Audiences: expectedAudiences,
		AuthPolicy: v1alpha1.AuthPolicy{
			Spec: v1alpha1.AuthPolicySpec{
				AcceptedResources: &expectedAcceptedResources,
			},
		},
	}
	actualValues := authorizationpolicy.ConstructAcceptedResources(scope)

	if !reflect.DeepEqual(expectedValues, actualValues) {
		t.Fatalf("expected != actual, expected: %v\n, actual: %v\n", expectedValues, actualValues)
	}
}
