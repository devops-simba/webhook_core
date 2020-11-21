package webhook_core

import (
	"net/http"

	admissionApi "k8s.io/api/admission/v1"
	admissionRegistration "k8s.io/api/admissionregistration/v1"
)

type AdmissionWebhookType string

const (
	MutatingAdmissionWebhook   AdmissionWebhookType = "mutating"
	ValidatingAdmissionWebhook AdmissionWebhookType = "validating"
)

// Webhook This interface represent a webhook
type AdmissionWebhook interface {
	// Name name of this webhook
	Name() string
	// Type type of this webhook
	Type() AdmissionWebhookType
	// Rules rules that will be applied to this
	Rules() []admissionRegistration.RuleWithOperations
	// Handler that will be used to process HTTP requests that sent to this plugin
	HandleAdmission(
		request *http.Request,
		ar *admissionApi.AdmissionReview,
	) (*admissionApi.AdmissionResponse, error)
}
