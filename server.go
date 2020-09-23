package webhook_core

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	log "github.com/golang/glog"
)

// WebhookServer actual server that listen for incoming requests and generate
// a response for them by calling `Webhook` instances
type WebhookServer struct {
	// Port number that server should listen on it
	Port int
	// Host that server should listen on it
	Host string
	// TlsConfig if this server should work in secure mode, then this is the certificate
	TlsConfig *tls.Config
	// Webhooks list of webhooks that should used by this server
	Webhooks []Webhook

	server *http.Server
}

// CreateServerFromFlags create a server from application flags
func CreateServerFromFlags(webhooks ...Webhook) (*WebhookServer, error) {
	var certFile, keyFile string

	server := &WebhookServer{}
	flag.IntVar(&server.Port, "port", 0, "port that server should listen on it")
	flag.StringVar(&server.Host, "host", "0.0.0.0", "host that server should listen on it")
	flag.StringVar(&certFile, "cert", "", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&keyFile, "key", "", "File containing the x509 private key to --cert.")

	// initialize all webhooks
	bindings := make([]AppFlagInitializationResponse, 0, len(webhooks))
	for _, webhook := range webhooks {
		bindings = append(bindings, TryInitializeApplicationFlags(webhook))
	}

	flag.Parse()

	for _, binding := range bindings {
		err := CompleteBinding(binding)
		if err != nil {
			return nil, err
		}
	}

	server.Webhooks = webhooks
	if server.Port == 0 {
		if server.TlsConfig != nil {
			server.Port = 443
		} else {
			server.Port = 80
		}
	}

	if certFile != "" {
		certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}

		server.TlsConfig = &tls.Config{
			Certificates: []tls.Certificate{certificate},
		}
	}

	return server, nil
}

// CreateServerFromFlagsAndRunToTermination create a server from application flags
// and run it until receive the termination signal
func CreateServerFromFlagsAndRunToTermination(webhooks ...Webhook) error {
	server, err := CreateServerFromFlags(webhooks...)
	if err != nil {
		return err
	}

	return server.RunToTermination()
}

// handleRequest handle incoming requests
func (this *WebhookServer) handleRequest(w http.ResponseWriter, r *http.Request) {
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

	for _, webhook := range this.Webhooks {
		action, ok := GetWebhookAction(r.URL.Path, webhook.Path())
		if !ok {
			continue
		}

		if log.V(10) {
			log.Infof("Trying to handle the request with %v at %v(%v)",
				webhook.Name(), webhook.Path(), action)
		}

		response, err := webhook.HandleAdmission(action, r, ar)
		if err != nil {
			e := fmt.Sprintf("Error in handling admission request: %v", err)
			log.Error(e)
			http.Error(w, e, http.StatusBadRequest)
			return
		}

		if response != nil {
			// we already got a response from this webhook
			WriteAdmissionResponse(w, apiVersion, ar, response)
			return
		}
	}

	http.Error(w, "Not found", http.StatusNotFound)
}

// Run run this webhook server
func (this *WebhookServer) Run() (chan error, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { this.handleRequest(w, r) })

	this.server = &http.Server{
		Handler:   mux,
		Addr:      fmt.Sprintf("%s:%d", this.Host, this.Port),
		TLSConfig: this.TlsConfig,
	}

	stopped := make(chan error, 1)
	go func() {
		var err error
		if this.server.TLSConfig != nil {
			log.Infof("Listening for client at (%s:%d) using HTTPS", this.Host, this.Port)
			err = this.server.ListenAndServeTLS("", "")
		} else {
			log.Infof("Listening for client at (%s:%d) using HTTP", this.Host, this.Port)
			err = this.server.ListenAndServe()
		}
		if err != nil {
			log.Errorf("Failed to start HTTP(S) server: %v", err)
		}
		stopped <- err
	}()

	return stopped, nil
}

// Shutdown this server
func (this *WebhookServer) Shutdown() {
	this.server.Shutdown(context.Background())
}

// RunToTermination Run the server until user press Ctrl+C
func (this *WebhookServer) RunToTermination() error {
	serverStoppedChan, err := this.Run()
	if err != nil {
		return err
	}

	stopRequestedChan := make(chan os.Signal, 1)
	signal.Notify(stopRequestedChan, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-stopRequestedChan:
		log.Infof("Got OS shutdown signal, shutting down webhook server gracefully...")
		this.Shutdown()
		<-serverStoppedChan // wait for completion of stop
	case err = <-serverStoppedChan:
		log.Warning("Server stopped unexpectedly")
		return err
	}
	return nil
}
