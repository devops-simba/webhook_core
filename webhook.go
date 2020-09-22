package webhook_core

import (
	"fmt"
	"net/http"
	"strings"

	admissionApi "k8s.io/api/admission/v1"
)

// Webhook This interface represent a webhook
type Webhook interface {
	// Initialize
	// this function will be called by main application after it parsed the flags to let
	// plugin load its configuration
	Initialize() error

	// Name name of this webhook
	Name() string
	// Description of this webhook, this is required for log and like of that
	Desc() string
	// Path that this webhook will be exposed on it(in HTTP server)
	// This path must start with '/' and must not end with it, otherwise
	Path() string
	// Handler that will be used to process HTTP requests that sent to this plugin
	// All requests that start in the path of this webhook will result in calling this function
	// Return `nil` as response if this review does not belong to this webhook
	HandleAdmission(
		action string,
		request *http.Request,
		ar *admissionApi.AdmissionReview) (*admissionApi.AdmissionResponse, error)
}

// GetWebhookAction extract web action from the path
func GetWebhookAction(path string, webhookPath string) (string, bool) {
	if strings.HasPrefix(path, "/"+webhookPath) {
		if len(path) == (1 + len(webhookPath)) {
			return "", true
		}

		if path[len(webhookPath)+1] == '/' {
			return path[len(webhookPath)+2:], true
		}
	}

	return "", false
}

// ToString return a string representation of a loaded webhook
func ToString(webhook Webhook, indent string) string {
	result := ""
	result += fmt.Sprintf("%vName:        %v\n", indent, webhook.Name())
	result += fmt.Sprintf("%vDescription: %v\n", indent, webhook.Desc())
	result += fmt.Sprintf("%vPath:        %v", indent, webhook.Path())
	return result
}
