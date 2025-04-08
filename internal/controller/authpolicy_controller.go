/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"github.com/gogo/protobuf/proto"
	v3 "istio.io/api/security/v1"
	"istio.io/api/security/v1beta1"
	v1 "istio.io/client-go/pkg/apis/security/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
)

// AuthPolicyReconciler reconciles a AuthPolicy object
type AuthPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *AuthPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ztoperatorv1alpha1.AuthPolicy{}).
		Owns(&v1.RequestAuthentication{}).
		Owns(&v1.AuthorizationPolicy{}).
		Complete(r)
}

type authPolicyReconcileFunc func(context.Context, *scope) (ctrl.Result, error)

type scope struct {
	authPolicy  *ztoperatorv1alpha1.AuthPolicy
	descendants authPolicyDescendants
}

type authPolicyDescendants struct {
	requestAuthentication          *v1.RequestAuthentication
	ignoreAuthAuthorizationPolicy  *v1.AuthorizationPolicy
	requireAuthAuthorizationPolicy *v1.AuthorizationPolicy
}

// +kubebuilder:rbac:groups=ztoperator.kartverket.no,resources=authpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ztoperator.kartverket.no,resources=authpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ztoperator.kartverket.no,resources=authpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=security.istio.io,resources=authorizationpolicies;requestauthentications,verbs=get;list;watch;create;update;patch;delete

func (r *AuthPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	authPolicy := new(ztoperatorv1alpha1.AuthPolicy)

	if err := r.Client.Get(ctx, req.NamespacedName, authPolicy); err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "AuthPolicy with name %s not found", req.NamespacedName.String())
			return reconcile.Result{}, nil
		}
		log.Error(err, "Failed to get AuthPolicy with name %s", req.NamespacedName.String())
		return reconcile.Result{}, err
	}

	// TODO: Finalize object if it's being deleted
	// TODO: Add finalizers + r.Client.Update() if missing

	originalAuthPolicy := authPolicy.DeepCopy()
	authPolicy.InitializeConditions()

	s := &scope{authPolicy: authPolicy}

	defer func() {
		r.updateStatus(ctx, s)
	}()

	// TODO: 5. if (originalAuthPolicy.Status != authPolicy.Status): client.Status.Patch(obj)
	if !equality.Semantic.DeepEqual(originalAuthPolicy.Status, authPolicy.Status) {
		if err := r.Status().Patch(ctx, authPolicy, client.MergeFrom(originalAuthPolicy)); err != nil {
			log.Error(err, "Failed to patch AuthPolicy status with name %s", req.NamespacedName.String())
			return ctrl.Result{}, err
		}
	}

	reconcileFuncs := []authPolicyReconcileFunc{
		r.reconcileRequestAuthentication,
		r.reconcileIgnoreAuthAuthorizationPolicy,
		r.reconcileRequireAuthAuthorizationPolicy,
	}

	if !authPolicy.DeletionTimestamp.IsZero() {
		reconcileFuncs = append(reconcileFuncs, r.reconcileDelete)
	}

	return doReconcile(ctx, reconcileFuncs, s)
}

func doReconcile(ctx context.Context, reconcileFuncs []authPolicyReconcileFunc, s *scope) (ctrl.Result, error) {
	result := ctrl.Result{}
	var errs []error
	for _, reconcileFunc := range reconcileFuncs {
		reconcileResult, err := reconcileFunc(ctx, s)
		if err != nil {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			continue
		}
		result = lowestNonZeroResult(result, reconcileResult)
	}

	if len(errs) > 0 {
		return ctrl.Result{}, errors.NewAggregate(errs)
	}
	return result, nil
}

func lowestNonZeroResult(i, j ctrl.Result) ctrl.Result {
	switch {
	case i.IsZero():
		return j
	case j.IsZero():
		return i
	case i.Requeue:
		return i
	case j.Requeue:
		return j
	case i.RequeueAfter < j.RequeueAfter:
		return i
	default:
		return j
	}
}

func (r *AuthPolicyReconciler) updateStatus(ctx context.Context, s *scope) {
	ap := s.authPolicy
	log := ctrl.LoggerFrom(ctx)

	ap.Status.ObservedGeneration = ap.GetGeneration()

	phase := ztoperatorv1alpha1.PhaseReady
	condition := v2.Condition{
		Type:               string(ztoperatorv1alpha1.PhaseReady),
		Status:             v2.ConditionTrue,
		LastTransitionTime: v2.Now(),
		Reason:             "ReconciliationSucceeded",
		Message:            "Descendants created or updated successfully",
	}

	if s.descendants.requestAuthentication.Name == "" || s.descendants.ignoreAuthAuthorizationPolicy.Name == "" || s.descendants.requireAuthAuthorizationPolicy.Name == "" {
		phase = ztoperatorv1alpha1.PhasePending
		condition = v2.Condition{
			Type:               string(ztoperatorv1alpha1.PhasePending),
			Status:             v2.ConditionFalse,
			LastTransitionTime: v2.Now(),
			Reason:             "MissingDescendants",
			Message:            "Descendants resources are not yet created",
		}
	}

	ap.Status.Phase = phase
	meta.SetStatusCondition(&ap.Status.Conditions, condition)

	if err := r.Status().Patch(ctx, ap, client.MergeFrom(ap)); err != nil {
		log.Error(err, "Failed to patch AuthPolicy status")
	}
}

