// AUTO GENERATED TEMPLATE FILE
package webhook_core

import (
    "io"
    "os"
    "strings"
    "text/template"
)


//region Dockerfile template
var DockerfileTemplate = template.Must(ParseTemplate("Dockerfile", strings.Join([]string{
    "FROM golang:alpine3.12 AS build-env",
    "WORKDIR /app",
    "",
    "COPY . /app",
    "",
    "RUN {{if .BuildProxy}}export HTTP_PROXY=\"{{.BuildProxy}}\"; export HTTPS_PROXY=\"{{.BuildProxy}}\";{{end}} go build -o webhook_server",
    "",
    "# Copy built application to actual image",
    "FROM alpine:3.12",
    "WORKDIR /app",
    "",
    "COPY --from=build-env /app/webhook_server /app",
}, "\n")))

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

//region DeployScript template
var DeployScriptTemplate = template.Must(ParseTemplate("DeployScript", strings.Join([]string{
    "#!/usr/bin/env sh",
    "",
    "{{ if not .Insecure }}",
    "if ! {{ .Kubectl }} get -n \"{{ .Namespace }}\" secrets/{{ .TlsSecretName }}; then",
    "  echo \"Creating TLS secret\"",
    "  {{ .Kubectl }} -n \"{{ .Namespace }}\" create secret tls \"{{ .TlsSecretName }}\" \\",
    "    --cert \"{{ .CertificateFile }}\" --key \"{{ .PrivateKeyFile }}\"",
    "fi",
    "{{ end }}{{/* if not .Insecure */}}",
    "",
    "echo \"Creating docker image\"",
    "docker build -t \"{{ if .ImageRegistry }}{{ .ImageRegistry }}/{{ end }}{{ .ImageName }}:{{ .ImageTag }}\" \\",
    "  -f \"{{ .DeploymentFolder }}/Dockerfile\" .",
    "",
    "echo \"Pushing docker image to the registry\"",
    "docker push \"{{ if .ImageRegistry }}{{ .ImageRegistry }}/{{ end }}{{ .ImageName }}:{{ .ImageTag }}\"",
    "",
    "echo \"Deploy the deployment to the kubernetes\"",
    "{{ .Kubectl }} apply -f \"{{ .DeploymentFolder }}/deployment.yml\"",
}, "\n")))

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

