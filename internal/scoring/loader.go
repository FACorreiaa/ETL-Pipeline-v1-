package scoring

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DataLoader interface: for reading data from a specific file/path.
type DataLoader interface {
	loadData(ctx context.Context, path string) (map[CompanyYearKey]map[string]float64, error)
}

// CSVLoader A simple CSV loader example.
type CSVLoader struct{}

func (CSVLoader) loadData(ctx context.Context, path string) (map[CompanyYearKey]map[string]float64, error) {
	return loadDatasetCSV(path) // reuse your existing CSV parsing function
}

// JSONLoader Temporary files to test the JSON Import only
type JSONLoader struct{}

func (JSONLoader) loadData(ctx context.Context, path string) (map[CompanyYearKey]map[string]float64, error) {
	return loadJSONDataset(path)
}

func loadDatasetCSV(filename string) (map[CompanyYearKey]map[string]float64, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read headers: %v", err)
	}

	idxCompany := indexOf(headers, "company_id")
	idxDate := indexOf(headers, "date")
	if idxCompany == -1 || idxDate == -1 {
		return nil, fmt.Errorf("missing required columns (company_id, date)")
	}

	// Use rowData to store the 'latest' row (by full date) for each (company, year)
	data := make(map[CompanyYearKey]rowData)

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		companyID := row[idxCompany]

		parsedTime, err := parseDateOrYear(row[idxDate])
		if err != nil {
			log.Printf("Skipping row for %s due to date parse error: %v", companyID, err)
			continue
		}
		yearInt := parsedTime.Year()

		if err := validateData(companyID, yearInt); err != nil {
			log.Printf("Skipping invalid row: company_id=%s, year=%d (error: %v)", companyID, yearInt, err)
			continue
		}

		key := CompanyYearKey{
			CompanyID: companyID,
			Year:      yearInt,
		}

		numericVals := map[string]float64{}
		for i, colName := range headers {
			if i == idxCompany || i == idxDate {
				continue
			}
			valStr := row[i]
			if valStr == "" {
				continue
			}
			if v, err := strconv.ParseFloat(valStr, 64); err == nil {
				numericVals[colName] = v
			}
		}

		// 3) Check if we have an existing entry for (company, year). If not, store it.
		if existing, ok := data[key]; !ok {
			data[key] = rowData{
				date:    parsedTime,
				numeric: numericVals,
			}
		} else {
			if parsedTime.After(existing.date) {
				data[key] = rowData{
					date:    parsedTime,
					numeric: numericVals,
				}
			}
		}
	}

	result := make(map[CompanyYearKey]map[string]float64)
	for key, rd := range data {
		result[key] = rd.numeric
	}

	return result, nil
}

// loadJSONDataset ignore example just to check loadJSON
func loadJSONDataset(filename string) (map[CompanyYearKey]map[string]float64, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	bytes, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", filename, err)
	}

	var rows []rawJSONRow
	if err := json.Unmarshal(bytes, &rows); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON from %s: %w", filename, err)
	}

	data := make(map[CompanyYearKey]rowData)

	for _, r := range rows {
		parsedTime, err := parseDateOrYear(r.DateStr)
		if err != nil {
			log.Printf("Skipping row for company=%s due to invalid date %q: %v", r.CompanyID, r.DateStr, err)
			continue
		}

		yearInt := parsedTime.Year()

		key := CompanyYearKey{
			CompanyID: r.CompanyID,
			Year:      yearInt,
		}

		numericVals := make(map[string]float64)
		if r.Dis1 != nil {
			numericVals["dis_1"] = *r.Dis1
		}
		if r.Dis2 != nil {
			numericVals["dis_2"] = *r.Dis2
		}
		if r.Dis3 != nil {
			numericVals["dis_3"] = *r.Dis3
		}
		if r.Dis4 != nil {
			numericVals["dis_4"] = *r.Dis4
		}

		if existing, ok := data[key]; !ok {
			data[key] = rowData{
				date:    parsedTime,
				numeric: numericVals,
			}
		} else {
			if parsedTime.After(existing.date) {
				data[key] = rowData{
					date:    parsedTime,
					numeric: numericVals,
				}
			}
		}
	}

	result := make(map[CompanyYearKey]map[string]float64, len(data))
	for k, rd := range data {
		result[k] = rd.numeric
	}

	return result, nil
}

// LoadAllData load data from folder
func (s *DataLoaderService) LoadAllData(
	ctx context.Context,
	dataDir string,
) (map[string]map[CompanyYearKey]map[string]float64, error) {

	files, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read data directory %s: %w", dataDir, err)
	}

	combined := make(map[string]map[CompanyYearKey]map[string]float64)

	for _, f := range files {
		if f.IsDir() {
			continue // skip subdirectories
		}

		fullPath := filepath.Join(dataDir, f.Name())
		ext := filepath.Ext(f.Name()) // e.g. ".csv"

		loader, ok := s.registry.GetLoader(ext)
		if !ok {
			log.Printf("[WARN] Skipping file with unsupported extension %q: %s", ext, f.Name())
			continue
		}

		ds, err := loader.loadData(ctx, fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load data from %s: %w", f.Name(), err)
		}

		// e.g. "waste_data.csv" => datasetName = "waste_data"
		datasetName := strings.TrimSuffix(f.Name(), ext)
		combined[datasetName] = ds
	}

	return combined, nil
}