func (r *AuthPolicyReconciler) reconcileRequestAuthentication(ctx context.Context, scope *scope) (ctrl.Result, error) {
	authPolicy := scope.authPolicy

	desired := &v1.RequestAuthentication{
		ObjectMeta: v2.ObjectMeta{
			Name:      authPolicy.Name,
			Namespace: authPolicy.Namespace,
			Labels:    map[string]string{"type": "ztoperator.kartverket.no"},
		},
		Spec: v1beta1.RequestAuthentication{
			Selector: &authPolicy.Spec.Selector,
			JwtRules: authPolicy.Spec.JWTRules.ToIstioRequestAuthenticationJWTRules(),
		},
	}

	current := new(v1.RequestAuthentication)
	err := r.Client.Get(ctx, client.ObjectKeyFromObject(desired), current)
	if apierrors.IsNotFound(err) {
		if err := ctrl.SetControllerReference(authPolicy, desired, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Client.Create(ctx, desired); err != nil {
			return ctrl.Result{}, err
		}
		scope.descendants.requestAuthentication = desired
		return ctrl.Result{}, nil
	}
	if !proto.Equal(&current.Spec, &desired.Spec) {
		current.Spec.Selector = desired.Spec.Selector
		current.Spec.JwtRules = desired.Spec.JwtRules
		if err := r.Client.Update(ctx, current); err != nil {
			return ctrl.Result{}, err
		}
	}

	scope.descendants.requestAuthentication = current
	return ctrl.Result{}, nil
}

func (r *AuthPolicyReconciler) reconcileIgnoreAuthAuthorizationPolicy(ctx context.Context, scope *scope) (ctrl.Result, error) {
	authPolicy := scope.authPolicy

	desired := &v1.AuthorizationPolicy{
		ObjectMeta: v2.ObjectMeta{
			Name:      authPolicy.Name,
			Namespace: authPolicy.Namespace,
			Labels:    map[string]string{"type": "ztoperator.kartverket.no"},
		},
		Spec: v1beta1.AuthorizationPolicy{
			Selector: &authPolicy.Spec.Selector,
			Rules: []*v3.Rule{
				{
					To: []*v1beta1.Rule_To{
						{
							Operation: &v1beta1.Operation{
								Paths: []string{"*"},
							},
						},
					},
				},
			},
		},
	}

	current := new(v1.AuthorizationPolicy)
	err := r.Client.Get(ctx, client.ObjectKeyFromObject(desired), current)
	if apierrors.IsNotFound(err) {
		if err := ctrl.SetControllerReference(authPolicy, desired, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Client.Create(ctx, desired); err != nil {
			return ctrl.Result{}, err
		}
		scope.descendants.ignoreAuthAuthorizationPolicy = desired
		return ctrl.Result{}, nil
	}
	if !proto.Equal(&current.Spec, &desired.Spec) {
		current.Spec.Selector = desired.Spec.Selector
		current.Spec.Rules = desired.Spec.Rules
		if err := r.Client.Update(ctx, current); err != nil {
			return ctrl.Result{}, err
		}
	}

	scope.descendants.ignoreAuthAuthorizationPolicy = current
	return ctrl.Result{}, nil
}

func (r *AuthPolicyReconciler) reconcileRequireAuthAuthorizationPolicy(ctx context.Context, scope *scope) (ctrl.Result, error) {
	authPolicy := scope.authPolicy

	desired := &v1.AuthorizationPolicy{
		ObjectMeta: v2.ObjectMeta{
			Name:      authPolicy.Name,
			Namespace: authPolicy.Namespace,
			Labels:    map[string]string{"type": "ztoperator.kartverket.no"},
		},
		Spec: v1beta1.AuthorizationPolicy{
			Selector: &authPolicy.Spec.Selector,
			Rules: []*v3.Rule{
				{
					To: []*v1beta1.Rule_To{
						{
							Operation: &v1beta1.Operation{
								Paths: []string{"*"},
							},
						},
					},
					When: []*v1beta1.Condition{
						{
							Key:    "request.auth.claims[iss]",
							Values: []string{"example-issuer"},
						},
					},
				},
			},
		},
	}

	current := new(v1.AuthorizationPolicy)
	err := r.Client.Get(ctx, client.ObjectKeyFromObject(desired), current)
	if apierrors.IsNotFound(err) {
		if err := ctrl.SetControllerReference(authPolicy, desired, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Client.Create(ctx, desired); err != nil {
			return ctrl.Result{}, err
		}
		scope.descendants.requireAuthAuthorizationPolicy = desired
		return ctrl.Result{}, nil
	}
	if !proto.Equal(&current.Spec, &desired.Spec) {
		current.Spec.Selector = desired.Spec.Selector
		current.Spec.Rules = desired.Spec.Rules
		if err := r.Client.Update(ctx, current); err != nil {
			return ctrl.Result{}, err
		}
	}

	scope.descendants.requireAuthAuthorizationPolicy = current
	return ctrl.Result{}, nil
}

func (r *AuthPolicyReconciler) reconcileDelete(ctx context.Context, scope *scope) (ctrl.Result, error) {
	authPolicy := scope.authPolicy

	resources := []client.Object{
		&v1.RequestAuthentication{
			ObjectMeta: v2.ObjectMeta{
				Name:      authPolicy.Name,
				Namespace: authPolicy.Namespace,
			},
		},
		&v1.AuthorizationPolicy{
			ObjectMeta: v2.ObjectMeta{
				Name:      authPolicy.Name,
				Namespace: authPolicy.Namespace,
			},
		},
	}

	var errs []error
	for _, resource := range resources {
		if err := r.Client.Delete(ctx, resource); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, err)
		}
	}

	// TODO: If we have finalizers, remove them (example: "ztoperator.kartverket.no/finalizer")
	if len(errs) > 0 {
		return ctrl.Result{}, errors.NewAggregate(errs)
	}
	return ctrl.Result{}, nil
}
