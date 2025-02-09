package scoring

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"

	c "esgbook-software-engineer-technical-test-2024/config"
)

const (
	dir        = "data"
	numWorkers = 5
)

type ScoredRow struct {
	Key     CompanyYearKey
	Metrics map[string]float64
}

// Wrap the result in the channel
type rowResult struct {
	Row ScoredRow
	Err error
}

type CompanyYearKey struct {
	CompanyID string
	Year      int
}

type rowData struct {
	date    time.Time
	numeric map[string]float64
}

// just to test the json file
type rawJSONRow struct {
	CompanyID string   `json:"company_id"`
	DateStr   string   `json:"date"`
	Dis1      *float64 `json:"dis_1,omitempty"`
	Dis2      *float64 `json:"dis_2,omitempty"`
	Dis3      *float64 `json:"dis_3,omitempty"`
	Dis4      *float64 `json:"dis_4,omitempty"`
}

// buildDependencyGraph list of metrics that depend on 'a'
func buildDependencyGraph(logger *slog.Logger, scoreConfig *c.Config) (map[string][]string, map[string]int) {
	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	for _, m := range scoreConfig.Metrics {
		inDegree[m.Name] = 0
		graph[m.Name] = []string{}
	}

	for _, m := range scoreConfig.Metrics {
		for _, param := range m.Operation.Parameters {
			if strings.HasPrefix(param.Source, "self.") {
				depMetric := strings.TrimPrefix(param.Source, "self.")
				graph[depMetric] = append(graph[depMetric], m.Name)
				inDegree[m.Name]++
			}
		}
	}

	logger.Debug("Built dependency graph",
		slog.Any("graph", graph),
		slog.Any("inDegree", inDegree),
	)

	return graph, inDegree
}

// topologicalSort check if dependency cycle exists
func topologicalSort(
	logger *slog.Logger,
	scoreConfig *c.Config,
	graph map[string][]string,
	inDegree map[string]int,
) ([]string, error) {
	var queue []string
	for metricName, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, metricName)
		}
	}

	var sorted []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		sorted = append(sorted, current)

		for _, dep := range graph[current] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(sorted) < len(scoreConfig.Metrics) {
		logger.Error("Cycle detected in metric dependencies")
		return nil, fmt.Errorf("cycle detected in metric dependencies")
	}

	logger.Debug("Topological sort completed", slog.Any("order", sorted))

	return sorted, nil
}

func buildMetricMap(scoreConfig *c.Config) map[string]c.Metric {
	m := make(map[string]c.Metric)
	for _, met := range scoreConfig.Metrics {
		m[met.Name] = met
	}
	return m
}

func indexOf(slice []string, target string) int {
	for i, s := range slice {
		if s == target {
			return i
		}
	}
	return -1
}

func getAllDataCompanyKeys(datasets map[string]map[CompanyYearKey]map[string]float64) []CompanyYearKey {
	unique := make(map[CompanyYearKey]bool)
	for _, ds := range datasets {
		for key := range ds {
			unique[key] = true
		}
	}
	out := make([]CompanyYearKey, 0, len(unique))
	for k := range unique {
		out = append(out, k)
	}
	return out
}

func evaluateMetric(
	ctx context.Context,
	logger *slog.Logger,
	metric c.Metric,
	key CompanyYearKey,
	//results map[CompanyYearKey]map[string]float64,
	results map[string]float64,
	datasets map[string]map[CompanyYearKey]map[string]float64,
) (float64, bool) {
	// if we've already computed metric, return it
	if val, ok := results[metric.Name]; ok {
		return val, false
	}

	opFn, ok := operations[metric.Operation.Type]
	if !ok {
		logger.Warn("Unknown operation",
			slog.String("operationType", metric.Operation.Type),
			slog.String("metric", metric.Name),
		)
		return 0, true
	}

	val, isNull, err := opFn(ctx, logger, metric.Operation.Parameters, key, results, datasets)
	if err != nil {
		logger.Error("Error in operation",
			slog.String("operationType", metric.Operation.Type),
			slog.String("metric", metric.Name),
			slog.String("company_id", key.CompanyID),
			slog.Int("year", key.Year),
			slog.Any("error", err),
		)
		return 0, true
	}

	if !isNull {
		results[metric.Name] = val
	}

	return val, isNull
}

