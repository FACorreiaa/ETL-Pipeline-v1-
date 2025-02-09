package scoring

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseDateOrYear(raw string) (time.Time, error) {
	if strings.Contains(raw, "-") {
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid date format %q: %v", raw, err)
		}
		return t, nil
	}

	yearInt, err := strconv.Atoi(raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid year %q: %v", raw, err)
	}

	return time.Date(yearInt, time.January, 1, 0, 0, 0, 0, time.UTC), nil
}

func validateData(companyID string, year int) error {
	if companyID == "" {
		return fmt.Errorf("missing company_id field")
	}
	if year < 1900 || year > 2100 {
		return fmt.Errorf("invalid year: %d", year)
	}
	return nil
}
