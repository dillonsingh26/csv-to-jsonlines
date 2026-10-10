package jsonl

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
)

// housingSample is the first three rows of the California housing data.
const housingSample = `value,income,age,rooms,bedrooms,pop,hh
452600,8.3252,41,880,129,322,126
358500,8.3014,21,7099,1106,2401,1138
352100,7.2574,52,1467,190,496,177
`

// TestConvertHousingSample checks the exact output the assignment shows.
func TestConvertHousingSample(t *testing.T) {
	want := `{"value":452600,"income":8.3252,"age":41,"rooms":880,"bedrooms":129,"pop":322,"hh":126}
{"value":358500,"income":8.3014,"age":21,"rooms":7099,"bedrooms":1106,"pop":2401,"hh":1138}
{"value":352100,"income":7.2574,"age":52,"rooms":1467,"bedrooms":190,"pop":496,"hh":177}
`
	var out bytes.Buffer
	stats, err := Convert(strings.NewReader(housingSample), &out)
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}
	if got := out.String(); got != want {
		t.Errorf("output mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
	if stats != (Stats{Columns: 7, Rows: 3}) {
		t.Errorf("stats = %+v, want 7 columns and 3 rows", stats)
	}
}

func TestAppendValue(t *testing.T) {
	tests := []struct {
		field string
		want  string
	}{
		{"41", "41"},
		{"8.3252", "8.3252"},
		{"-122.23", "-122.23"},
		{"0", "0"},
		{"0.5", "0.5"},
		{"4.526e+005", "452600"},
		{"1e300", "1e+300"},
		{"", "null"},
		{"   ", "null"},
		{" 7 ", "7"},
		{"02134", `"02134"`},
		{"-007", `"-007"`},
		{"NaN", `"NaN"`},
		{"Inf", `"Inf"`},
		{"Fresno", `"Fresno"`},
		{`say "hi"`, `"say \"hi\""`},
		{"line\nbreak", `"line\nbreak"`},
		{"café", `"café"`},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			got, err := AppendValue(nil, tt.field)
			if err != nil {
				t.Fatalf("AppendValue(%q) error: %v", tt.field, err)
			}
			if string(got) != tt.want {
				t.Errorf("AppendValue(%q) = %s, want %s", tt.field, got, tt.want)
			}
		})
	}
}

// TestConvertGeneralCSV shows the converter works on any CSV, not just the
// housing data: quoted commas, text columns, blanks, and a byte-order mark.
func TestConvertGeneralCSV(t *testing.T) {
	input := "\xef\xbb\xbfcity,zip,note\n" +
		"\"Los Angeles, CA\",90001,\n" +
		"Fresno,93650,\"has \"\"quotes\"\"\"\n"
	want := `{"city":"Los Angeles, CA","zip":90001,"note":null}
{"city":"Fresno","zip":93650,"note":"has \"quotes\""}
`
	var out bytes.Buffer
	if _, err := Convert(strings.NewReader(input), &out); err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}
	if got := out.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestConvertHeaderOnly(t *testing.T) {
	var out bytes.Buffer
	stats, err := Convert(strings.NewReader("a,b\n"), &out)
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}
	if out.Len() != 0 || stats.Rows != 0 {
		t.Errorf("expected empty output, got %q and %d rows", out.String(), stats.Rows)
	}
}

func TestConvertErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMsg string
	}{
		{"empty input", "", "no header"},
		{"blank header", "a,,c\n1,2,3\n", "column 2 is blank"},
		{"duplicate header", "a,b,a\n1,2,3\n", `"a" appears more than once`},
		{"short row", "a,b,c\n1,2\n", "wrong number of fields"},
		{"bad quote", "a,b\n\"1,2\n", "quote"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Convert(strings.NewReader(tt.input), &bytes.Buffer{})
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("error %q does not mention %q", err, tt.wantMsg)
			}
		})
	}
	if _, err := Convert(strings.NewReader(""), &bytes.Buffer{}); !errors.Is(err, ErrNoHeader) {
		t.Errorf("empty input should return ErrNoHeader, got %v", err)
	}
}

// TestConvertSyntheticRoundTrip generates random housing-style rows, converts
// them, and checks that every line is valid JSON holding the original values.
func TestConvertSyntheticRoundTrip(t *testing.T) {
	const rowCount = 5000
	rng := rand.New(rand.NewPCG(1, 2)) // fixed seed so failures can be reproduced
	input, rows := syntheticHousingCSV(rng, rowCount)

	var out bytes.Buffer
	if _, err := Convert(strings.NewReader(input), &out); err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) != rowCount {
		t.Fatalf("got %d lines, want %d", len(lines), rowCount)
	}
	for i, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatalf("line %d is not valid JSON: %s", i+1, line)
		}
		var decoded map[string]float64
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("line %d: %v", i+1, err)
		}
		for column, want := range rows[i] {
			if decoded[column] != want {
				t.Fatalf("line %d %s = %v, want %v", i+1, column, decoded[column], want)
			}
		}
	}
}

// syntheticHousingCSV builds CSV text shaped like the housing data, and
// returns the generated values so a test can compare against them.
func syntheticHousingCSV(rng *rand.Rand, rowCount int) (string, []map[string]float64) {
	var text strings.Builder
	text.WriteString("value,income,age,rooms,bedrooms,pop,hh\n")
	rows := make([]map[string]float64, rowCount)

	for i := range rows {
		rows[i] = map[string]float64{
			"value":    float64(15000 + rng.IntN(485000)),
			"income":   float64(rng.IntN(150000)) / 10000, // four decimal places, like the real data
			"age":      float64(1 + rng.IntN(52)),
			"rooms":    float64(2 + rng.IntN(39000)),
			"bedrooms": float64(1 + rng.IntN(6400)),
			"pop":      float64(3 + rng.IntN(35000)),
			"hh":       float64(1 + rng.IntN(6000)),
		}
		fmt.Fprintf(&text, "%s,%s,%s,%s,%s,%s,%s\n",
			formatFloat(rows[i]["value"]), formatFloat(rows[i]["income"]),
			formatFloat(rows[i]["age"]), formatFloat(rows[i]["rooms"]),
			formatFloat(rows[i]["bedrooms"]), formatFloat(rows[i]["pop"]),
			formatFloat(rows[i]["hh"]))
	}
	return text.String(), rows
}

func formatFloat(number float64) string {
	return strconv.FormatFloat(number, 'f', -1, 64)
}

// BenchmarkConvert measures speed and memory per conversion of 10,000
// synthetic rows. Run with: go test -bench . -benchmem ./jsonl
func BenchmarkConvert(b *testing.B) {
	input, _ := syntheticHousingCSV(rand.New(rand.NewPCG(1, 2)), 10000)
	b.SetBytes(int64(len(input)))
	for b.Loop() {
		if _, err := Convert(strings.NewReader(input), &bytes.Buffer{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAppendValue(b *testing.B) {
	buf := make([]byte, 0, 64)
	for b.Loop() {
		buf, _ = AppendValue(buf[:0], "8.3252")
	}
}
