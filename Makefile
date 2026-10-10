# Common tasks. Run "make" with no target to format, vet, lint, test, and build.
BINARY := csv2jsonl
# Use staticcheck from the PATH, or from Go's install folder if the PATH lacks it.
STATICCHECK := $(or $(shell command -v staticcheck),$(shell go env GOPATH)/bin/staticcheck)

.PHONY: all fmt vet lint test cover bench run build build-windows build-mac profile clean

all: fmt vet lint test build

fmt:            ## reformat code to the standard Go style
	gofmt -s -w .

vet:            ## catch suspicious code the compiler allows
	go vet ./...

lint:           ## stricter style and bug checks (needs staticcheck installed)
	$(STATICCHECK) ./...

test:           ## run every unit test
	go test -v ./...

cover:          ## test coverage report, opened in the browser
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

bench:          ## speed and memory benchmarks
	go test -run '^$$' -bench . -benchmem ./jsonl

build:          ## executable for this computer
	go build -o $(BINARY) .

build-windows:  ## Windows executable (.exe), built from any computer
	GOOS=windows GOARCH=amd64 go build -o dist/$(BINARY).exe .

build-mac:      ## Mac executables for Apple Silicon and Intel
	GOOS=darwin GOARCH=arm64 go build -o dist/$(BINARY)-mac-arm64 .
	GOOS=darwin GOARCH=amd64 go build -o dist/$(BINARY)-mac-intel .

run: build      ## convert the housing data
	./$(BINARY) -force data/housesInput.csv housesOutput.jl

profile: build  ## convert with CPU and memory profiles, then show the top CPU users
	./$(BINARY) -force -cpuprofile cpu.prof -memprofile mem.prof data/housesInput.csv housesOutput.jl
	go tool pprof -top $(BINARY) cpu.prof

clean:
	rm -rf $(BINARY) dist housesOutput.jl *.prof coverage.out
