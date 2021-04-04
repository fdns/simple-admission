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
)

const (
	port = "8443"
)

var (
	certFile, keyFile, runtimeClass string
)

func main() {
	flag.StringVar(&certFile, "certFileFile", "/certs/server.pem", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&keyFile, "keyFileFile", "/certs/server-key.pem", "File containing the x509 private key to --certFileFile.")
	flag.StringVar(&runtimeClass, "runtimeClass", "gvisor", "RuntimeClass of the sandboxed environment")

	flag.Parse()

	certs, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Printf("Error loading key pair: %v", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr: fmt.Sprintf(":%v", port),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{certs},
		},
	}

	// Define server  handler
	handler := AdmissionHandler{
		RuntimeClass: runtimeClass,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/validate", handler.handler)
	server.Handler = mux

	go func() {
		log.Printf("Listening on port %v", port)
		if err := server.ListenAndServeTLS("", ""); err != nil {
			log.Printf("Failed to listen and serve webhook server: %v", err)
			os.Exit(1)
		}
	}()

	// listening shutdown singal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	log.Printf("Shutting down webserver")
	server.Shutdown(context.Background())
}
