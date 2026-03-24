package importer

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// CSVRecord is one parsed row with column->value mapping
type CSVRecord struct {
	Fields map[string]string // column name -> value
	Row    int               // 1-based row number
}

// ParseCSV reads all rows from a CSV reader, returns header names + records
func ParseCSV(r io.Reader) (headers []string, records []CSVRecord, err error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true

	headers, err = cr.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("reading CSV headers: %w", err)
	}
	// Normalize header names: trim whitespace, lowercase
	for i, h := range headers {
		headers[i] = strings.TrimSpace(h)
	}

	row := 1
	for {
		row++
		vals, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("reading CSV row %d: %w", row, err)
		}
		rec := CSVRecord{Fields: make(map[string]string), Row: row}
		for i, h := range headers {
			if i < len(vals) {
				rec.Fields[h] = strings.TrimSpace(vals[i])
			}
		}
		records = append(records, rec)
	}
	return headers, records, nil
}
