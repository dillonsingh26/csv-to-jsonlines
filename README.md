# csv-to-jsonlines

A Go command-line application that converts a comma-separated values (CSV) file into a **JSON lines** file, where every line is one complete JSON object.

```
./csv2jsonl data/housesInput.csv housesOutput.jl
```

```
{"value":452600,"income":8.3252,"age":41,"rooms":880,"bedrooms":129,"pop":322,"hh":126}
{"value":358500,"income":8.3014,"age":21,"rooms":7099,"bedrooms":1106,"pop":2401,"hh":1138}
{"value":352100,"income":7.2574,"age":52,"rooms":1467,"bedrooms":190,"pop":496,"hh":177}
```

JSON lines (also called NDJSON, newline-delimited JSON) is a common format for loading databases and document stores. Unlike a JSON array, each line stands alone, so a file can be streamed and loaded one record at a time.

The tool is **general-purpose**: it works on any CSV with a header row, not only the housing data.

## Repository contents

| Path | Role |
|---|---|
| `main.go` | Command-line program: reads arguments, validates input, runs the conversion, logs a summary, optional profiling |
| `main_test.go` | End-to-end tests of the command, including every bad-input case |
| `jsonl/jsonl.go` | Reusable package that does the conversion; standard library only |
| `jsonl/jsonl_test.go` | Unit tests, a synthetic-data round-trip test, and benchmarks |
| `jsonl/example_test.go` | Runnable example of using the package from other Go code |
| `data/housesInput.csv` | California housing data, 20,640 rows (Miller 2015) |
| `Makefile` | Shortcuts for formatting, vetting, linting, testing, benchmarking, and building |
| `go.mod` | Go module definition |

## Requirements

