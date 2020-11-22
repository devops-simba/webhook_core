package webhook_core

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"text/template"

	admissionRegistration "k8s.io/api/admissionregistration/v1"
)

var (
	funcs = template.FuncMap{
		"json":  json.Marshal,
		"join":  strings.Join,
		"deref": func(value *string) string { return *value },
		"quote": func(value string) string {
			return `"` + strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`) + `"`
		},
		"joinOperations": func(operations []admissionRegistration.OperationType, sep string) string {
			result := ""
			first := true
			for _, operation := range operations {
				if first {
					first = false
				} else {
					result += sep
				}
				result += string(operation)
			}
			return result
		},
		"joinScope": func(outer, inner interface{}) JoinedScope {
			return JoinedScope{
				Inner: inner,
				Outer: outer,
			}
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values) == 0 {
				return nil, errors.New("Invalid call to dict")
			}
			if (len(values) & 1) == 1 {
				return nil, errors.New("Invalid number of arguments for dict")
			}

			var result map[string]interface{}
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, errors.New("Keys must be string")
				}

				result[key] = values[i+1]
			}

			return result, nil
		},
		"backToRoot": func(path string) string {
			if path == "" {
				return "."
			}

			if strings.HasPrefix(path, "./") {
				path = path[2:]
			}

			parts := strings.Split(path, "/")
			if parts[len(parts)-1] == "" {
				parts = parts[:len(parts)-1]
			}
			if len(parts) == 1 {
				return ".."
			}

			for i := 0; i < len(parts); i++ {
				parts[i] = ".."
			}

			return strings.Join(parts, "/")
		},
	}
)

type JoinedScope struct {
	Inner interface{}
	Outer interface{}
}

func MustParseTemplate(name, body string) *template.Template {
	tmpl, err := template.New(name).Funcs(funcs).Parse(body)
	if err != nil {
		panic(err)
	}
	return tmpl
}
func MustParseYamlTemplate(name, body string) *template.Template {
	return MustParseTemplate(name, strings.Replace(body, "\t", "  ", -1))
}

//region Dockerfile
var DockerfileTemplate = MustParseTemplate("Dockerfile", `# Build
FROM golang:alpine3.12 AS build-env
WORKDIR /app

COPY . /app

RUN {{ if .BuildProxy -}}
  export HTTP_PROXY="{{ .BuildProxy }}"; export HTTPS_PROXY="{{ .BuildProxy }}";
{{- end}} go build -o webhook_server

# Copy built application to actual image
FROM alpine:3.12
WORKDIR /app

# Set configuration environments

COPY --from=build-env /app/webhook_server /app
CMD [ "/app/webhook_server", "-logtostderr"
{{- if (ne .LogLevel 0) -}}
  , "-v", "{{ .LogLevel }}"
{{- end -}}
{{- if (ne .Port 0) -}}
  , "--port", "{{ .Port }}"
{{- end -}}
{{- if .Insecure -}}
  , "--insecure"
{{- else -}}
    , "--cert", "{{ .CertificateFile }}", "--key", "{{ .PrivateKeyFile }}"
{{- end -}}
]
`)

type DockerfileData struct {
	BuildProxy      string
	Port            int
	LogLevel        int
	Insecure        bool
	CertificateFile string
	PrivateKeyFile  string
}

func WriteDockerfile(w io.Writer, data DockerfileData) error {
	return DockerfileTemplate.Execute(w, data)
}
func WriteDockerfileToFile(path string, data DockerfileData) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return WriteDockerfile(f, data)
}
func RenderDockerfile(data DockerfileData) (string, error) {
	builder := &strings.Builder{}
	err := WriteDockerfile(builder, data)
	if err != nil {
		return "", err
	}
	return builder.String(), nil
}

//endregion

