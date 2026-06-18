package authorizationpolicy_test

import (
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBaselineAuthConditionsForAllowPolicy_WithNilBaselineAuth_ReturnsEmptySlice(t *testing.T) {
	// 1. Arrange + 2. Act
	result := authorizationpolicy.GetBaselineAuthConditionsForAllowPolicy(nil)

	// 3. Assert
	assert.Empty(t, result)
}

func TestGetBaselineAuthConditionsForAllowPolicy_WithEmptyClaims_ReturnsEmptySlice(t *testing.T) {
	// 1. Arrange
	baselineAuth := &ztoperatorv1alpha1.BaselineAuth{
		Claims: []ztoperatorv1alpha1.Condition{},
	}

	// 2. Act
	result := authorizationpolicy.GetBaselineAuthConditionsForAllowPolicy(baselineAuth)

	// 3. Assert
	assert.Empty(t, result)
}

func TestGetBaselineAuthConditionsForAllowPolicy_WithClaim_ReturnsIstioConditionWithValues(t *testing.T) {
	// 1. Arrange
	baselineAuth := &ztoperatorv1alpha1.BaselineAuth{
		Claims: []ztoperatorv1alpha1.Condition{
			{Claim: "role", Values: []string{"admin"}},
		},
	}

	// 2. Act
	result := authorizationpolicy.GetBaselineAuthConditionsForAllowPolicy(baselineAuth)

	// 3. Assert
	require.Len(t, result, 1)
	assert.Equal(t, "request.auth.claims[role]", result[0].Key)
	assert.Equal(t, []string{"admin"}, result[0].Values)
	assert.Empty(t, result[0].NotValues)
}

func TestGetBaselineAuthConditionsForDenyPolicy_WithNilBaselineAuth_ReturnsEmptySlice(t *testing.T) {
	// 1. Arrange + 2. Act
	result := authorizationpolicy.GetBaselineAuthConditionsForDenyPolicy(nil)

	// 3. Assert
	assert.Empty(t, result)
}

func TestGetBaselineAuthConditionsForDenyPolicy_WithClaim_ReturnsIstioConditionWithNotValues(t *testing.T) {
	// 1. Arrange
	baselineAuth := &ztoperatorv1alpha1.BaselineAuth{
		Claims: []ztoperatorv1alpha1.Condition{
			{Claim: "role", Values: []string{"admin"}},
		},
	}

	// 2. Act
	result := authorizationpolicy.GetBaselineAuthConditionsForDenyPolicy(baselineAuth)

	// 3. Assert
	require.Len(t, result, 1)
	assert.Equal(t, "request.auth.claims[role]", result[0].Key)
	assert.Equal(t, []string{"admin"}, result[0].NotValues)
	assert.Empty(t, result[0].Values)
}

func TestGetBaselineAuthConditionsForDenyPolicy_WithMultipleClaims_ReturnsOneConditionPerClaim(t *testing.T) {
	// 1. Arrange
	baselineAuth := &ztoperatorv1alpha1.BaselineAuth{
		Claims: []ztoperatorv1alpha1.Condition{
			{Claim: "role", Values: []string{"admin"}},
			{Claim: "tenant", Values: []string{"acme", "globex"}},
		},
	}

	// 2. Act
	result := authorizationpolicy.GetBaselineAuthConditionsForDenyPolicy(baselineAuth)

	// 3. Assert
	require.Len(t, result, 2)
	assert.Equal(t, "request.auth.claims[role]", result[0].Key)
	assert.Equal(t, []string{"admin"}, result[0].NotValues)
	assert.Equal(t, "request.auth.claims[tenant]", result[1].Key)
	assert.Equal(t, []string{"acme", "globex"}, result[1].NotValues)
}
