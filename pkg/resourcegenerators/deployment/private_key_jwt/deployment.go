package private_key_jwt

import (
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/kartverket/ztoperator/pkg/utilities"
	v2 "k8s.io/api/apps/v1"
	v3 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *v2.Deployment {
	if !scope.ShouldHaveTokenProxy() {
		return nil
	}

	return &v2.Deployment{
		ObjectMeta: objectMeta,
		Spec: v2.DeploymentSpec{
			Replicas: utilities.Ptr(int32(2)),
			Selector: &v1.LabelSelector{
				MatchLabels: map[string]string{"app": objectMeta.Name},
			},
			Template: v3.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{"app": objectMeta.Name},
					Annotations: map[string]string{
						"cluster-autoscaler.kubernetes.io/safe-to-evict": "true",
						"prometheus.io/scrape":                           "true",
						"prometheus.istio.io/merge-metrics":              "false",
						"sidecar.istio.io/inject":                        "false",
					},
				},
				Spec: v3.PodSpec{
					Containers: []v3.Container{
						{
							Name:  objectMeta.Name,
							Image: utilities.TokenProxyImageName + ":" + utilities.TokenProxyImageTag,
							ImagePullPolicy: func() v3.PullPolicy {
								if config.IsLocal {
									return v3.PullNever
								}
								return v3.PullAlways
							}(),
							Ports: []v3.ContainerPort{
								{
									Name:          "main",
									ContainerPort: utilities.TokenProxyPort,
									Protocol:      v3.ProtocolTCP,
								},
								{
									Name:          "istio-metrics",
									ContainerPort: utilities.IstioProxyPort,
									Protocol:      v3.ProtocolTCP,
								},
							},
							Env: []v3.EnvVar{
								{
									Name:  utilities.TokenProxyServerModeEnvVarName,
									Value: utilities.TokenProxyServerModeEnvVarValue,
								},
								{
									Name:  utilities.TokenProxyIssuerEnvVarName,
									Value: scope.IdentityProviderUris.IssuerURI,
								},
								{
									Name:  utilities.TokenProxyTokenEndpointEnvVarName,
									Value: scope.AutoLoginConfig.TokenProxy.TokenEndpointParsedAsUrl.String(),
								},
								{
									Name: utilities.TokenProxyPrivateJWKEnvVarName,
									ValueFrom: &v3.EnvVarSource{
										SecretKeyRef: &v3.SecretKeySelector{
											LocalObjectReference: v3.LocalObjectReference{
												Name: scope.AuthPolicy.Spec.OAuthCredentials.SecretRef,
											},
											Key:      scope.AuthPolicy.Spec.OAuthCredentials.PrivateJWKKey,
											Optional: utilities.Ptr(false),
										},
									},
								},
							},
							Resources: v3.ResourceRequirements{},
							SecurityContext: &v3.SecurityContext{
								AllowPrivilegeEscalation: utilities.Ptr(false),
								Capabilities: &v3.Capabilities{
									Add: []v3.Capability{
										"NET_BIND_SERVICE",
									},
									Drop: []v3.Capability{
										"ALL",
									},
								},
								Privileged:             utilities.Ptr(false),
								ReadOnlyRootFilesystem: utilities.Ptr(true),
								RunAsGroup:             utilities.Ptr(int64(150)),
								RunAsNonRoot:           utilities.Ptr(true),
								RunAsUser:              utilities.Ptr(int64(150)),
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: "File",
						},
					},
					DNSPolicy: "ClusterFirst",
					ImagePullSecrets: []v3.LocalObjectReference{
						{
							Name: "github-auth",
						},
					},
					PriorityClassName: "skip-medium",
					RestartPolicy:     v3.RestartPolicyAlways,
					SchedulerName:     "default-scheduler",
					SecurityContext: &v3.PodSecurityContext{
						FSGroup: utilities.Ptr(int64(150)),
						SeccompProfile: &v3.SeccompProfile{
							Type: v3.SeccompProfileTypeRuntimeDefault,
						},
						SupplementalGroups: []int64{
							int64(150),
						},
					},
					ServiceAccountName:            objectMeta.Name,
					TerminationGracePeriodSeconds: utilities.Ptr(int64(30)),
					TopologySpreadConstraints: []v3.TopologySpreadConstraint{
						{
							LabelSelector: &v1.LabelSelector{
								MatchExpressions: []v1.LabelSelectorRequirement{
									{
										Key:      "app",
										Operator: v1.LabelSelectorOpIn,
										Values:   []string{objectMeta.Name},
									},
								},
							},
							MatchLabelKeys:    []string{"pod-template-hash"},
							MaxSkew:           int32(1),
							TopologyKey:       "kubernetes.io/hostname",
							WhenUnsatisfiable: "ScheduleAnyway",
						},
						{
							LabelSelector: &v1.LabelSelector{
								MatchExpressions: []v1.LabelSelectorRequirement{
									{
										Key:      "app",
										Operator: v1.LabelSelectorOpIn,
										Values:   []string{objectMeta.Name},
									},
								},
							},
							MatchLabelKeys:    []string{"pod-template-hash"},
							MaxSkew:           int32(1),
							TopologyKey:       "onprem.gke.io/failure-domain-name",
							WhenUnsatisfiable: "ScheduleAnyway",
						},
					},
				},
			},
			Strategy: v2.DeploymentStrategy{
				Type: v2.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &v2.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
					MaxSurge:       &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
				},
			},
			RevisionHistoryLimit:    utilities.Ptr(int32(2)),
			ProgressDeadlineSeconds: utilities.Ptr(int32(600)),
		},
	}
}
