package webhook_core

import (
	"net/http"

	admissionApi "k8s.io/api/admission/v1"
	admissionRegistration "k8s.io/api/admissionregistration/v1"
)

type AdmissionWebhookType string

const (
	DefaultTimeoutInSeconds                         = 5
	MutatingAdmissionWebhook   AdmissionWebhookType = "mutating"
	ValidatingAdmissionWebhook AdmissionWebhookType = "validating"
)

var (
	SupportedAdmissionVersions = []string{"v1", "v1beta1"}
)

type WebhookConfiguration struct {
	Name         string
	Desc         string
	DefaultValue *string
}

func CreateConfig(name, defaultValue, desc string) WebhookConfiguration {
	return WebhookConfiguration{
		Name:         name,
		DefaultValue: &defaultValue,
		Desc:         desc,
	}
}

// Webhook This interface represent a webhook
type AdmissionWebhook interface {
	// Name name of this webhook
	Name() string
	// Type type of this webhook
	Type() AdmissionWebhookType
	// Rules rules that will be applied to this
	Rules() []admissionRegistration.RuleWithOperations
	// Configurations of this webhook
	Configurations() []WebhookConfiguration
	// TimeoutInSeconds timeout of this webhook
	TimeoutInSeconds() int
	// SupportedAdmissionVersions admission versions that supported by this webhook
	SupportedAdmissionVersions() []string
	// SideEffects side effects of running this webhook
	SideEffects() admissionRegistration.SideEffectClass
	// Initialize added an opportunity to initialize before actual running
	Initialize()
	// Handler that will be used to process HTTP requests that sent to this plugin
	HandleAdmission(
		request *http.Request,
		ar *admissionApi.AdmissionReview,
	) (*admissionApi.AdmissionResponse, error)
}
