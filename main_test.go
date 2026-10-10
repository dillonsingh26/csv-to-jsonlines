package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempFile creates a file with the given contents in a test folder.
func writeTempFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRunHousingFile converts the real data file end to end and checks that
// every output line is valid JSON with all seven keys.
func TestRunHousingFile(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "housesOutput.jl")
	var stderr bytes.Buffer

	if code := run([]string{"data/housesInput.csv", outputPath}, &stderr); code != exitOK {
		t.Fatalf("exit code %d, stderr: %s", code, stderr.String())
	}

	file, err := os.Open(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	lineCount := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineCount++
		var record map[string]float64
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("line %d is not valid JSON: %v", lineCount, err)
		}
		if len(record) != 7 {
			t.Fatalf("line %d has %d keys, want 7", lineCount, len(record))
		}
	}
	if lineCount != 20640 {
		t.Errorf("got %d lines, want 20640", lineCount)
	}
}

func TestRunRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	goodCSV := writeTempFile(t, dir, "good.csv", "a,b\n1,2\n")
	existing := writeTempFile(t, dir, "existing.jl", "old")
	badCSV := writeTempFile(t, dir, "bad.csv", "a,b\n1\n")

	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantMsg  string
	}{
		{"no arguments", nil, exitUsageErr, "expected 2 file paths"},
		{"one argument", []string{goodCSV}, exitUsageErr, "expected 2 file paths"},
		{"missing input", []string{filepath.Join(dir, "nope.csv"), filepath.Join(dir, "o.jl")}, exitUsageErr, "does not exist"},
		{"input is folder", []string{dir, filepath.Join(dir, "o.jl")}, exitUsageErr, "is a folder"},
		{"same file", []string{goodCSV, goodCSV}, exitUsageErr, "must be different"},
		{"missing output folder", []string{goodCSV, filepath.Join(dir, "nowhere", "o.jl")}, exitUsageErr, "output folder"},
		{"output exists", []string{goodCSV, existing}, exitUsageErr, "-force"},
		{"unknown flag", []string{"-bogus", goodCSV, existing}, exitUsageErr, "flag provided but not defined"},
		{"malformed csv", []string{badCSV, filepath.Join(dir, "bad.jl")}, exitConvertErr, "conversion failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			if code := run(tt.args, &stderr); code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			if !strings.Contains(stderr.String(), tt.wantMsg) {
				t.Errorf("stderr %q does not mention %q", stderr.String(), tt.wantMsg)
			}
		})
	}

	// A failed conversion must not leave a half-written file behind.
	if _, err := os.Stat(filepath.Join(dir, "bad.jl")); !os.IsNotExist(err) {
		t.Error("partial output file was not removed after a failed conversion")
	}
}

func TestRunForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	input := writeTempFile(t, dir, "in.csv", "a\n1\n")
	output := writeTempFile(t, dir, "out.jl", "old contents")

	if code := run([]string{"-force", "-quiet", input, output}, &bytes.Buffer{}); code != exitOK {
		t.Fatalf("exit code %d", code)
	}
	got, _ := os.ReadFile(output)
	if string(got) != "{\"a\":1}\n" {
		t.Errorf("output = %q", got)
	}
}

func TestRunHelp(t *testing.T) {
	var stderr bytes.Buffer
	if code := run([]string{"-h"}, &stderr); code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Error("help text was not printed")
	}
}
