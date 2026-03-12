package state_test

import (
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReplaceDescendant_ReplacesExistingEntry(t *testing.T) {
	s := &state.Scope{}
	secretName := "my-policy-envoy-secret"
	initialSecret := newSecret(secretName)
	updatedSecret := newSecret(secretName)
	updatedSecret.StringData = map[string]string{"client-secret": "rotated"}

	// First call — registers the descendant with an error.
	errMsg := "something went wrong"
	s.ReplaceDescendant(initialSecret, &errMsg, nil, "Secret", secretName)

	// Second call — should replace the entry and overwrite previous status.
	successMsg := "some success msg"
	s.ReplaceDescendant(updatedSecret, nil, &successMsg, "Secret", secretName)

	assert.Len(t, s.Descendants, 1, "expected exactly one descendant entry after replacing")
	descendant := s.Descendants[0]

	assert.Equal(t, state.GetID("Secret", secretName), descendant.ID)
	assert.Same(t, updatedSecret, descendant.Object)

	assert.Nil(t, descendant.ErrorMessage, "expected stale error message to be cleared")
	assert.Empty(t, s.GetErrors(), "expected no errors after a successful replacement")

	assert.NotNil(t, descendant.SuccessMessage)
	assert.Equal(t, successMsg, *descendant.SuccessMessage)

}

func TestReplaceDescendant_AppendsWhenExistingIDDoesNotMatch(t *testing.T) {
	s := &state.Scope{
		Descendants: []state.Descendant[client.Object]{
			{
				ID:     state.GetID("Secret", "other-secret"),
				Object: newSecret("other-secret"),
			},
		},
	}
	secretName := "envoy-secret"
	secret := newSecret(secretName)
	successMsg := "envoy secret success msg"

	s.ReplaceDescendant(secret, nil, &successMsg, "Secret", secretName)

	require.Len(t, s.Descendants, 2)
	assert.Equal(t, state.GetID("Secret", "other-secret"), s.Descendants[0].ID)
	assert.Equal(t, state.GetID("Secret", secretName), s.Descendants[1].ID)
	assert.Same(t, secret, s.Descendants[1].Object)
	require.NotNil(t, s.Descendants[1].SuccessMessage)
	assert.Equal(t, successMsg, *s.Descendants[1].SuccessMessage)
}

func TestGetErrors_ReturnsOnlyDescendantErrors(t *testing.T) {
	s := &state.Scope{}
	firstErr := "first error"
	secondErr := "second error"
	successMsg := "first success"

	s.Descendants = []state.Descendant[client.Object]{
		{
			ID:           state.GetID("Secret", "first"),
			Object:       newSecret("first"),
			ErrorMessage: &firstErr,
		},
		{
			ID:             state.GetID("Secret", "second"),
			Object:         newSecret("second"),
			SuccessMessage: &successMsg,
		},
		{
			ID:           state.GetID("Secret", "third"),
			Object:       newSecret("third"),
			ErrorMessage: &secondErr,
		},
	}

	assert.Equal(t, []string{"first error", "second error"}, s.GetErrors())
}

func TestSetSaneDefaults_PreservesExplicitPaths(t *testing.T) {
	autoLoginConfig := state.AutoLoginConfig{}

	callback := "/custom-callback"
	logout := "/custom-logout"

	autoLoginConfig.SetSaneDefaults(ztoperatorv1alpha1.AutoLogin{
		RedirectPath: helperfunctions.Ptr(callback),
		LogoutPath:   helperfunctions.Ptr(logout),
	})

	assert.Equal(t, callback, autoLoginConfig.RedirectPath)
	assert.Equal(t, logout, autoLoginConfig.LogoutPath)
}

func newSecret(name string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
	}
}
