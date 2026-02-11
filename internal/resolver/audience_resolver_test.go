package resolver_test

import (
	"context"
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/resolver"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolveAudiences_WithNoAudiences_ReturnsEmptyList(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	k8sClient := createFakeClientForAudiences()

	// 2. Act
	result, err := resolver.ResolveAudiences(
		ctx,
		k8sClient,
		"default",
		[]ztoperatorv1alpha1.AllowedAudience{},
	)

	// 3. Assert
	require.NoError(t, err, "ResolveAudiences should not return an error with empty audiences")
	require.NotNil(t, result, "Result should not be nil")
	assert.Empty(t, *result, "Result should be empty list")
}

func TestResolveAudiences_WithStaticValueAudience_ReturnsValue(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	allowedAudiences := []ztoperatorv1alpha1.AllowedAudience{
		{Value: helperfunctions.Ptr("my-static-audience")},
	}
	k8sClient := createFakeClientForAudiences()

	// 2. Act
	result, err := resolver.ResolveAudiences(ctx, k8sClient, "default", allowedAudiences)

	// 3. Assert
	require.NoError(t, err, "ResolveAudiences should not return an error with static value")
	require.NotNil(t, result, "Result should not be nil")
	assert.Equal(t, []string{"my-static-audience"}, *result, "Result should contain static audience")
}

func TestResolveAudiences_WithEmptyStaticValue_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	allowedAudiences := []ztoperatorv1alpha1.AllowedAudience{
		{Value: helperfunctions.Ptr("")},
	}
	k8sClient := createFakeClientForAudiences()

	// 2. Act
	result, err := resolver.ResolveAudiences(ctx, k8sClient, "default", allowedAudiences)

	// 3. Assert
	require.Error(t, err, "ResolveAudiences should return an error with empty static value")
	assert.Nil(t, result, "Result should be nil on error")
	assert.Contains(t, err.Error(), "audience value cannot be empty")
}

func TestResolveAudiences_WithConfigMapRef_ReturnsConfigMapValue(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "audience-configmap",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"audience-key": "audience-from-configmap",
		},
	}

	allowedAudiences := []ztoperatorv1alpha1.AllowedAudience{
		{
			ValueFrom: &ztoperatorv1alpha1.ValueFrom{
				ConfigMapKeyRef: &ztoperatorv1alpha1.KeyRef{
					Name: "audience-configmap",
					Key:  "audience-key",
				},
			},
		},
	}
	k8sClient := createFakeClientForAudiences(configMap)

	// 2. Act
	result, err := resolver.ResolveAudiences(ctx, k8sClient, "test-namespace", allowedAudiences)

	// 3. Assert
	require.NoError(t, err, "ResolveAudiences should not return an error with valid ConfigMap")
	require.NotNil(t, result, "Result should not be nil")
	assert.Equal(t, []string{"audience-from-configmap"}, *result, "Result should contain audience from ConfigMap")
}

func TestResolveAudiences_WithMissingConfigMap_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	allowedAudiences := []ztoperatorv1alpha1.AllowedAudience{
		{
			ValueFrom: &ztoperatorv1alpha1.ValueFrom{
				ConfigMapKeyRef: &ztoperatorv1alpha1.KeyRef{
					Name: "non-existent-configmap",
					Key:  "audience-key",
				},
			},
		},
	}
	k8sClient := createFakeClientForAudiences()

	// 2. Act
	result, err := resolver.ResolveAudiences(ctx, k8sClient, "default", allowedAudiences)

	// 3. Assert
	require.Error(t, err, "ResolveAudiences should return an error when ConfigMap is missing")
	assert.Nil(t, result, "Result should be nil on error")
}

func TestResolveAudiences_WithSecretRef_ReturnsSecretValue(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "audience-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"audience-key": []byte("audience-from-secret"),
		},
	}

	allowedAudiences := []ztoperatorv1alpha1.AllowedAudience{
		{
			ValueFrom: &ztoperatorv1alpha1.ValueFrom{
				SecretKeyRef: &ztoperatorv1alpha1.KeyRef{
					Name: "audience-secret",
					Key:  "audience-key",
				},
			},
		},
	}
	k8sClient := createFakeClientForAudiences(secret)

	// 2. Act
	result, err := resolver.ResolveAudiences(ctx, k8sClient, "test-namespace", allowedAudiences)

	// 3. Assert
	require.NoError(t, err, "ResolveAudiences should not return an error with valid Secret")
	require.NotNil(t, result, "Result should not be nil")
	assert.Equal(t, []string{"audience-from-secret"}, *result, "Result should contain audience from Secret")
}