//region Deployment template
var DeploymentTemplate = template.Must(ParseTemplate("Deployment", strings.Join([]string{
    "{{ define \"RenderWebhook\" }}",
    "- name: \"{{ .Hook.Name }}.{{ .Deployment.ServiceName }}.{{ .Deployment.Namespace }}.svc\"",
    "  clientConfig:",
    "    service:",
    "      name: \"{{ .Deployment.ServiceName }}\"",
    "      namespace: \"{{ .Deployment.Namespace }}\"",
    "      path: \"/{{ .Type }}/{{ .Hook.Name }}\"",
    "    {{ if and (not .Deployment.Insecure) (ne 0 (len .Deployment.CABundle)) -}}",
    "    caBundle: {{ Quote .Deployment.CABundle }}",
    "    {{- end }}",
    "  rules:",
    "    {{ range .Hook.Rules -}}",
    "    - apiGroups:   [{{ QuoteAndJoin .APIGroups `, ` }}]",
    "      resources:   [{{ QuoteAndJoin .Resources `, ` }}]",
    "      apiVersions: [{{ QuoteAndJoin .APIVersions `, ` }}]",
    "      operations:  [{{ JoinOperations .Operations `, ` }}]",
    "    {{- end }}",
    "  admissionReviewVersions: [{{ QuoteAndJoin .Hook.SupportedAdmissionVersions `, ` }}]",
    "  sideEffects: {{ if eq .Hook.SideEffects `` }}None{{else}}{{ Quote .Hook.SideEffects }}{{end}}",
    "  timeoutSeconds: {{ .Hook.TimeoutInSeconds }}",
    "{{- end }}",
    "apiVersion: apps/v1",
    "kind: Deployment",
    "metadata:",
    "  name: \"{{ .Name }}\"",
    "  namespace: \"{{ .Namespace }}\"",
    "  labels:",
    "    app: \"{{ .Name }}\"",
    "spec:",
    "  replicas: 1",
    "  selector:",
    "    matchLabels:",
    "      app: \"{{ .Name }}\"",
    "  template:",
    "    metadata:",
    "      labels:",
    "        app: \"{{ .Name }}\"",
    "    spec:",
    "      {{ if (ne .RunAsUser 0) -}}",
    "      securityContext:",
    "        runAsNonRoot: true",
    "        runAsUser: {{ .RunAsUser }}",
    "      {{ end -}}",
    "      containers:",
    "        - name: \"server\"",
    "          image: \"{{ if .ImageRegistry }}{{ .ImageRegistry }}/{{ end }}{{ .ImageName }}:{{ .ImageTag }}\"",
    "          args:",
    "            - \"/app/webhook_server\"",
    "            - \"-logtostderr\"",
    "            {{- if (ne .LogLevel 0) }}",
    "            - \"-v\"",
    "            - \"{{ .LogLevel }}\"",
    "            {{- end }}",
    "            - \"--port\"",
    "            - \"{{ .ContainerPort }}\"",
    "            {{ if not .Insecure -}}",
    "            - \"--cert\"",
    "            - \"/run/secrets/{{ .Name }}/tls.crt\"",
    "            - \"--key\"",
    "            - \"/run/secrets/{{ .Name }}/tls.key\"",
    "            {{ else }}",
    "            - \"--insecure\"",
    "            {{- end }}",
    "          imagePullPolicy: Always",
    "          ports:",
    "            - containerPort: {{ .ContainerPort }}",
    "              name: \"{{ .Name }}-api\"",
    "          env:",
    "            {{ if (ne .Desc \"\") }}# {{ .Desc }}{{ end }}",
    "            {{ range .AllHooks }}{{ range .Configurations }}{{ if (ne .DefaultValue nil) -}}",
    "            - name: \"{{ .Name }}\"",
    "              value: {{ Quote (Deref .DefaultValue) }}",
    "            {{ end }}{{ end }}{{ end }}",
    "      {{- if not .Insecure }}",
    "          volumeMounts:",
    "            - name: \"{{ .Name }}-tls-certs\"",
    "              mountPath: \"/run/secrets/{{ .Name }}\"",
    "              readOnly: true",
    "      volumes:",
    "        - name: \"{{ .Name }}-tls-certs\"",
    "          secret:",
    "            secretName: \"{{ .TlsSecretName }}\"",
    "      {{- end }}",
    "---",
    "apiVersion: v1",
    "kind: Service",
    "metadata:",
    "  name: \"{{ .ServiceName }}\"",
    "  namespace: \"{{ .Namespace }}\"",
    "  labels:",
    "    app: \"{{ .Name }}\"",
    "spec:",
    "  {{ if .ServiceUser }}serviceAccountName: {{ .ServiceUser }}{{ end }}",
    "  selector:",
    "    app: \"{{ .Name }}\"",
    "  ports:",
    "    - port: {{ .ServerPort }}",
    "      targetPort: \"{{ .Name }}-api\"",
    "{{if (ne 0 (len .MutatingWebhooks)) -}}",
    "---",
    "apiVersion: admissionregistration.k8s.io/v1",
    "kind: MutatingWebhookConfiguration",
    "metadata:",
    "  name: \"{{ .ServiceName }}.{{ .Namespace }}.svc\"",
    "webhooks:",
    "  {{- range .MutatingWebhooks }}",
    "    {{- template \"RenderWebhook\" (MakeDict \"Deployment\" $ \"Hook\" . \"Type\" \"mutate\") }}",
    "  {{- end}}",
    "{{- end }}",
    "{{if (ne 0 (len .ValidatingWebhooks)) -}}",
    "---",
    "apiVersion: admissionregistration.k8s.io/v1",
    "kind: ValidatingWebhookConfiguration",
    "metadata:",
    "  name: \"{{ .ServiceName }}.{{ .Namespace }}.svc\"",
    "webhooks:",
    "  {{- range .ValidatingWebhooks }}",
    "    {{- template \"RenderWebhook\" (MakeDict \"Deployment\" $ \"Hook\" . \"Type\" \"validate\") }}",
    "  {{- end }}",
    "{{- end }}",
}, "\n")))

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

