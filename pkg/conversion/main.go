package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	rlv1beta1 "github.com/chenliu1993/resourcelimiter/api/v1beta1"
	rlv1beta2 "github.com/chenliu1993/resourcelimiter/api/v1beta2"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var (
	infoLogger    *log.Logger
	warningLogger *log.Logger
	errorLogger   *log.Logger
)

var (
	port                            int
	webhookCertFile, webhookKeyFile string
)

func init() {
	// init loggers
	infoLogger = log.New(os.Stderr, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	warningLogger = log.New(os.Stderr, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionregistrationv1.AddToScheme(runtimeScheme)
	_ = v1.AddToScheme(runtimeScheme)
	_ = rlv1beta1.AddToScheme(runtimeScheme)
	_ = rlv1beta2.AddToScheme(runtimeScheme)
}

func main() {
	// init command flags
	flag.IntVar(&port, "port", 8444, "Webhook server port.")
	flag.StringVar(&webhookCertFile, "tlsCertFile", "/etc/webhook/certs/cert.pem", "x509 Certificate file.")
	flag.StringVar(&webhookKeyFile, "tlsKeyFile", "/etc/webhook/certs/key.pem", "x509 private key file.")
	flag.Parse()

	var err error

	certFileReader, err := os.Open(webhookCertFile)
	if err != nil {
		errorLogger.Fatalf("failed to open cert file %v", err)
	}
	certPEM, err := io.ReadAll(certFileReader)
	if err != nil {
		errorLogger.Fatalf("failed to read cert file %v", err)
	}

	keyFileReader, err := os.Open(webhookKeyFile)
	if err != nil {
		errorLogger.Fatalf("failed to open key file %v", err)
	}
	certKeyPEM, err := io.ReadAll(keyFileReader)
	if err != nil {
		errorLogger.Fatalf("failed to read key file %v", err)
	}

	pair, err := tls.X509KeyPair(certPEM, certKeyPEM)
	if err != nil {
		errorLogger.Fatalf("Failed to load certificate key pair: %v", err)
	}

	whsvr := &WebhookServer{
		server: &http.Server{
			Addr:      fmt.Sprintf(":%v", port),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},
	}

	// define http server and server handler
	mux := http.NewServeMux()
	mux.HandleFunc(WebhookConvertPath, whsvr.ServeConvert)
	whsvr.server.Handler = mux

	// start webhook server in new rountine
	go func() {
		if err := whsvr.server.ListenAndServeTLS("", ""); err != nil {
			errorLogger.Fatalf("Failed to listen and serve webhook server: %v", err)
		}
	}()

	// listening OS shutdown singal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	infoLogger.Printf("Got OS shutdown signal, shutting down conversion webhook server gracefully...")
	whsvr.server.Shutdown(context.Background())
}