func TestResolveAudiences_WithMissingSecret_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	allowedAudiences := []ztoperatorv1alpha1.AllowedAudience{
		{
			ValueFrom: &ztoperatorv1alpha1.ValueFrom{
				SecretKeyRef: &ztoperatorv1alpha1.KeyRef{
					Name: "non-existent-secret",
					Key:  "audience-key",
				},
			},
		},
	}
	k8sClient := createFakeClientForAudiences()

	// 2. Act
	result, err := resolver.ResolveAudiences(ctx, k8sClient, "default", allowedAudiences)

	// 3. Assert
	require.Error(t, err, "ResolveAudiences should return an error when Secret is missing")
	assert.Nil(t, result, "Result should be nil on error")
}

func TestResolveAudiences_WithMultipleAudiences_ReturnsAllAudiences(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "audience-configmap",
			Namespace: "default",
		},
		Data: map[string]string{
			"audience-key": "audience-from-configmap",
		},
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "audience-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"audience-key": []byte("audience-from-secret"),
		},
	}

	allowedAudiences := []ztoperatorv1alpha1.AllowedAudience{
		{Value: helperfunctions.Ptr("static-audience")},
		{
			ValueFrom: &ztoperatorv1alpha1.ValueFrom{
				ConfigMapKeyRef: &ztoperatorv1alpha1.KeyRef{
					Name: "audience-configmap",
					Key:  "audience-key",
				},
			},
		},
		{
			ValueFrom: &ztoperatorv1alpha1.ValueFrom{
				SecretKeyRef: &ztoperatorv1alpha1.KeyRef{
					Name: "audience-secret",
					Key:  "audience-key",
				},
			},
		},
	}
	k8sClient := createFakeClientForAudiences(configMap, secret)

	// 2. Act
	result, err := resolver.ResolveAudiences(ctx, k8sClient, "default", allowedAudiences)

	// 3. Assert
	require.NoError(t, err, "ResolveAudiences should not return an error with multiple valid audiences")
	require.NotNil(t, result, "Result should not be nil")
	expected := []string{"static-audience", "audience-from-configmap", "audience-from-secret"}
	assert.Equal(t, expected, *result, "Result should contain all audiences in correct order")
}

func TestResolveAudiences_WithBothValueAndValueFrom_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	allowedAudiences := []ztoperatorv1alpha1.AllowedAudience{
		{
			Value: helperfunctions.Ptr("my-static-audience"),
			ValueFrom: &ztoperatorv1alpha1.ValueFrom{
				ConfigMapKeyRef: &ztoperatorv1alpha1.KeyRef{
					Name: "my-configmap",
					Key:  "audience",
				},
			},
		},
	}
	k8sClient := createFakeClientForAudiences()

	// 2. Act
	result, err := resolver.ResolveAudiences(ctx, k8sClient, "default", allowedAudiences)

	// 3. Assert
	require.Error(t, err, "ResolveAudiences should return an error when both value and valueFrom are set")
	assert.Nil(t, result, "Result should be nil on error")
	assert.Contains(t, err.Error(), "cannot define an audience as both string and ConfigMap/Secret ref")
}

func TestResolveAudiences_WithBothConfigMapAndSecretRef_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	allowedAudiences := []ztoperatorv1alpha1.AllowedAudience{
		{
			ValueFrom: &ztoperatorv1alpha1.ValueFrom{
				ConfigMapKeyRef: &ztoperatorv1alpha1.KeyRef{
					Name: "my-configmap",
					Key:  "audience",
				},
				SecretKeyRef: &ztoperatorv1alpha1.KeyRef{
					Name: "my-secret",
					Key:  "audience",
				},
			},
		},
	}
	k8sClient := createFakeClientForAudiences()

	// 2. Act
	result, err := resolver.ResolveAudiences(ctx, k8sClient, "default", allowedAudiences)

	// 3. Assert
	require.Error(t, err, "ResolveAudiences should return an error when both ConfigMap and Secret refs are set")
	assert.Nil(t, result, "Result should be nil on error")
	assert.Contains(t, err.Error(), "failed to resolve audience reference")
	assert.Contains(t, err.Error(), "cannot get value from both ConfigMap and Secret")
}

