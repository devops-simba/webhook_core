package webhook_core

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	log "github.com/golang/glog"
)

type WebhookServerFactory struct {
	Port            int
	Host            string
	CertificateFile string
	PrivateKeyFile  string
	Handlers        map[string]http.HandlerFunc
}

func admissionHandlerFunc(webhook Webhook) http.HandlerFunc {
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

func (this *WebhookServerFactory) BindToFlags() {
	flag.IntVar(&this.Port, "port", 0, "port that server should listen on it")
	flag.StringVar(&this.Host, "host", "0.0.0.0", "host that server should listen on it")
	flag.StringVar(&this.CertificateFile, "cert", "", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&this.PrivateKeyFile, "key", "", "File containing the x509 private key to --cert.")
}
func (this *WebhookServerFactory) appendWebhook(path string, webhook Webhook) {
	webhookName := webhook.Name()
	if webhookName != "" {
		path += "/" + webhookName
	}
	this.Handlers[path] = admissionHandlerFunc(webhook)
}
func (this *WebhookServerFactory) AppendMutatinWebhook(webhook Webhook) {
	this.appendWebhook("/mutate", webhook)
}
func (this *WebhookServerFactory) AppendValidatingWebhook(webhook Webhook) {
	this.appendWebhook("/validate", webhook)
}
func (this *WebhookServerFactory) CreateServer() *WebhookServer {
	mux := http.NewServeMux()
	for path, handler := range this.Handlers {
		mux.Handle(path, handler)
	}

	port := this.Port
	if port == 0 {
		if this.CertificateFile != "" {
			port = 443
		} else {
			port = 80
		}
	}

	result := &WebhookServer{
		server: &http.Server{
			Addr:    fmt.Sprintf("%s:%d", this.Host, port),
			Handler: mux,
		},
	}
	if this.CertificateFile != "" {
		result.runner = func() error {
			return result.server.ListenAndServeTLS(this.CertificateFile, this.PrivateKeyFile)
		}
	} else {
		result.runner = result.server.ListenAndServe
	}
	return result
}

// WebhookServer actual server that listen for incoming requests and generate
// a response for them by calling `Webhook` instances
type WebhookServer struct {
	server *http.Server
	runner func() error
}

// Run run this webhook server
func (this *WebhookServer) Run() <-chan error {
	stopped := make(chan error, 1)
	go func() {
		stopped <- this.runner()
	}()

	return stopped
}

// Shutdown this server
func (this *WebhookServer) Shutdown() {
	this.server.Shutdown(context.Background())
}

// RunToTermination Run the server until user press Ctrl+C
func (this *WebhookServer) RunToTermination() error {
	serverStoppedChan := this.Run()

	stopRequestedChan := make(chan os.Signal, 1)
	signal.Notify(stopRequestedChan, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-stopRequestedChan:
		log.Infof("Got OS shutdown signal, shutting down webhook server gracefully...")
		this.Shutdown()
		<-serverStoppedChan // wait for stop of server
	case err := <-serverStoppedChan:
		log.Warningf("Server stopped unexpectedly: %v", err)
		return err
	}
	return nil
}
