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

func Quote(value string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`) + `"`
}

var (
	funcs = template.FuncMap{
		"Json":  json.Marshal,
		"Join":  strings.Join,
		"Deref": func(value *string) string { return *value },
		"Quote": Quote,
		"QuoteAndJoin": func(values []string, sep string) string {
			builder := &strings.Builder{}
			for i := 0; i < len(values); i++ {
				if i != 0 {
					builder.WriteString(sep)
				}
				builder.WriteString(Quote(values[i]))
			}
			return builder.String()
		},
		"JoinOperations": func(operations []admissionRegistration.OperationType, sep string) string {
			result := ""
			first := true
			for _, operation := range operations {
				if first {
					first = false
				} else {
					result += sep
				}
				result += Quote(string(operation))
			}
			return result
		},
		"JoinScope": func(outer, inner interface{}) JoinedScope {
			return JoinedScope{
				Inner: inner,
				Outer: outer,
			}
		},
		"MakeDict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values) == 0 {
				return nil, errors.New("Invalid call to MakeDict")
			}
			if (len(values) & 1) == 1 {
				return nil, errors.New("Invalid number of arguments for MakeDict")
			}

			result := make(map[string]interface{})
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, errors.New("Keys must be string")
				}

				result[key] = values[i+1]
			}

			return result, nil
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
    caBundle: {{ Quote .Deployment.CABundle }}
    {{- end }}
  rules:
    {{ range .Hook.Rules -}}
    - apiGroups:   [{{ QuoteAndJoin .APIGroups ", " }}]
      resources:   [{{ QuoteAndJoin .Resources ", " }}]
      apiVersions: [{{ QuoteAndJoin .APIVersions ", " }}]
      operations:  [{{ JoinOperations .Operations ", " }}]
    {{- end }}
  admissionReviewVersions: [{{ QuoteAndJoin .Hook.SupportedAdmissionVersions ", " }}]
  sideEffects: {{ if eq .Hook.SideEffects "" }}None{{else}}{{ Quote .Hook.SideEffects }}{{end}}
  timeoutSeconds: {{ .Hook.TimeoutInSeconds }}
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
          args:
            - "/app/webhook_server"
            - "-logtostderr"
            {{- if (ne .LogLevel 0) }}
            - "-v"
            - "{{ .LogLevel }}"
            {{- end }}
            - "--port"
            - "{{ .ContainerPort }}"
            {{ if not .Insecure -}}
            - "--cert"
            - "/run/secrets/{{ .Name }}/tls.crt"
            - "--key"
            - "/run/secrets/{{ .Name }}/tls.key"
            {{ else }}
            - "--insecure"
            {{- end }}
          imagePullPolicy: Always
          ports:
            - containerPort: {{ .ContainerPort }}
              name: "{{ .Name }}-api"
          env:
            {{ range .AllHooks }}{{ range .Configurations }}{{ if (ne .DefaultValue nil) -}}
        {{ if (ne .Desc "") }}# {{ .Desc }}{{ end }}
            - name: "{{ .Name }}"
              value: {{ Quote (Deref .DefaultValue) }}
            {{ end }}{{ end }}{{ end }}
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
    {{- template "RenderWebhook" (MakeDict "Deployment" $ "Hook" . "Type" "mutate") }}
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
    {{- template "RenderWebhook" (MakeDict "Deployment" $ "Hook" . "Type" "validate") }}
  {{- end }}
{{- end }}
`)

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