func TestResolveAudiences_WithEmptyConfigMapValue_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "audience-configmap",
			Namespace: "default",
		},
		Data: map[string]string{
			"audience-key": "", // Empty value
		},
	}

	allowedAudiences := []ztoperatorv1alpha1.AllowedAudience{
		{
			ValueFrom: &ztoperatorv1alpha1.ValueFrom{
				ConfigMapKeyRef: &ztoperatorv1alpha1.KeyRef{
					Name: "audience-configmap",
					Key:  "audience-key",
				},
			},
		},
	}
	k8sClient := createFakeClientForAudiences(configMap)

	// 2. Act
	result, err := resolver.ResolveAudiences(ctx, k8sClient, "default", allowedAudiences)

	// 3. Assert
	require.Error(t, err, "ResolveAudiences should return an error with empty ConfigMap value")
	assert.Nil(t, result, "Result should be nil on error")
	assert.Contains(t, err.Error(), "audience value from configmap")
	assert.Contains(t, err.Error(), "is empty or missing")
}

func TestResolveAudiences_WithEmptySecretValue_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "audience-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"audience-key": []byte(""), // Empty value
		},
	}

	allowedAudiences := []ztoperatorv1alpha1.AllowedAudience{
		{
			ValueFrom: &ztoperatorv1alpha1.ValueFrom{
				SecretKeyRef: &ztoperatorv1alpha1.KeyRef{
					Name: "audience-secret",
					Key:  "audience-key",
				},
			},
		},
	}
	k8sClient := createFakeClientForAudiences(secret)

	// 2. Act
	result, err := resolver.ResolveAudiences(ctx, k8sClient, "default", allowedAudiences)

	// 3. Assert
	require.Error(t, err, "ResolveAudiences should return an error with empty Secret value")
	assert.Nil(t, result, "Result should be nil on error")
	assert.Contains(t, err.Error(), "audience value from secret")
	assert.Contains(t, err.Error(), "is empty or missing")
}

func TestResolveAudiences_WithMissingConfigMapKey_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "audience-configmap",
			Namespace: "default",
		},
		Data: map[string]string{
			"other-key": "some-value",
		},
	}

	allowedAudiences := []ztoperatorv1alpha1.AllowedAudience{
		{
			ValueFrom: &ztoperatorv1alpha1.ValueFrom{
				ConfigMapKeyRef: &ztoperatorv1alpha1.KeyRef{
					Name: "audience-configmap",
					Key:  "missing-key",
				},
			},
		},
	}
	k8sClient := createFakeClientForAudiences(configMap)

	// 2. Act
	result, err := resolver.ResolveAudiences(ctx, k8sClient, "default", allowedAudiences)

	// 3. Assert
	require.Error(t, err, "ResolveAudiences should return an error with missing ConfigMap key")
	assert.Nil(t, result, "Result should be nil on error")
	assert.Contains(t, err.Error(), "audience value from configmap")
	assert.Contains(t, err.Error(), "is empty or missing")
}

func TestResolveAudiences_WithMissingSecretKey_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "audience-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"other-key": []byte("some-value"),
		},
	}

	allowedAudiences := []ztoperatorv1alpha1.AllowedAudience{
		{
			ValueFrom: &ztoperatorv1alpha1.ValueFrom{
				SecretKeyRef: &ztoperatorv1alpha1.KeyRef{
					Name: "audience-secret",
					Key:  "missing-key",
				},
			},
		},
	}
	k8sClient := createFakeClientForAudiences(secret)

	// 2. Act
	result, err := resolver.ResolveAudiences(ctx, k8sClient, "default", allowedAudiences)

	// 3. Assert
	require.Error(t, err, "ResolveAudiences should return an error with missing Secret key")
	assert.Nil(t, result, "Result should be nil on error")
	assert.Contains(t, err.Error(), "audience value from secret")
	assert.Contains(t, err.Error(), "is empty or missing")
}

func createFakeClientForAudiences(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = ztoperatorv1alpha1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}
