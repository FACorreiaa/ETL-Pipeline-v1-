package scoring

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const wastePath = "data/waste_data.csv"
const emissionPath = "data/emissions_data.csv"
const disclosurePath = "data/disclosure_data.csv"

type Handler struct {
	Ctx            context.Context
	Logger         *slog.Logger
	ConfigFileName string
}

// CalculateScoreHandler Calculate scores and print in csv format
func (h *Handler) CalculateScoreHandler(w http.ResponseWriter, r *http.Request) {
	h.Logger.Info("Calculating score")
	
	lr := NewLoaderRegistry()
	dataService := NewDataLoaderService(lr)

	scoreConfig, scoredResults, err := CalculateScore(r.Context(), h.Logger, h.ConfigFileName, dataService)
	if err != nil {
		h.Logger.Info(fmt.Sprintf("Error calculating score: %s", err.Error()))
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="scores.csv"`)
	csvWriter := csv.NewWriter(w)
	defer csvWriter.Flush()

	header := []string{"company", "year"}
	for _, metric := range scoreConfig.Metrics {
		header = append(header, metric.Name)
	}
	err = csvWriter.Write(header)
	if err != nil {
		h.Logger.Info(fmt.Sprintf("Error writing header: %s", err.Error()))
		http.Error(w, "Failed to write CSV header", http.StatusInternalServerError)
		return
	}

	for _, sr := range scoredResults {
		row := []string{
			sr.Key.CompanyID,
			strconv.Itoa(sr.Key.Year),
		}
		for _, metric := range scoreConfig.Metrics {
			if val, ok := sr.Metrics[metric.Name]; ok {
				row = append(row, fmt.Sprintf("%.2f", val))
			} else {
				row = append(row, "") // or "NULL"
			}
		}

		if err = csvWriter.Write(row); err != nil {
			h.Logger.Info(fmt.Sprintf("Error writing row: %s", err.Error()))
			http.Error(w, "Failed to write CSV row", http.StatusInternalServerError)
			return
		}
	}
}

func HealthCheckHandler(w http.ResponseWriter, _ *http.Request) {
	if err := isServiceHealthy(); err != nil {
		log.Printf("Health check failed: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Print("Status ok!")
	w.WriteHeader(http.StatusOK)
}

func isServiceHealthy() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	select {
	case <-time.After(2 * time.Second):
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timeout while checking service health")
	}
}
