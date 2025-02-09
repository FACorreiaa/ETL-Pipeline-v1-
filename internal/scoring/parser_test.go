package scoring

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDatasetCSV(t *testing.T) {
	// 1) Create a temporary file with minimal CSV content for testing.
	csvContent := `company_id,date,dis_1,dis_2,dis_3,dis_4
1000,2023-06-01,12.34,56.78,77.85,39.61
1001,2024-01-15,90.12,40.40,80.10,10.10
1001,2024-06-30,44.44,88.88,30.00,20.00`

	tmpfile, err := os.CreateTemp("", "disclosure-*.csv")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name()) // clean up

	_, err = tmpfile.WriteString(csvContent)
	require.NoError(t, err)
	require.NoError(t, tmpfile.Close())

	// 2) Call the function under test
	results, err := loadDatasetCSV(tmpfile.Name())
	require.NoError(t, err)

	// 3) Validate what we expect
	//
	// For the CSV above, we have:
	// company=1000, date=2023-06-01 => year=2023
	// company=1001, date=2024-01-15 => year=2024
	// company=1001, date=2024-06-30 => year=2024 (later date => should overwrite the earlier row)

	// We expect 2 unique (company, year) keys: (1000, 2023) and (1001, 2024).
	require.Len(t, results, 2)

	// Check (1000, 2023)
	key1000_2023 := CompanyYearKey{CompanyID: "1000", Year: 2023}
	row, ok := results[key1000_2023]
	require.True(t, ok)
	assert.Equal(t, 12.34, row["dis_1"])
	assert.Equal(t, 56.78, row["dis_2"])

	key1001_2024 := CompanyYearKey{CompanyID: "1001", Year: 2024}
	row2, ok2 := results[key1001_2024]
	require.True(t, ok2)
	assert.Equal(t, 44.44, row2["dis_1"])
	assert.Equal(t, 88.88, row2["dis_2"])
}

func TestLoadDatasetJSON(t *testing.T) {
	// Minimal JSON content
	jsonContent := `[
        {"company_id":"1000","date":"2023-06-01","dis_1":12.34,"dis_2":56.78,"dis_3":77.85,"dis_4":39.61},
        {"company_id":"1001","date":"2024-01-15","dis_1":90.12,"dis_2":40.40,"dis_3":80.10,"dis_4":10.10},
        {"company_id":"1001","date":"2024-06-30","dis_1":44.44,"dis_2":88.88,"dis_3":30.00,"dis_4":20.00}
    ]`

	tmpfile, err := os.CreateTemp("", "disclosure-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.WriteString(jsonContent)
	require.NoError(t, err)
	require.NoError(t, tmpfile.Close())

	loader := JSONLoader{}

	results, err := loader.loadData(context.Background(), tmpfile.Name())
	require.NoError(t, err)

	// 3) We expect (1000, 2023) and (1001, 2024) final entries, with "later" row overwriting the earlier one for (1001,2024).
	require.Len(t, results, 2)

	key1000_2023 := CompanyYearKey{CompanyID: "1000", Year: 2023}
	row, ok := results[key1000_2023]
	require.True(t, ok, "Expected an entry for (1000,2023)")
	assert.Equal(t, 12.34, row["dis_1"])
	assert.Equal(t, 56.78, row["dis_2"])

	key1001_2024 := CompanyYearKey{CompanyID: "1001", Year: 2024}
	row2, ok2 := results[key1001_2024]
	require.True(t, ok2, "Expected an entry for (1001,2024)")

	// date "2024-06-30" vs "2024-01-15", we expect the 3rd to overwrite.
	assert.Equal(t, 44.44, row2["dis_1"])
	assert.Equal(t, 88.88, row2["dis_2"])
}

func TestParseDateOrYear(t *testing.T) {
	// Define a table of test cases
	tests := []struct {
		name     string
		input    string
		wantTime time.Time
		wantErr  bool
	}{
		{
			name:     "Full date (YYYY-MM-DD)",
			input:    "2023-06-01",
			wantTime: time.Date(2023, time.June, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "Year only (YYYY)",
			input:    "2024",
			wantTime: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:    "Invalid date format",
			input:   "2023/06/01",
			wantErr: true,
		},
		{
			name:    "Invalid year",
			input:   "20ab",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDateOrYear(tt.input)

			if tt.wantErr {
				// We expect an error
				require.Error(t, err, "Expected an error but got none.")
			} else {
				// We expect no error
				require.NoError(t, err, "Unexpected error: %v", err)
				// Check that the time.Time is exactly as expected
				assert.Equal(t, tt.wantTime, got)
			}
		})
	}
}
