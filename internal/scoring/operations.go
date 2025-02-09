package scoring

import (
	"context"
	"fmt"
	"log/slog"

	c "esgbook-software-engineer-technical-test-2024/config"
)

func evalSum(
	ctx context.Context,
	logger *slog.Logger,
	params []c.Parameter,
	key CompanyYearKey,
	results map[string]float64,
	datasets map[string]map[CompanyYearKey]map[string]float64,
) (float64, bool, error) {

	var total float64
	var anyNonNull bool

	for _, p := range params {
		val, isNull := getValue(logger, p.Source, key, results, datasets)
		if !isNull {
			total += val
			anyNonNull = true
		}
	}

	if !anyNonNull {
		logger.Error("[evalSum] All parameters are null => null")
		return 0, true, nil
	}

	return total, false, nil
}

func evalOr(
	ctx context.Context,
	logger *slog.Logger,
	params []c.Parameter,
	key CompanyYearKey,
	results map[string]float64,
	datasets map[string]map[CompanyYearKey]map[string]float64,
) (float64, bool, error) {

	if len(params) < 2 {
		logger.Warn("[evalOr] Not enough parameters found")
		return 0, true, fmt.Errorf("[evalOr] not enough parameters")
	}

	xVal, xNull := getValue(logger, params[0].Source, key, results, datasets)
	yVal, yNull := getValue(logger, params[1].Source, key, results, datasets)

	if !xNull {
		return xVal, false, nil
	}
	if !yNull {
		return yVal, false, nil
	}

	return 0, true, nil
}

func evalDivide(
	ctx context.Context,
	logger *slog.Logger,
	params []c.Parameter,
	key CompanyYearKey,
	results map[string]float64,
	//results map[CompanyYearKey]map[string]float64,

	datasets map[string]map[CompanyYearKey]map[string]float64,
) (float64, bool, error) {

	if len(params) < 2 {
		return 0, true, fmt.Errorf("[evalDivide] not enough parameters")
	}
	xVal, xNull := getValue(logger, params[0].Source, key, results, datasets)
	yVal, yNull := getValue(logger, params[1].Source, key, results, datasets)

	if xNull || yNull {
		return 0, true, nil
	}
	if yVal == 0 {
		return 0, true, fmt.Errorf("[evalDivide] division by zero")
	}

	return xVal / yVal, false, nil
}

// OperationFn file operations store
type OperationFn func(
	ctx context.Context,
	logger *slog.Logger,
	params []c.Parameter,
	key CompanyYearKey,
	results map[string]float64,
	datasets map[string]map[CompanyYearKey]map[string]float64,
) (float64, bool, error)

var operations = map[string]OperationFn{
	"sum":    evalSum,
	"or":     evalOr,
	"divide": evalDivide,
}
