package utils

import (
	"fmt"
	"istio.io/istio/pkg/config/security"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"
)

func LowestNonZeroResult(i, j ctrl.Result) ctrl.Result {
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

func BuildObjectMeta(name, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    map[string]string{"type": "ztoperator.kartverket.no"},
	}
}

func Ptr[T any](v T) *T {
	return &v
}

func ValidatePaths(paths []string) error {
	for _, path := range paths {
		if strings.Contains(path, "{") || strings.Contains(path, "}") {
			if err := security.CheckValidPathTemplate("Paths", paths); err != nil {
				return err
			}
			continue
		}
		if strings.Count(path, "*") > 1 || (strings.Contains(path, "*") && !(path == "*" || strings.HasPrefix(path, "*") || strings.HasSuffix(path, "*"))) {
			return fmt.Errorf("invalid path: %s; '*' must appear only once, be at the start, end, or be '*'", path)
		}
	}
	return nil
}
