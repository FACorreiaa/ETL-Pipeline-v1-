package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/otel"

	"esgbook-software-engineer-technical-test-2024/internal/scoring"
	"esgbook-software-engineer-technical-test-2024/middleware"
)

const timeout = 10

const file = "score_1.yaml"

func BoostrapServer(logger *slog.Logger, ctx context.Context) error {
	server := http.NewServeMux()

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8000"
	}

	h := scoring.Handler{
		Ctx:            ctx,
		Logger:         logger,
		ConfigFileName: file,
	}

	exp, err := middleware.NewOTLPExporter(ctx)
	if err != nil {
		log.Fatal(err)
	}

	tp := middleware.NewTraceProvider(exp)

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()
	otel.SetTracerProvider(tp)

	server.HandleFunc("/run-scores", h.CalculateScoreHandler)
	server.HandleFunc("/health", scoring.HealthCheckHandler)
	wrapped := middleware.LoggingMiddleware(logger)(server)
	logger.Info("Starting service on :8000")
	if err := http.ListenAndServe(":"+serverPort, wrapped); err != nil {
		logger.Error("Failed to start server", "err", err)
	}

	return nil
}

func main() {
	logger := middleware.InitLogger()
	ctx, cancel := context.WithTimeout(context.Background(), timeout*time.Minute)
	defer cancel()
	errChan := make(chan error, 2)

	go func() {
		if err := BoostrapServer(logger, ctx); err != nil {
			logger.Error("Failed to boostrap server", "err", err)
			log.Fatal(err)
		}
	}()

	go func() {
		if err := middleware.ServePrometheus(ctx, ""); err != nil {
			logger.Error("Failed to serve prometheus", "err", err)
			log.Fatal(err)
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("Shutting down server")
	case err := <-errChan:
		logger.Error("Server exited with error", "err", err)
	}

}
