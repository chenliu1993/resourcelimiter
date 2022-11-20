package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	infoLogger    *log.Logger
	warningLogger *log.Logger
	errorLogger   *log.Logger
)

var (
	port                                 int
	webhookNamespace, webhookServiceName string
)

func init() {
	// init loggers
	infoLogger = log.New(os.Stderr, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	warningLogger = log.New(os.Stderr, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	// webhook server running namespace
	webhookNamespace = os.Getenv("POD_NAMESPACE")

	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionregistrationv1.AddToScheme(runtimeScheme)
	_ = v1.AddToScheme(runtimeScheme)
	_ = rlv1beta1.AddToScheme(runtimeScheme)
	_ = rlv1beta2.AddToScheme(runtimeScheme)
}

func main() {
	// init command flags
	flag.IntVar(&port, "port", 8443, "Webhook server port.")
	flag.StringVar(&webhookServiceName, "service-name", "rl-checker", "Webhook service name.")
	// flag.StringVar(&sidecarConfigFile, "sidecar-config-file", "/etc/webhook/config/sidecarconfig.yaml", "Sidecar injector configuration file.")
	// flag.StringVar(&certFile, "tlsCertFile", "/etc/webhook/certs/cert.pem", "x509 Certificate file.")
	// flag.StringVar(&keyFile, "tlsKeyFile", "/etc/webhook/certs/key.pem", "x509 private key file.")
	flag.Parse()

	dnsNames := []string{
		webhookServiceName,
		webhookServiceName + "." + webhookNamespace,
		webhookServiceName + "." + webhookNamespace + ".svc",
	}
	commonName := webhookServiceName + "." + webhookNamespace + ".svc"

	var err error
	org := "cliufreever"
	caPEM, certPEM, certKeyPEM, err := generateCert([]string{org}, dnsNames, commonName)
	if err != nil {
		errorLogger.Fatalf("Failed to generate ca and certificate key pair: %v", err)
	}

	pair, err := tls.X509KeyPair(certPEM.Bytes(), certKeyPEM.Bytes())
	if err != nil {
		errorLogger.Fatalf("Failed to load certificate key pair: %v", err)
	}

	var config *rest.Config
	infoLogger.Println("Initializing the kube client...")
	kubeconfig := os.Getenv("KUBECONFIG")

	if kubeconfig == "" {
		if config, err = rest.InClusterConfig(); err != nil {
			errorLogger.Fatalf("failed to get incluster config %v", err)
		}
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		errorLogger.Fatalf("failed to build config %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		errorLogger.Fatalf("failed to create clientset: %v", err)
	}

	// create or update the mutatingwebhookconfiguration
	err = createOrUpdateWebhookConfiguration(clientset, caPEM, webhookServiceName, webhookNamespace, true)
	if err != nil {
		errorLogger.Fatalf("Failed to create or update the mutating webhook configuration: %v", err)
	}

	err = createOrUpdateWebhookConfiguration(clientset, caPEM, webhookServiceName, webhookNamespace, false)
	if err != nil {
		errorLogger.Fatalf("Failed to create or update the validating webhook configuration: %v", err)
	}

	whsvr := &WebhookServer{
		server: &http.Server{
			Addr:      fmt.Sprintf(":%v", port),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},
	}

	// define http server and server handler
	mux := http.NewServeMux()
	mux.HandleFunc(WebhookMutatePath, whsvr.ServeMutate)
	mux.HandleFunc(WebhookValidatePath, whsvr.ServeValidate)
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

	infoLogger.Printf("Got OS shutdown signal, shutting down webhook server gracefully...")
	whsvr.server.Shutdown(context.Background())
}
