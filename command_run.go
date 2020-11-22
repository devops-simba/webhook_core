package webhook_core

import (
	"context"
	"fmt"
	"net/http"

	"github.com/devops-simba/helpers"
	log "github.com/golang/glog"
)

// RunWebhooks run webhooks, listening to admission reviews and reply to them with a proper admission response
func RunWebhooks(command *CLICommand) error {
	handler, err := createServerHandler(command)
	if err != nil {
		return err
	}

	server := createHttpServer(command, handler)
	stopped := make(chan error, 1)
	if command.CertificateFile != "" {
		go func() {
			server.ListenAndServeTLS(command.CertificateFile, command.PrivateKeyFile)
			close(stopped)
		}()
	} else {
		go func() {
			server.ListenAndServe()
			close(stopped)
		}()
	}

	return helpers.WaitForApplicationTermination(func() { server.Shutdown(context.Background()) }, stopped)
}
func getWebhookPath(webhook AdmissionWebhook) (path string, err error) {
	switch webhook.Type() {
	case MutatingAdmissionWebhook:
		path = "/mutate"
	case ValidatingAdmissionWebhook:
		path = "/validate"
	default:
		err = InvalidWebhookType
		return
	}

	path += "/" + webhook.Name()
	return
}
func admissionHandlerFunc(webhook AdmissionWebhook) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if log.V(8) {
			log.Infof("Request(%v):\n  Content-Type: %v\n  Content-Length: %v",
				r.URL.Path, r.Header["Content-Type"], r.Header["Content-Length"])
			if r.Body == nil {
				log.Infof("Request body is nil!")
			}
		}

		apiVersion, ar, err := ReadAdmissionReview(r)
		if err != nil {
			log.Errorf("Error in deserializing admission request: %v", err)
			http.Error(w, "Invalid content", http.StatusBadRequest)
			return
		}

		log.V(10).Infof("Trying to handle request with %s", webhook.Name())
		response, err := webhook.HandleAdmission(r, ar)
		if err != nil {
			e := fmt.Sprintf("Error in handling admission request: %v", err)
			log.Error(e)
			http.Error(w, e, http.StatusBadRequest)
		} else {
			WriteAdmissionResponse(w, apiVersion, ar, response)
		}
	})
}
func createServerHandler(command *CLICommand) (http.Handler, error) {
	mux := http.NewServeMux()
	for _, webhook := range command.Webhooks {
		webhook.Initialize()

		path, err := getWebhookPath(webhook)
		if err != nil {
			return nil, err
		}

		mux.Handle(path, admissionHandlerFunc(webhook))
	}
	return mux, nil
}
func createHttpServer(command *CLICommand, handler http.Handler) *http.Server {
	port := command.Port
	if port == 0 {
		if command.CertificateFile != "" {
			port = 443
		} else {
			port = 80
		}
	}

	return &http.Server{
		Addr:    fmt.Sprintf("%s:%d", command.Host, port),
		Handler: handler,
	}
}