// getValue from source file
func getValue(
	logger *slog.Logger,
	source string,
	key CompanyYearKey,
	//results map[CompanyYearKey]map[string]float64,
	results map[string]float64,
	datasets map[string]map[CompanyYearKey]map[string]float64,
) (float64, bool) {
	if strings.HasPrefix(source, "self.") {
		metricName := strings.TrimPrefix(source, "self.")
		val, ok := results[metricName]
		if !ok {
			logger.Debug("Self reference not yet computed",
				slog.String("requested_metric", metricName),
				slog.String("company_id", key.CompanyID),
				slog.Int("year", key.Year),
			)
			return 0, true
		}
		return val, false
	}

	parts := strings.Split(source, ".")
	if len(parts) != 2 {
		return 0, true // invalid format => null
	}
	datasetName := parts[0]
	metricKey := parts[1]

	ds, ok := datasets[datasetName]
	if !ok {
		logger.Debug("Unknown dataset",
			slog.String("datasetName", datasetName),
		)
		return 0, true
	}
	row, ok := ds[key]
	if !ok {
		logger.Debug("No row for key",
			slog.String("company_id", key.CompanyID),
			slog.Int("year", key.Year),
			slog.String("datasetName", datasetName),
		)
		return 0, true
	}
	val, ok := row[metricKey]
	if !ok {
		logger.Debug("Metric key not found in row",
			slog.String("metricKey", metricKey),
			slog.String("datasetName", datasetName),
			slog.String("company_id", key.CompanyID),
			slog.Int("year", key.Year),
		)
		return 0, true
	}
	return val, false
}

// parallelComputeScores
func parallelComputeScores(
	ctx context.Context,
	logger *slog.Logger,
	allKeys []CompanyYearKey,
	topoOrder []string,
	metricMap map[string]c.Metric,
	datasets map[string]map[CompanyYearKey]map[string]float64,
	numWorkers int,
) []ScoredRow {
	jobs := make(chan CompanyYearKey, len(allKeys))
	results := make(chan rowResult, len(allKeys))

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for key := range jobs {
				metricsMap := computeScoresForKey(ctx, logger, key, topoOrder, metricMap, datasets)
				results <- rowResult{
					Row: ScoredRow{
						Key:     key,
						Metrics: metricsMap,
					},
					Err: nil,
				}
			}
		}()
	}

	for _, key := range allKeys {
		jobs <- key
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	var scoredRows []ScoredRow
	for r := range results {
		if r.Err != nil {
			logger.Error("Error computing row",
				slog.String("company_id", r.Row.Key.CompanyID),
				slog.Int("year", r.Row.Key.Year),
				slog.Any("error", r.Err),
			)
			continue
		}
		scoredRows = append(scoredRows, r.Row)
	}

	sort.Slice(scoredRows, func(i, j int) bool {
		iKey := scoredRows[i].Key
		jKey := scoredRows[j].Key
		if iKey.CompanyID == jKey.CompanyID {
			return iKey.Year < jKey.Year
		}
		return iKey.CompanyID < jKey.CompanyID
	})

	return scoredRows
}

// computeScoresForKey
func computeScoresForKey(
	ctx context.Context,
	logger *slog.Logger,
	key CompanyYearKey,
	topoOrder []string,
	metricMap map[string]c.Metric,
	datasets map[string]map[CompanyYearKey]map[string]float64,
) map[string]float64 {
	metricResults := make(map[string]float64)
	for _, metricName := range topoOrder {
		metricDef := metricMap[metricName]
		val, isNull := evaluateMetric(ctx, logger, metricDef, key, metricResults, datasets)
		if !isNull {
			// store the computed value
			metricResults[metricName] = val
		}
	}
	return metricResults
}

// CalculateScore from file data
func CalculateScore(
	ctx context.Context,
	logger *slog.Logger,
	configFileName string,
	dataService *DataLoaderService,
) (*c.Config, []ScoredRow, error) {
	tracer := otel.Tracer("score-app")
	ctx, span := tracer.Start(ctx, "CalculateScoreApp")
	defer span.End()

	scoreConfig, err := c.InitScoreConfig(configFileName)
	if err != nil {
		return nil, nil, fmt.Errorf("error initializing score config: %w", err)
	}
	logger.Info("Loaded config", "configFileName", configFileName, "dataService", dataService)

	// Build the dependency graph & get topological order
	graph, inDegree := buildDependencyGraph(logger, scoreConfig)
	topoOrder, err := topologicalSort(logger, scoreConfig, graph, inDegree)
	if err != nil {
		return nil, nil, fmt.Errorf("failed topological sort: %v", err)
	}
	metricMap := buildMetricMap(scoreConfig)

	// Load all CSVs (or other files) from "data/" using the injected service
	combined, err := dataService.LoadAllData(ctx, dir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load data from folder: %w", err)
	}

	datasets := make(map[string]map[CompanyYearKey]map[string]float64)
	for logicalName, csvKey := range datasetKeys {
		data, ok := combined[csvKey]
		if !ok {
			return nil, nil, fmt.Errorf("missing dataset for key=%q", csvKey)
		}
		datasets[logicalName] = data
	}

	allKeys := getAllDataCompanyKeys(datasets)

	scoredResults := parallelComputeScores(ctx, logger, allKeys, topoOrder, metricMap, datasets, numWorkers)

	logger.Info("Scoring results", "results", scoredResults)
	return scoreConfig, scoredResults, nil
}