- Go 1.24 or later ([go.dev/dl](https://go.dev/dl/))
- Optional: `staticcheck` for linting: `go install honnef.co/go/tools/cmd/staticcheck@latest`
- Optional: `make` (preinstalled on macOS and Linux). Every `make` target below also lists the plain Go command.

## Quick start

```bash
git clone https://github.com/dillonsingh26/csv-to-jsonlines.git
cd csv-to-jsonlines
go build -o csv2jsonl .
./csv2jsonl data/housesInput.csv housesOutput.jl
```

On Windows (PowerShell or Command Prompt):

```powershell
go build -o csv2jsonl.exe .
.\csv2jsonl.exe data\housesInput.csv housesOutput.jl
```

## Usage

```
csv2jsonl [flags] <input.csv> <output.jl>
```

| Flag | Effect |
|---|---|
| `-force` | Overwrite the output file if it already exists |
| `-quiet` | Skip the summary line |
| `-cpuprofile file` | Write a CPU profile |
| `-memprofile file` | Write a memory profile |
| `-h` | Show help |

### Conversion rules

| CSV field | JSON value |
|---|---|
| `41`, `8.3252`, `4.5e5` | number: `41`, `8.3252`, `450000` |
| empty | `null` |
| `02134`, a leading zero | string `"02134"`, so ZIP codes and IDs are not damaged |
| `Fresno`, any other text | string `"Fresno"` |

Header names become keys and keep their column order. Quoted fields, commas inside quotes, Windows line endings, and a spreadsheet byte-order mark are all handled.

### Input checking

The program checks its input before creating any file and explains what went wrong:

```
$ ./csv2jsonl
csv2jsonl: expected 2 file paths (input and output), got 0
Run 'csv2jsonl -h' for usage.

$ ./csv2jsonl missing.csv out.jl
csv2jsonl: input file "missing.csv" does not exist

$ ./csv2jsonl data/housesInput.csv housesOutput.jl
csv2jsonl: output file "housesOutput.jl" already exists; add -force to overwrite it
```

It also rejects a folder as input, the same file as input and output, a missing output folder, blank or duplicate header names, and rows with the wrong number of fields. If a conversion fails partway, the partial output file is deleted so an invalid file is never left behind.

Exit codes: `0` success, `1` conversion failed, `2` usage mistake.

## Testing

```bash
make test        # go test -v ./...
make cover       # coverage report in the browser
```

The tests cover:

- **Exact output.** The first rows of the housing data match the assignment's expected output character for character.
- **Value conversion.** A table of field types: integers, decimals, exponents, blanks, leading zeros, NaN, quotes, newlines, Unicode.
- **General CSV.** Quoted commas, text columns, empty fields, and a byte-order mark.
- **Errors.** Empty input, blank or duplicate headers, short rows, and broken quotes.
- **Synthetic round trip.** 5,000 randomly generated housing-style rows (fixed seed) are converted, then each line is parsed back and compared to the generated values.
- **End to end.** The full housing file is converted through the command; all 20,640 lines must be valid JSON with seven keys.
- **Command-line handling.** Every bad-input case returns the right exit code and message.

## Code quality tools

```bash
make fmt     # gofmt -s -w .    reformat to standard Go style
make vet     # go vet ./...     find suspicious code the compiler allows
make lint    # staticcheck ./... stricter bug and style checks
make         # all of the above, then test and build
```

## Performance

The converter streams one row at a time, so memory stays flat regardless of file size. It reuses buffers between rows and encodes the header keys only once.

```bash
make bench      # go test -run '^$' -bench . -benchmem ./jsonl
make profile    # run with -cpuprofile and -memprofile, then print the top CPU users
```

Every run logs its time and memory use:

```
csv2jsonl: wrote 20640 rows x 7 columns to housesOutput.jl in 21.491ms (allocated 0.8 MiB, system 8.0 MiB)
```

Benchmark on an Apple M4:

| Benchmark | Time per op | Throughput | Memory per op |
|---|---|---|---|
| Convert 10,000 rows | 4.9 ms | 78 MB/s | 2.4 MB |
| Convert one field | 43 ns | | 0 B |

To explore a profile interactively: `go tool pprof -http=:8080 csv2jsonl cpu.prof`.

## Building an executable

Go compiles to a single self-contained executable with no runtime to install. It can also build for another operating system from any computer by setting `GOOS` (target system) and `GOARCH` (target processor).

| Target | Command | Output |
|---|---|---|
| This computer | `go build -o csv2jsonl .` | `csv2jsonl` |
| Windows | `GOOS=windows GOARCH=amd64 go build -o dist/csv2jsonl.exe .` | `dist/csv2jsonl.exe` |
| Mac, Apple Silicon | `GOOS=darwin GOARCH=arm64 go build -o dist/csv2jsonl-mac-arm64 .` | `dist/csv2jsonl-mac-arm64` |
| Mac, Intel | `GOOS=darwin GOARCH=amd64 go build -o dist/csv2jsonl-mac-intel .` | `dist/csv2jsonl-mac-intel` |

`make build-windows` and `make build-mac` run these. On Windows PowerShell, set the variables first: `$env:GOOS="windows"; go build -o csv2jsonl.exe .`

A Mac command-line tool is a plain executable run from Terminal. The `.app` bundle format is for windowed applications, so it does not apply here. To run the tool from any folder, copy it onto your PATH, for example `sudo cp csv2jsonl /usr/local/bin/`.

## Design notes

- **Reusable package.** All conversion logic lives in `jsonl`, which takes any `io.Reader` and `io.Writer`. Other programs can import it to convert files, network streams, or in-memory text.
- **Testable command.** `main` only calls `run`, which takes arguments and an error stream and returns an exit code. Tests call `run` directly.
- **Standard library only.** The `flag` package covers two arguments and a few flags. Gerardi (2021) recommends the Cobra framework for larger tools with subcommands, which this tool does not need.
- **Ordered keys.** Lines are written directly instead of through a Go map, because `encoding/json` sorts map keys alphabetically and would lose column order.

## References

Bodner, Jon. 2024. *Learning Go: An Idiomatic Approach to Real-World Go Programming*. 2nd ed. Sebastopol, CA: O'Reilly.

Gerardi, Ricardo. 2021. *Powerful Command-Line Applications in Go: Build Fast and Maintainable Tools*. Raleigh, NC: The Pragmatic Bookshelf.

McConnell, Steve. 2004. *Code Complete*. 2nd ed. Redmond, WA: Microsoft Press.

Miller, Thomas W. 2015. *Modeling Techniques in Predictive Analytics with Python and R: A Guide to Data Science*. Upper Saddle River, NJ: Pearson Education.

## Use of AI assistants

Claude AI was used for assistance with code creation for this assignment. 
