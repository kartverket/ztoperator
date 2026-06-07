package labels

const (
	ManagedByLabelKey  = "app.kubernetes.io/managed-by"
	ControllerLabelKey = "ztoperator.kartverket.no/controller"
	TypeLabelKey       = "type"

	ManagedByLabelValue            = "ztoperator"
	AuthPolicyControllerLabelValue = "authpolicy"
	TypeLabelValue                 = "ztoperator.kartverket.no"
)

// AuthPolicyStandardLabels returns the set of labels applied to every resource created by Ztoperator for the AuthPolicy
// controller.
func AuthPolicyStandardLabels() map[string]string {
	return map[string]string{
		ManagedByLabelKey:  ManagedByLabelValue,
		ControllerLabelKey: AuthPolicyControllerLabelValue,
		TypeLabelKey:       TypeLabelValue,
	}
}
