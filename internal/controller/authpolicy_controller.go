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
	"fmt"
	v3 "istio.io/api/security/v1"
	"istio.io/api/security/v1beta1"
	v1beta2 "istio.io/api/type/v1beta1"
	v1 "istio.io/client-go/pkg/apis/security/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"reflect"
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

	log.Info(fmt.Sprintf("Received reconcile request for AuthPolicy with name %s", req.NamespacedName.String()))

	if err := r.Client.Get(ctx, req.NamespacedName, authPolicy); err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, fmt.Sprintf("AuthPolicy with name %s not found", req.NamespacedName.String()))
			return reconcile.Result{}, nil
		}
		log.Error(err, fmt.Sprintf("Failed to get AuthPolicy with name %s", req.NamespacedName.String()))
		return reconcile.Result{}, err
	}

	log.Info(fmt.Sprintf("AuthPolicy with name %s found", req.NamespacedName.String()))

	// TODO: Finalize object if it's being deleted
	// TODO: Add finalizers + r.Client.Update() if missing

	originalAuthPolicy := authPolicy.DeepCopy()
	authPolicy.InitializeConditions()

	s := &scope{authPolicy: authPolicy}

	defer func() {
		r.updateStatus(ctx, s, originalAuthPolicy)
	}()

	reconcileFuncs := []authPolicyReconcileFunc{
		r.reconcileRequestAuthentication,
		r.reconcileIgnoreAuthAuthorizationPolicy,
		r.reconcileRequireAuthAuthorizationPolicy,
	}

	if !authPolicy.DeletionTimestamp.IsZero() {
		log.Info(fmt.Sprintf("Deleting AuthPolicy with name %s", req.NamespacedName.String()))
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

func (r *AuthPolicyReconciler) updateStatus(ctx context.Context, s *scope, original *ztoperatorv1alpha1.AuthPolicy) {
	ap := s.authPolicy
	log := ctrl.LoggerFrom(ctx)
	log.Info(fmt.Sprintf("Updating AuthPolicy status for %s/%s", ap.Namespace, ap.Name))

	ap.Status = ztoperatorv1alpha1.AuthPolicyStatus{
		ObservedGeneration: ap.GetGeneration(),
		Conditions: []v2.Condition{
			{
				Type:               string(ztoperatorv1alpha1.PhaseReady),
				Status:             v2.ConditionTrue,
				LastTransitionTime: v2.Now(),
				Reason:             string(ztoperatorv1alpha1.PhaseReady),
				Message:            "AuthPolicy ready",
			},
		},
		Ready:   true,
		Phase:   ztoperatorv1alpha1.PhaseReady,
		Message: "AuthPolicy ready",
	}
	if !s.IsDescendantsSet() {
		log.Info(fmt.Sprintf("Descendants of AuthPolicy with name %s/%s not set", ap.Namespace, ap.Name))
		ap.Status = ztoperatorv1alpha1.AuthPolicyStatus{
			Ready:   false,
			Phase:   ztoperatorv1alpha1.PhasePending,
			Message: "AuthPolicy pending",
		}
		log.Info(fmt.Sprintf("Updating status condition of AuthPolicy with name %s/%s: %s", ap.Namespace, ap.Name, string(ztoperatorv1alpha1.PhasePending)))
		meta.SetStatusCondition(&ap.Status.Conditions, v2.Condition{
			Type:               string(ztoperatorv1alpha1.PhasePending),
			Status:             v2.ConditionFalse,
			LastTransitionTime: v2.Now(),
			Reason:             string(ztoperatorv1alpha1.PhasePending),
			Message:            "AuthPolicy pending",
		})
	}

	if !equality.Semantic.DeepEqual(original.Status, ap.Status) {
		log.Info(fmt.Sprintf("Patching AuthPolicy status with name %s/%s", ap.Namespace, ap.Name))
		if err := r.Status().Patch(ctx, ap, client.MergeFrom(original)); err != nil {
			log.Error(err, fmt.Sprintf("Failed to patch AuthPolicy status with name %s/%s", ap.Namespace, ap.Name))
		}
	}
}

func (r *AuthPolicyReconciler) reconcileRequestAuthentication(ctx context.Context, scope *scope) (ctrl.Result, error) {
	authPolicy := scope.authPolicy

	desired := &v1.RequestAuthentication{
		ObjectMeta: buildObjectMeta(authPolicy.Name, authPolicy.Namespace),
		Spec: v1beta1.RequestAuthentication{
			Selector: &v1beta2.WorkloadSelector{MatchLabels: authPolicy.Spec.Selector.MatchLabels},
			JwtRules: authPolicy.Spec.JWTRules.ToIstioRequestAuthenticationJWTRules(),
		},
	}

	return reconcileAuthPolicy[*v1.RequestAuthentication](
		ctx,
		r.Client,
		r.Scheme,
		authPolicy,
		desired,
		func(obj *v1.RequestAuthentication) {
			scope.descendants.requestAuthentication = obj
		},
		func(current, desired *v1.RequestAuthentication) bool {
			return !reflect.DeepEqual(current.Spec.Selector, desired.Spec.Selector) ||
				!reflect.DeepEqual(current.Spec.JwtRules, desired.Spec.JwtRules)
		},
		func(current, desired *v1.RequestAuthentication) {
			current.Spec.Selector = desired.Spec.Selector
			current.Spec.JwtRules = desired.Spec.JwtRules
		},
	)
}

func (r *AuthPolicyReconciler) reconcileIgnoreAuthAuthorizationPolicy(ctx context.Context, scope *scope) (ctrl.Result, error) {
	authPolicy := scope.authPolicy

	desired := &v1.AuthorizationPolicy{
		ObjectMeta: buildObjectMeta(authPolicy.Name+"-allow", authPolicy.Namespace),
		Spec: v1beta1.AuthorizationPolicy{
			Selector: &v1beta2.WorkloadSelector{MatchLabels: authPolicy.Spec.Selector.MatchLabels},
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

	return reconcileAuthorizationPolicy(ctx, r.Client, r.Scheme, authPolicy, desired, func(obj *v1.AuthorizationPolicy) {
		scope.descendants.ignoreAuthAuthorizationPolicy = obj
	})
}

func (r *AuthPolicyReconciler) reconcileRequireAuthAuthorizationPolicy(ctx context.Context, scope *scope) (ctrl.Result, error) {
	authPolicy := scope.authPolicy

	desired := &v1.AuthorizationPolicy{
		ObjectMeta: buildObjectMeta(authPolicy.Name+"-auth", authPolicy.Namespace),
		Spec: v1beta1.AuthorizationPolicy{
			Selector: &v1beta2.WorkloadSelector{MatchLabels: authPolicy.Spec.Selector.MatchLabels},
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

	return reconcileAuthorizationPolicy(ctx, r.Client, r.Scheme, authPolicy, desired, func(obj *v1.AuthorizationPolicy) {
		scope.descendants.requireAuthAuthorizationPolicy = obj
	})
}

func (r *AuthPolicyReconciler) reconcileDelete(ctx context.Context, scope *scope) (ctrl.Result, error) {
	authPolicy := scope.authPolicy

	resources := []client.Object{
		&v1.RequestAuthentication{
			ObjectMeta: buildObjectMeta(authPolicy.Name, authPolicy.Namespace),
		},
		&v1.AuthorizationPolicy{
			ObjectMeta: buildObjectMeta(authPolicy.Name, authPolicy.Namespace),
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

func reconcileAuthPolicy[T client.Object](
	ctx context.Context,
	k8sClient client.Client,
	scheme *runtime.Scheme,
	authPolicy *ztoperatorv1alpha1.AuthPolicy,
	desired T,
	assign func(T),
	shouldUpdate func(current, desired T) bool,
	updateFields func(current, desired T),
) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	kind := reflect.TypeOf(desired).Elem().Name()
	current := reflect.New(reflect.TypeOf(desired).Elem()).Interface().(T)
	log.Info(fmt.Sprintf("Generating %s with name %s", kind, desired.GetName()))
	log.Info(fmt.Sprintf("Checking if %s with name %s exists", kind, desired.GetName()))
	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(desired), current)
	if apierrors.IsNotFound(err) {
		log.Info(fmt.Sprintf("%s with name %s does not exist", kind, desired.GetName()))
		if err := ctrl.SetControllerReference(authPolicy, desired, scheme); err != nil {
			return ctrl.Result{}, err
		}
		log.Info(fmt.Sprintf("Creating a %s with name %s", kind, desired.GetName()))
		if err := k8sClient.Create(ctx, desired); err != nil {
			return ctrl.Result{}, err
		}
		assign(desired)
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	log.Info(fmt.Sprintf("%s with name %s exists", kind, desired.GetName()))
	log.Info(fmt.Sprintf("Determing if %s with name %s should be updated", kind, desired.GetName()))
	if shouldUpdate(current, desired) {
		log.Info(fmt.Sprintf("Current %s with name %s != desired", kind, desired.GetName()))
		log.Info(fmt.Sprintf("Updating current %s with name %s with desired", kind, desired.GetName()))
		updateFields(current, desired)
		if err := k8sClient.Update(ctx, current); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		log.Info(fmt.Sprintf("Current %s with name %s == desired. No update needed.", kind, desired.GetName()))
	}

	assign(current)

	return ctrl.Result{}, nil
}

func reconcileAuthorizationPolicy(
	ctx context.Context,
	k8sClient client.Client,
	scheme *runtime.Scheme,
	authPolicy *ztoperatorv1alpha1.AuthPolicy,
	desired *v1.AuthorizationPolicy,
	assign func(*v1.AuthorizationPolicy),
) (ctrl.Result, error) {
	return reconcileAuthPolicy[*v1.AuthorizationPolicy](
		ctx,
		k8sClient,
		scheme,
		authPolicy,
		desired,
		assign,
		func(current, desired *v1.AuthorizationPolicy) bool {
			return !reflect.DeepEqual(current.Spec.Selector, desired.Spec.Selector) ||
				!reflect.DeepEqual(current.Spec.Rules, desired.Spec.Rules)
		},
		func(current, desired *v1.AuthorizationPolicy) {
			current.Spec.Selector = desired.Spec.Selector
			current.Spec.Rules = desired.Spec.Rules
		},
	)
}

func buildObjectMeta(name, namespace string) v2.ObjectMeta {
	return v2.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    map[string]string{"type": "ztoperator.kartverket.no"},
	}
}

func (s *scope) IsDescendantsSet() bool {
	return s != nil &&
		s.descendants.requestAuthentication != nil &&
		s.descendants.ignoreAuthAuthorizationPolicy != nil &&
		s.descendants.requireAuthAuthorizationPolicy != nil

}
