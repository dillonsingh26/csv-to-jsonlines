// Command csv2jsonl converts a comma-separated values (CSV) file into a
// JSON lines file, with one JSON object per line.
//
// Usage:
//
//	csv2jsonl [flags] <input.csv> <output.jl>
//
// The first CSV row must be a header; its names become the JSON keys.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/dillonsingh26/csv-to-jsonlines/jsonl"
)

// Exit codes let scripts tell a usage mistake apart from a failed conversion.
const (
	exitOK         = 0
	exitConvertErr = 1
	exitUsageErr   = 2
)

const usageText = `csv2jsonl converts a CSV file into a JSON lines file (one JSON object per line).

Usage:
  csv2jsonl [flags] <input.csv> <output.jl>

Example:
  csv2jsonl data/housesInput.csv housesOutput.jl

The first row of the CSV must be a header. Its names become the JSON keys.
Numbers become JSON numbers, empty fields become null, other text becomes strings.

Flags:
`

// options holds everything the user can set on the command line.
type options struct {
	inputPath  string
	outputPath string
	force      bool
	quiet      bool
	cpuProfile string
	memProfile string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run is the whole program apart from os.Exit, which keeps it testable.
// Messages for the user go to stderr; the converted data goes to the output file.
func run(args []string, stderr io.Writer) int {
	logger := log.New(stderr, "csv2jsonl: ", 0)

	opts, err := parseArgs(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return exitOK
	}
	if err != nil {
		logger.Print(err)
		fmt.Fprintln(stderr, "Run 'csv2jsonl -h' for usage.")
		return exitUsageErr
	}

	if err := validatePaths(opts); err != nil {
		logger.Print(err)
		return exitUsageErr
	}

	if opts.cpuProfile != "" {
		stopProfile, err := startCPUProfile(opts.cpuProfile)
		if err != nil {
			logger.Print(err)
			return exitConvertErr
		}
		defer stopProfile()
	}

	start := time.Now()
	stats, err := convertFile(opts.inputPath, opts.outputPath)
	if err != nil {
		logger.Printf("conversion failed: %v", err)
		return exitConvertErr
	}

	if !opts.quiet {
		logger.Printf("wrote %d rows x %d columns to %s in %v (%s)",
			stats.Rows, stats.Columns, opts.outputPath,
			time.Since(start).Round(time.Microsecond), memorySummary())
	}

	if opts.memProfile != "" {
		if err := writeMemProfile(opts.memProfile); err != nil {
			logger.Print(err)
			return exitConvertErr
		}
	}
	return exitOK
}

// parseArgs reads the flags and the two required file paths.
func parseArgs(args []string, stderr io.Writer) (options, error) {
	var opts options
	flags := flag.NewFlagSet("csv2jsonl", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(stderr, usageText)
		flags.PrintDefaults()
	}

	flags.BoolVar(&opts.force, "force", false, "overwrite the output file if it already exists")
	flags.BoolVar(&opts.quiet, "quiet", false, "do not print the summary line")
	flags.StringVar(&opts.cpuProfile, "cpuprofile", "", "write a CPU profile to this `file`")
	flags.StringVar(&opts.memProfile, "memprofile", "", "write a memory profile to this `file`")

	if err := flags.Parse(args); err != nil {
		return opts, err
	}
	if flags.NArg() != 2 {
		return opts, fmt.Errorf("expected 2 file paths (input and output), got %d", flags.NArg())
	}
	opts.inputPath = flags.Arg(0)
	opts.outputPath = flags.Arg(1)
	return opts, nil
}

// validatePaths catches bad input before any file is created, so the user
// gets a clear explanation instead of a half-written output file.
func validatePaths(opts options) error {
	info, err := os.Stat(opts.inputPath)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("input file %q does not exist", opts.inputPath)
	}
	if err != nil {
		return fmt.Errorf("cannot read input file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("input %q is a folder, not a CSV file", opts.inputPath)
	}

	if sameFile(opts.inputPath, opts.outputPath) {
		return errors.New("input and output must be different files")
	}

	outputDir := filepath.Dir(opts.outputPath)
	if _, err := os.Stat(outputDir); err != nil {
		return fmt.Errorf("output folder %q does not exist", outputDir)
	}
	if _, err := os.Stat(opts.outputPath); err == nil && !opts.force {
		return fmt.Errorf("output file %q already exists; add -force to overwrite it", opts.outputPath)
	}
	return nil
}

// sameFile reports whether two paths point at the same file on disk.
func sameFile(pathA, pathB string) bool {
	infoA, errA := os.Stat(pathA)
	infoB, errB := os.Stat(pathB)
	if errA != nil || errB != nil {
		return false
	}
	return os.SameFile(infoA, infoB)
}

// convertFile opens both files and runs the conversion. If anything fails,
// the partial output file is removed so no invalid JSON lines file is left behind.
func convertFile(inputPath, outputPath string) (stats jsonl.Stats, err error) {
	input, err := os.Open(inputPath)
	if err != nil {
		return stats, err
	}
	defer input.Close()

	output, err := os.Create(outputPath)
	if err != nil {
		return stats, err
	}
	defer func() {
		// Close errors matter on write: they can mean the data never reached disk.
		if closeErr := output.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			os.Remove(outputPath)
		}
	}()

	return jsonl.Convert(input, output)
}

// startCPUProfile begins CPU profiling and returns the function that stops it.
func startCPUProfile(path string) (func(), error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("creating CPU profile: %w", err)
	}
	if err := pprof.StartCPUProfile(file); err != nil {
		file.Close()
		return nil, fmt.Errorf("starting CPU profile: %w", err)
	}
	return func() {
		pprof.StopCPUProfile()
		file.Close()
	}, nil
}

// writeMemProfile saves a heap profile showing where memory was allocated.
func writeMemProfile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating memory profile: %w", err)
	}
	defer file.Close()

	runtime.GC() // collect garbage first so the profile shows live data accurately
	if err := pprof.WriteHeapProfile(file); err != nil {
		return fmt.Errorf("writing memory profile: %w", err)
	}
	return nil
}

// memorySummary reports total bytes allocated and peak memory taken from the
// operating system, for the summary log line.
func memorySummary() string {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return fmt.Sprintf("allocated %.1f MiB, system %.1f MiB",
		float64(mem.TotalAlloc)/(1<<20), float64(mem.Sys)/(1<<20))
}
