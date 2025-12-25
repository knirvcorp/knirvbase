.PHONY: build clean test bench bench-integration bench-sla

build:
	go build -o ./bin/knirvbase cmd/main.go

clean:
	rm -rf ./bin/knirvbase

test:
	go test ./...

bench:
	go test -bench=. -benchmem -run=^$$ ./internal/benchmarks

bench-integration:
	go test -run=TestBenchmarkSLAs -v ./internal/benchmarks

bench-sla:
	go test -run=TestBenchmarkSLAs -v ./internal/benchmarks

# Run all benchmarks with detailed output
bench-all: bench bench-integration

# Run benchmarks and generate profile
bench-profile:
	go test -bench=. -benchmem -cpuprofile=cpu.prof -memprofile=mem.prof -run=^$$ ./internal/benchmarks