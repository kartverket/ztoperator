package reconciler_test

import (
	"bytes"
	"context"
	"fmt"
	"time"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/reconciler"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/reconciliation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("ControllerResourceAdapter", func() {
	var (
		testNamespace string
		scope         *state.Scope
	)

	// secretShouldUpdate / secretUpdateFields mimic the production callbacks closely enough to exercise the
	// reconciliation engine: a Secret needs updating when its data differs from desired.
	secretShouldUpdate := func(current, desired *corev1.Secret) bool {
		return !bytes.Equal(current.Data["token"], desired.Data["token"])
	}
	secretUpdateFields := func(current, desired *corev1.Secret) {
		current.Data = desired.Data
	}

	newSecret := func(name string, token string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNamespace,
			},
			Data: map[string][]byte{"token": []byte(token)},
			Type: corev1.SecretTypeOpaque,
		}
	}

	BeforeEach(func() {
		testNamespace = fmt.Sprintf("test-reconciler-%d", time.Now().UnixNano())

		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())

		// A minimal scope whose AuthPolicy acts as the owner of reconciled resources.
		scope = &state.Scope{
			AuthPolicy: ztoperatorv1alpha1.AuthPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-authpolicy",
					Namespace: testNamespace,
					UID:       "test-uid-12345",
				},
			},
		}
	})

	AfterEach(func() {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		_ = k8sClient.Delete(ctx, ns)
	})

	Describe("GetResourceKind", func() {
		It("should return the correct resource kind", func() {
			adapter := reconciler.ControllerResourceAdapter[*corev1.Secret]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*corev1.Secret]{
					Func: reconciliation.ResourceReconciler[*corev1.Secret]{
						ResourceKind: "Secret",
						ResourceName: "test-secret",
					},
				},
			}

			Expect(adapter.GetResourceKind()).To(Equal("Secret"))
		})
	})

	Describe("GetResourceName", func() {
		It("should return the correct resource name", func() {
			adapter := reconciler.ControllerResourceAdapter[*corev1.Secret]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*corev1.Secret]{
					Func: reconciliation.ResourceReconciler[*corev1.Secret]{
						ResourceKind: "Secret",
						ResourceName: "my-test-secret",
					},
				},
			}

			Expect(adapter.GetResourceName()).To(Equal("my-test-secret"))
		})
	})

	Describe("IsResourceNil", func() {
		It("should return true when DesiredResource is nil", func() {
			adapter := reconciler.ControllerResourceAdapter[*corev1.Secret]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*corev1.Secret]{
					Func: reconciliation.ResourceReconciler[*corev1.Secret]{
						ResourceKind:    "Secret",
						ResourceName:    "test-secret",
						DesiredResource: nil,
					},
				},
			}

			Expect(adapter.IsResourceNil()).To(BeTrue())
		})

		It("should return true when DesiredResource points to nil", func() {
			var nilSecret *corev1.Secret
			adapter := reconciler.ControllerResourceAdapter[*corev1.Secret]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*corev1.Secret]{
					Func: reconciliation.ResourceReconciler[*corev1.Secret]{
						ResourceKind:    "Secret",
						ResourceName:    "test-secret",
						DesiredResource: &nilSecret,
					},
				},
			}

			Expect(adapter.IsResourceNil()).To(BeTrue())
		})

		It("should return false when DesiredResource is not nil", func() {
			secret := newSecret("test-secret", "token")
			adapter := reconciler.ControllerResourceAdapter[*corev1.Secret]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*corev1.Secret]{
					Func: reconciliation.ResourceReconciler[*corev1.Secret]{
						ResourceKind:    "Secret",
						ResourceName:    "test-secret",
						DesiredResource: &secret,
					},
				},
			}

			Expect(adapter.IsResourceNil()).To(BeFalse())
		})
	})

	Describe("Reconcile", func() {
		newAdapter := func(name string, desired *corev1.Secret) reconciler.ControllerResourceAdapter[*corev1.Secret] {
			return reconciler.ControllerResourceAdapter[*corev1.Secret]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*corev1.Secret]{
					Func: reconciliation.ResourceReconciler[*corev1.Secret]{
						ResourceKind:    "Secret",
						ResourceName:    name,
						DesiredResource: &desired,
						Scope:           scope,
						ShouldUpdate:    secretShouldUpdate,
						UpdateFields:    secretUpdateFields,
					},
				},
			}
		}

		It("should create a resource when it does not exist", func() {
			desired := newSecret("test-secret-create", "first-token")
			adapter := newAdapter(desired.Name, desired)

			result, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			createdSecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      desired.Name,
				Namespace: testNamespace,
			}, createdSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdSecret.Data["token"]).To(Equal([]byte("first-token")))
			Expect(metav1.IsControlledBy(createdSecret, &scope.AuthPolicy)).To(BeTrue())
		})

		It("should NOT update a resource when it exists and needs updating, but is not owned by AuthPolicy", func() {
			secretName := "test-secret-update"
			existing := newSecret(secretName, "old-token")
			Expect(k8sClient.Create(ctx, existing)).To(Succeed())

			desired := newSecret(secretName, "new-token")
			adapter := newAdapter(secretName, desired)

			result, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot update"))
			Expect(err.Error()).To(ContainSubstring("as it is not owned by AuthPolicy"))
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))

			// Verify the resource was left untouched.
			updated := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      secretName,
				Namespace: testNamespace,
			}, updated)).To(Succeed())
			Expect(updated.Data["token"]).To(Equal([]byte("old-token")))
		})

		It("should update a resource when it exists, is owned by AuthPolicy and needs updating", func() {
			secretName := "test-secret-update"
			existing := newSecret(secretName, "old-token")
			Expect(ctrl.SetControllerReference(&scope.AuthPolicy, existing, scheme.Scheme)).To(Succeed())
			Expect(k8sClient.Create(ctx, existing)).To(Succeed())

			desired := newSecret(secretName, "new-token")
			adapter := newAdapter(secretName, desired)

			result, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			updated := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      secretName,
				Namespace: testNamespace,
			}, updated)).To(Succeed())
			Expect(updated.Data["token"]).To(Equal([]byte("new-token")))
		})

		It("should not update a resource when no changes are needed", func() {
			secretName := "test-secret-noupdate"
			existing := newSecret(secretName, "same-token")
			Expect(ctrl.SetControllerReference(&scope.AuthPolicy, existing, scheme.Scheme)).To(Succeed())
			Expect(k8sClient.Create(ctx, existing)).To(Succeed())

			var beforeSecret corev1.Secret
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      secretName,
				Namespace: testNamespace,
			}, &beforeSecret)).To(Succeed())
			resourceVersionBefore := beforeSecret.ResourceVersion

			desired := newSecret(secretName, "same-token")
			adapter := newAdapter(secretName, desired)

			result, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			var afterSecret corev1.Secret
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      secretName,
				Namespace: testNamespace,
			}, &afterSecret)).To(Succeed())
			Expect(afterSecret.ResourceVersion).To(Equal(resourceVersionBefore))
		})

		It("should NOT delete a resource when desired is nil, the resource exists, but the resource is NOT owned by AuthPolicy", func() {
			secretName := "test-secret-delete"
			existing := newSecret(secretName, "to-be-deleted")
			Expect(k8sClient.Create(ctx, existing)).To(Succeed())

			var nilSecret *corev1.Secret
			adapter := newAdapter(secretName, nilSecret)

			result, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			Eventually(func() bool {
				notDeleted := &corev1.Secret{}
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      secretName,
					Namespace: testNamespace,
				}, notDeleted)
				return err == nil
			}).Should(BeTrue())
		})

		It("should delete a resource when desired is nil, the resource exists, and the resource is owned by AuthPolicy", func() {
			secretName := "test-secret-delete"
			existing := newSecret(secretName, "to-be-deleted")
			Expect(ctrl.SetControllerReference(&scope.AuthPolicy, existing, scheme.Scheme)).To(Succeed())
			Expect(k8sClient.Create(ctx, existing)).To(Succeed())

			var nilSecret *corev1.Secret
			adapter := newAdapter(secretName, nilSecret)

			result, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			Eventually(func() bool {
				deleted := &corev1.Secret{}
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      secretName,
					Namespace: testNamespace,
				}, deleted)
				return err != nil && errors.IsNotFound(err)
			}).Should(BeTrue())
		})

		It("should handle reconcile when desired is nil and resource does not exist", func() {
			var nilSecret *corev1.Secret
			adapter := newAdapter("non-existent-secret", nilSecret)

			result, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())
		})
	})
})
