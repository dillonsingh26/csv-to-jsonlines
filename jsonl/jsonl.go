// Package jsonl converts comma-separated values (CSV) into JSON lines,
// also called newline-delimited JSON (NDJSON).
//
// The first CSV row is treated as a header and supplies the JSON keys.
// Every later row becomes one JSON object on its own line. Fields that look
// like numbers are written as JSON numbers, empty fields as null, and
// everything else as JSON strings. Column order is preserved.
//
// The package has no dependencies outside the standard library, so it can be
// reused by any Go program that needs CSV-to-JSON-lines conversion.
package jsonl

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// byteOrderMark is the invisible marker some spreadsheet tools put at the
// start of a UTF-8 file. It would otherwise end up inside the first key.
const byteOrderMark = "\xef\xbb\xbf"

// ErrNoHeader is returned when the input contains no rows at all.
var ErrNoHeader = errors.New("input has no header row")

// Stats summarizes a completed conversion.
type Stats struct {
	Columns int // number of fields in the header
	Rows    int // number of data rows written (header excluded)
}

// Convert reads CSV from r and writes JSON lines to w.
//
// Reading and writing are streamed one row at a time, so memory use stays
// flat no matter how large the input file is.
func Convert(r io.Reader, w io.Writer) (Stats, error) {
	reader := csv.NewReader(r)
	reader.ReuseRecord = true // reuse the row slice to avoid an allocation per row
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if errors.Is(err, io.EOF) {
		return Stats{}, ErrNoHeader
	}
	if err != nil {
		return Stats{}, fmt.Errorf("reading header: %w", err)
	}

	keys, err := encodeKeys(header)
	if err != nil {
		return Stats{}, err
	}
	stats := Stats{Columns: len(keys)}

	out := bufio.NewWriter(w)
	line := make([]byte, 0, 256) // buffer reused for every output line

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			// csv.ParseError already names the line and column.
			return stats, err
		}

		line, err = appendObject(line[:0], keys, record)
		if err != nil {
			return stats, err
		}
		if _, err := out.Write(line); err != nil {
			return stats, fmt.Errorf("writing output: %w", err)
		}
		stats.Rows++
	}

	if err := out.Flush(); err != nil {
		return stats, fmt.Errorf("writing output: %w", err)
	}
	return stats, nil
}

// encodeKeys turns header names into ready-to-write JSON keys such as "value":
// Encoding them once up front saves repeating the work on every row.
func encodeKeys(header []string) ([][]byte, error) {
	keys := make([][]byte, len(header))
	seen := make(map[string]bool, len(header))

	for i, name := range header {
		name = strings.TrimSpace(strings.TrimPrefix(name, byteOrderMark))
		if name == "" {
			return nil, fmt.Errorf("header column %d is blank", i+1)
		}
		if seen[name] {
			return nil, fmt.Errorf("header column %q appears more than once", name)
		}
		seen[name] = true

		quoted, err := json.Marshal(name)
		if err != nil {
			return nil, fmt.Errorf("encoding header %q: %w", name, err)
		}
		keys[i] = append(quoted, ':')
	}
	return keys, nil
}

// appendObject appends one JSON object, followed by a newline, to dst.
// The csv package already guarantees that every row has as many fields as
// the header, so keys and record always line up.
func appendObject(dst []byte, keys [][]byte, record []string) ([]byte, error) {
	dst = append(dst, '{')
	for i, field := range record {
		if i > 0 {
			dst = append(dst, ',')
		}
		dst = append(dst, keys[i]...)

		var err error
		dst, err = AppendValue(dst, field)
		if err != nil {
			return dst, err
		}
	}
	return append(dst, '}', '\n'), nil
}

// AppendValue appends the JSON form of a single CSV field to dst:
//
//	""        -> null
//	"41"      -> 41
//	"8.3252"  -> 8.3252
//	"02134"   -> "02134" (leading zeros are kept, so ZIP codes survive)
//	"Fresno"  -> "Fresno"
func AppendValue(dst []byte, field string) ([]byte, error) {
	field = strings.TrimSpace(field)
	if field == "" {
		return append(dst, "null"...), nil
	}
	if number, ok := parseNumber(field); ok {
		return appendNumber(dst, number), nil
	}

	quoted, err := json.Marshal(field)
	if err != nil {
		return dst, fmt.Errorf("encoding field %q: %w", field, err)
	}
	return append(dst, quoted...), nil
}

// parseNumber reports whether field should become a JSON number.
func parseNumber(field string) (float64, bool) {
	if hasLeadingZero(field) {
		return 0, false
	}
	number, err := strconv.ParseFloat(field, 64)
	if err != nil {
		return 0, false
	}
	// JSON has no way to write NaN or infinity, so keep those as text.
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	return number, true
}

// hasLeadingZero catches identifiers like "007" or "-02134". Zero itself and
// decimals such as "0.5" do not count.
func hasLeadingZero(field string) bool {
	digits := strings.TrimLeft(field, "+-")
	return len(digits) > 1 && digits[0] == '0' && digits[1] != '.'
}

// appendNumber writes the shortest form that reproduces the value exactly,
// so 452600.0 becomes 452600 and 8.3252 stays 8.3252. Very large or very
// small values use exponent notation, which is also valid JSON.
func appendNumber(dst []byte, number float64) []byte {
	magnitude := math.Abs(number)
	if magnitude != 0 && (magnitude < 1e-6 || magnitude >= 1e21) {
		return strconv.AppendFloat(dst, number, 'e', -1, 64)
	}
	return strconv.AppendFloat(dst, number, 'f', -1, 64)
}
