package helperfunctions_test

import (
	"context"
	"testing"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const testNamespace = "test-namespace"

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	return scheme
}

func authPolicySelecting(matchLabels map[string]string) v1alpha1.AuthPolicy {
	return v1alpha1.AuthPolicy{
		Name: "test-policy", Namespace: testNamespace,
		Spec: v1alpha1.AuthPolicySpec{
			Selector: v1alpha1.WorkloadSelector{MatchLabels: matchLabels},
		},
	}
}

func deployment(name string, deploymentLabels, podTemplateLabels map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		Name: name, Namespace: testNamespace, Labels: deploymentLabels,
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: podTemplateLabels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: podTemplateLabels},
			},
		},
	}
}

func TestGetProtectedDeployment_ReturnsTheSingleMatchingDeployment(t *testing.T) {
	result, err := helperfunctions.GetProtectedDeployment(
		context.Background(),
		helperfunctions.GetMockKubernetesClient(
			newScheme(t),
			deployment("my-app", nil, map[string]string{"app": "my-app"}),
			deployment("other-app", nil, map[string]string{"app": "other-app"}),
		),
		authPolicySelecting(map[string]string{"app": "my-app"}),
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "my-app", result.Name)
	assert.Equal(t, testNamespace, result.Namespace)
}

func TestGetProtectedDeployment_MatchesSubsetOfPodTemplateLabels(t *testing.T) {
	result, err := helperfunctions.GetProtectedDeployment(
		context.Background(),
		helperfunctions.GetMockKubernetesClient(
			newScheme(t),
			deployment("my-app", nil, map[string]string{"app": "my-app", "team": "platform"}),
		),
		authPolicySelecting(map[string]string{"app": "my-app"}),
	)

	require.NoError(t, err)
	assert.Equal(t, "my-app", result.Name)
}

func TestGetProtectedDeployment_RequiresAllSelectorLabelsToMatch(t *testing.T) {
	_, err := helperfunctions.GetProtectedDeployment(
		context.Background(),
		helperfunctions.GetMockKubernetesClient(
			newScheme(t),
			deployment("my-app", nil, map[string]string{"app": "my-app", "version": "v1"}),
		),
		authPolicySelecting(map[string]string{"app": "my-app", "version": "v2"}),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no deployments matched the selector")
}

func TestGetProtectedDeployment_MatchesOnPodTemplateLabelsNotDeploymentLabels(t *testing.T) {
	_, err := helperfunctions.GetProtectedDeployment(
		context.Background(),
		helperfunctions.GetMockKubernetesClient(
			newScheme(t),
			deployment("my-app", map[string]string{"app": "my-app"}, map[string]string{"app": "something-else"}),
		),
		authPolicySelecting(map[string]string{"app": "my-app"}),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no deployments matched the selector")
}

func TestGetProtectedDeployment_ReturnsErrorWhenNoDeploymentsExist(t *testing.T) {
	result, err := helperfunctions.GetProtectedDeployment(
		context.Background(),
		helperfunctions.GetMockKubernetesClient(newScheme(t)),
		authPolicySelecting(map[string]string{"app": "my-app"}),
	)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no deployments matched the selector")
}

func TestGetProtectedDeployment_ReturnsErrorWhenMultipleDeploymentsMatch(t *testing.T) {
	result, err := helperfunctions.GetProtectedDeployment(
		context.Background(),
		helperfunctions.GetMockKubernetesClient(
			newScheme(t),
			deployment("my-app-a", nil, map[string]string{"app": "my-app"}),
			deployment("my-app-b", nil, map[string]string{"app": "my-app", "variant": "b"}),
		),
		authPolicySelecting(map[string]string{"app": "my-app"}),
	)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "more than 1 deployment matched the selector")
}

func TestGetProtectedDeployment_EmptySelectorMatchesEveryDeployment(t *testing.T) {
	t.Run("single deployment is returned", func(t *testing.T) {
		result, err := helperfunctions.GetProtectedDeployment(
			context.Background(),
			helperfunctions.GetMockKubernetesClient(
				newScheme(t),
				deployment("my-app", nil, map[string]string{"app": "my-app"}),
			),
			authPolicySelecting(map[string]string{}),
		)

		require.NoError(t, err)
		assert.Equal(t, "my-app", result.Name)
	})

	t.Run("multiple deployments is an error", func(t *testing.T) {
		_, err := helperfunctions.GetProtectedDeployment(
			context.Background(),
			helperfunctions.GetMockKubernetesClient(
				newScheme(t),
				deployment("my-app", nil, map[string]string{"app": "my-app"}),
				deployment("other-app", nil, map[string]string{"app": "other-app"}),
			),
			authPolicySelecting(nil),
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "more than 1 deployment matched the selector")
	})
}