//region deployment.yaml
var DeploymentTemplate = MustParseYamlTemplate("deployment", `{{ define "RenderWebhook" }}
- name: {{ .Hook.Name }}
  clientConfig:
    service:
      name: "{{ .Deployment.ServiceName }}"
      namespace: "{{ .Deployment.Namespace }}"
      path: "/{{ .Type }}/{{ .Hook.Name }}"
    {{ if and (not .Deployment.Insecure) (ne 0 (len .Deployment.CABundle)) -}}
    caBundle: {{ json .Deployment.CABundle }}
    {{- end }}
  rules:
    {{ range .Hook.Rules -}}
    - apiGroups:   [{{ quoteAndJoin .APIGroups ", " }}]
      resources:   [{{ quoteAndJoin .Resources ", " }}]
      apiVersions: [{{ quoteAndJoin .APIVersions ", " }}]
      operations:  [{{ joinOperations .Operations ", " }}]
    {{- end }}
{{- end }}{{/* end of RenderWebhook template */}}---
apiVersion: apps/v1
kind: Deployment
metadata:
name: "{{ .Name }}"
namespace: "{{ .Namespace }}"
labels:
app: "{{ .Name }}"
spec:
replicas: 1
selector:
matchLabels:
  app: "{{ .Name }}"
template:
metadata:
  labels:
    app: "{{ .Name }}"
spec:
  {{ if (ne .RunAsUser 0) -}}
  securityContext:
    runAsNonRoot: true
    runAsUser: {{ .RunAsUser }}
  {{ end -}}
  containers:
    - name: "server"
      image: "{{ if .ImageRegistry }}{{ .ImageRegistry }}/{{ end }}{{ .ImageName }}:{{ .ImageTag }}"
      imagePullPolicy: Always
      ports:
        - containerPort: {{ .ContainerPort }}
          name: "{{ .Name }}-api"
      env:
        {{ range .AllHooks -}}
        {{ range .Configurations -}}
        {{ if (ne .DefaultValue nil) -}}
        - name: "{{ .Name }}"
          value: {{ quote (deref .DefaultValue) }}
          {{ if (ne .Desc "") }}# {{ .Desc }}{{ end }}
        {{- end }}
        {{- end }}
        {{- end }}
  {{- if not .Insecure }}
      volumeMounts:
        - name: "{{ .Name }}-tls-certs"
          mountPath: "/run/secrets/{{ .Name }}"
          readOnly: true
  volumes:
    - name: "{{ .Name }}-tls-certs"
      secret:
        secretName: "{{ .TlsSecretName }}"
  {{- end }}
---
apiVersion: v1
kind: Service
metadata:
name: "{{ .ServiceName }}"
namespace: "{{ .Namespace }}"
spec:
selector:
app: "{{ .Name }}"
ports:
- port: {{ .ServerPort }}
  targetPort: "{{ .Name }}-api"
{{if (ne 0 (len .MutatingWebhooks)) -}}
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
name: "{{ .Name }}"
webhooks:
{{- range .MutatingWebhooks }}
  {{- template "RenderWebhook" (dict "Deployment" $ "Hook" . "Type" "mutate") }}
{{- end}}
{{- end }}
{{if (ne 0 (len .ValidatingWebhooks)) -}}
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
name: "{{ .Name }}"
webhooks:
{{- range .ValidatingWebhooks }}
  {{- template "RenderWebhook" (dict "Deployment" $ "Hook" . "Type" "validate") }}
{{- end }}
{{- end }}
`)

type WebhookData struct {
	Name  string
	Rules []admissionRegistration.RuleWithOperations
}

type DeploymentData struct {
	Name               string
	Namespace          string
	RunAsUser          int
	ImageRegistry      string
	ImageName          string
	ImageTag           string
	ContainerPort      int
	ServerPort         int
	Insecure           bool
	CABundle           string
	TlsSecretName      string
	ServiceName        string
	MutatingWebhooks   []WebhookData
	ValidatingWebhooks []WebhookData
}

func (this DeploymentData) AllHooks() []WebhookData {
	result := make([]WebhookData, 0, len(this.MutatingWebhooks)+len(this.ValidatingWebhooks))
	result = append(result, this.MutatingWebhooks...)
	result = append(result, this.ValidatingWebhooks...)
	return result
}

func WriteDeployment(w io.Writer, data DeploymentData) error {
	return DeploymentTemplate.Execute(w, data)
}
func WriteDeploymentToFile(path string, data DeploymentData) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return WriteDeployment(f, data)
}
func RenderDeployment(data DeploymentData) (string, error) {
	builder := &strings.Builder{}
	err := WriteDeployment(builder, data)
	if err != nil {
		return "", err
	}
	return builder.String(), nil
}

//endregion

//region deploy script
var DeployScriptTemplate = MustParseTemplate("deploy", `#!/usr/bin/env sh

{{ if not .Insecure }}
if ! kubectl get -n "{{ .Namespace }}" secrets/{{ .TlsSecretName }}; then
  echo "Creating TLS secret"
  kubectl -n "{{ .Namespace }}" create secret tls "{{ .TlsSecretName }}" \
    --cert "{{ .CertificateFile }}" --key "{{ .PrivateKeyFile }}"
fi
{{ end }}{{/* if not .Insecure */}}

echo "Creating docker image"
docker build -t "{{ if .ImageRegistry }}{{ .ImageRegistry }}/{{ end }}{{ .ImageName }}:{{ .ImageTag }}" \
  -f "{{ .DeploymentFolder }}/Dockerfile" .

echo "Pushing docker image to the registry"
docker push "{{ if .ImageRegistry }}{{ .ImageRegistry }}/{{ end }}{{ .ImageName }}:{{ .ImageTag }}"

echo "Deploy the deployment to the kubernetes"
kubectl apply -f "{{ .DeploymentFolder }}/deployment.yml"
`)

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
}

func WriteDeployScript(w io.Writer, data DeployScriptData) error {
	return DeployScriptTemplate.Execute(w, data)
}
func WriteDeployScriptToFile(path string, data DeployScriptData) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return WriteDeployScript(f, data)
}
func RenderDeployScript(data DeployScriptData) (string, error) {
	builder := &strings.Builder{}
	err := WriteDeployScript(builder, data)
	if err != nil {
		return "", err
	}
	return builder.String(), nil
}

//endregion