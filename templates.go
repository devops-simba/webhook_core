//go:generate go run templates/template_gen.go -package "webhook_core" -parser-fn "ParseTemplate"

package webhook_core

import (
	"sync"
	"text/template"

	"github.com/devops-simba/helpers"
	admissionRegistration "k8s.io/api/admissionregistration/v1"
)

var (
	initializeTemplateFuncs = &sync.Once{}
)

type DockerfileData struct {
	BuildProxy      string
	Port            int
	LogLevel        int
	Insecure        bool
	CertificateFile string
	PrivateKeyFile  string
}

type WebhookData struct {
	Name                       string
	SideEffects                string
	SupportedAdmissionVersions []string
	TimeoutInSeconds           int
	Configurations             []WebhookConfiguration
	Rules                      []admissionRegistration.RuleWithOperations
}

type DeploymentData struct {
	Name               string
	Namespace          string
	RunAsUser          int
	LogLevel           int
	ImageRegistry      string
	ImageName          string
	ImageTag           string
	ContainerPort      int
	ServerPort         int
	Insecure           bool
	CABundle           string
	TlsSecretName      string
	ServiceName        string
	ServiceUser        string
	MutatingWebhooks   []WebhookData
	ValidatingWebhooks []WebhookData
}

func (this DeploymentData) AllHooks() []WebhookData {
	result := make([]WebhookData, 0, len(this.MutatingWebhooks)+len(this.ValidatingWebhooks))
	result = append(result, this.MutatingWebhooks...)
	result = append(result, this.ValidatingWebhooks...)
	return result
}

type DeployScriptData struct {
	Insecure         bool
	Namespace        string
	TlsSecretName    string
	CertificateFile  string
	PrivateKeyFile   string
	DeploymentFolder string
	ImageRegistry    string
	ImageName        string
	ImageTag         string
	Kubectl          string
}

func ParseTemplate(name, body string) (*template.Template, error) {
	initializeTemplateFuncs.Do(func() {
		helpers.RegisterTemplateFunc("JoinOperations",
			func(operations []admissionRegistration.OperationType, sep string) (string, error) {
				result := ""
				first := true
				for _, operation := range operations {
					if first {
						first = false
					} else {
						result += sep
					}
					s, err := helpers.THF_Quote(string(operation))
					if err != nil {
						return "", err
					}
					result += s
				}
				return result, nil
			})
	})

	return helpers.ParseTemplate(name, body)
}
