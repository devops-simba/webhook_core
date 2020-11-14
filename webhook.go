package webhook_core

import (
	"net/http"

	admissionApi "k8s.io/api/admission/v1"
)

// Webhook This interface represent a webhook
type Webhook interface {
	// Name name of this webhook
	Name() string
	// Handler that will be used to process HTTP requests that sent to this plugin
	HandleAdmission(
		request *http.Request,
		ar *admissionApi.AdmissionReview) (*admissionApi.AdmissionResponse, error)
}
