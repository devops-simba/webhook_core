package webhook_core

import (
	"github.com/devops-simba/helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func IsMatch(object *metav1.ObjectMeta, selector *metav1.LabelSelector) bool {
	if len(selector.MatchLabels) != 0 {
		if len(object.Labels) < len(selector.MatchLabels) {
			return false
		}

		for key, value := range selector.MatchLabels {
			ovalue, ok := object.Labels[key]
			if !ok || ovalue != value {
				return false
			}
		}
	}

	if len(selector.MatchExpressions) != 0 {
		for _, expr := range selector.MatchExpressions {
			ovalue, ok := object.Labels[expr.Key]
			switch expr.Operator {
			case metav1.LabelSelectorOpIn:
				if !ok || !helpers.ContainsString(expr.Values, ovalue) {
					return false
				}
			case metav1.LabelSelectorOpNotIn:
				if ok && helpers.ContainsString(expr.Values, ovalue) {
					return false
				}
			case metav1.LabelSelectorOpExists:
				if !ok {
					return false
				}
			case metav1.LabelSelectorOpDoesNotExist:
				if ok {
					return false
				}
			}
		}
	}

	return true
}
