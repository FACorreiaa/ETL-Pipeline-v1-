package middleware

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func ServePrometheus(ctx context.Context, port string) error {
	server := http.NewServeMux()

	server.Handle("/metrics", promhttp.Handler())

	// health check
	server.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	if val := os.Getenv("METRICS_PORT"); val != "" {
		port = val
	}
	fmt.Println("Prometheus Listening on port:", port)
	return http.ListenAndServe(":"+port, server)
}
