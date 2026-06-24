package reconciliation_test

import (
	"testing"

	"github.com/kartverket/ztoperator/pkg/reconciliation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReconciliation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reconciliation Suite")
}

var _ = Describe("DetermineReconcileAction", func() {
	// Build a current/desired pair of *corev1.ConfigMap to use across tests.
	// The concrete type only matters insofar as T satisfies client.Object;
	// ConfigMap is the simplest such type available here.
	makeConfigMap := func(name, ns string) *corev1.ConfigMap {
		return &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		}
	}

	// shouldUpdateAlways and shouldUpdateNever are stub predicates used to
	// pin the should-update branch independently of object equality logic.
	shouldUpdateAlways := func(_, _ *corev1.ConfigMap) bool { return true }
	shouldUpdateNever := func(_, _ *corev1.ConfigMap) bool { return false }

	Context("when isDesiredNil is true", func() {
		It("returns RequiresDeleteAction when the resource exists and is owned by the AuthPolicy", func() {
			action, err := reconciliation.DetermineReconcileAction[*corev1.ConfigMap](
				makeConfigMap("foo", "ns"),
				nil,
				true,
				shouldUpdateNever,
				true,
				true,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(action).NotTo(BeNil())
			Expect(*action).To(Equal(reconciliation.RequiresDeleteAction))
		})

		It("returns RequiresNoAction when the resource exists but is NOT owned by the AuthPolicy", func() {
			action, err := reconciliation.DetermineReconcileAction[*corev1.ConfigMap](
				makeConfigMap("foo", "ns"),
				nil,
				true,
				shouldUpdateNever,
				true,
				false,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(action).NotTo(BeNil())
			Expect(*action).To(Equal(reconciliation.RequiresNoAction))
		})

		It("returns RequiresNoAction when the resource does not exist", func() {
			action, err := reconciliation.DetermineReconcileAction[*corev1.ConfigMap](
				makeConfigMap("foo", "ns"),
				nil,
				true,
				shouldUpdateNever,
				false,
				false,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(action).NotTo(BeNil())
			Expect(*action).To(Equal(reconciliation.RequiresNoAction))
		})
	})

	Context("when isDesiredNil is false", func() {
		desired := makeConfigMap("foo", "ns")

		It("returns RequiresCreateAction when the resource does not exist", func() {
			action, err := reconciliation.DetermineReconcileAction[*corev1.ConfigMap](
				makeConfigMap("foo", "ns"),
				&desired,
				false,
				shouldUpdateNever,
				false,
				false,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(action).NotTo(BeNil())
			Expect(*action).To(Equal(reconciliation.RequiresCreateAction))
		})

		It("returns an error when the resource exists but is not owned by the AuthPolicy", func() {
			action, err := reconciliation.DetermineReconcileAction[*corev1.ConfigMap](
				makeConfigMap("foo", "ns"),
				&desired,
				false,
				shouldUpdateNever,
				true,
				false,
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("cannot update ns/foo as it is not owned by AuthPolicy"))
			Expect(action).To(BeNil())
		})

		It("returns RequiresUpdateAction when the resource exists, is owned, and shouldUpdate returns true", func() {
			action, err := reconciliation.DetermineReconcileAction[*corev1.ConfigMap](
				makeConfigMap("foo", "ns"),
				&desired,
				false,
				shouldUpdateAlways,
				true,
				true,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(action).NotTo(BeNil())
			Expect(*action).To(Equal(reconciliation.RequiresUpdateAction))
		})

		It("returns RequiresNoAction when the resource exists, is owned, and shouldUpdate returns false", func() {
			action, err := reconciliation.DetermineReconcileAction[*corev1.ConfigMap](
				makeConfigMap("foo", "ns"),
				&desired,
				false,
				shouldUpdateNever,
				true,
				true,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(action).NotTo(BeNil())
			Expect(*action).To(Equal(reconciliation.RequiresNoAction))
		})

		It("passes current and *desired to shouldUpdate verbatim", func() {
			current := makeConfigMap("current-name", "ns")
			desiredCM := makeConfigMap("desired-name", "ns")
			desiredPtr := &desiredCM

			var gotCurrent, gotDesired *corev1.ConfigMap
			shouldUpdateSpy := func(c, d *corev1.ConfigMap) bool {
				gotCurrent = c
				gotDesired = d
				return true
			}

			_, err := reconciliation.DetermineReconcileAction[*corev1.ConfigMap](
				current,
				desiredPtr,
				false,
				shouldUpdateSpy,
				true, // currentExists
				true, // currentIsOwnedByAuthPolicy
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(gotCurrent).To(BeIdenticalTo(current))
			Expect(gotDesired).To(BeIdenticalTo(desiredCM))
		})
	})
})
