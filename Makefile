# Chess Engine (ngn) - Makefile
# Provides ergonomic commands for development, testing, and building
.DEFAULT_GOAL := help

# Variables
BINARY_NAME=ngn
MAIN_FILE=main.go
BUILD_DIR=build
COVERAGE_FILE=coverage.out
RELEASE_VERSION?=0.2.0-rc.1
RELEASE_SOURCE_COMMIT?=$(shell git rev-parse HEAD)
RELEASE_VERIFICATION?=build/owned-verification/report.json
OWNED_RELEASE_OUT?=build/owned-$(RELEASE_VERSION)

.PHONY: build-owned-release
build-owned-release: ## Build verified Linux/Windows executables with owned NNUE defaults (WSL)
	python3 scripts/build_owned_release.py --version "$(RELEASE_VERSION)" \
		--source-commit "$(RELEASE_SOURCE_COMMIT)" --verification "$(RELEASE_VERIFICATION)" \
		--out "$(OWNED_RELEASE_OUT)"

# Default target
.PHONY: help
help: ## Display this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

# Build commands
.PHONY: build
build: ## Build the engine binary (Linux + Windows)
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"
	@echo "Building $(BINARY_NAME) for Windows..."
	GOOS=windows GOARCH=amd64 GOAMD64=v3 go build -o $(BUILD_DIR)/$(BINARY_NAME).exe $(MAIN_FILE)
	@echo "Built Windows executable: $(BUILD_DIR)/$(BINARY_NAME).exe"
	@if [ -w "/mnt/c/Users/behrlich/Desktop/ngn" ] && cp $(BUILD_DIR)/$(BINARY_NAME).exe "/mnt/c/Users/behrlich/Desktop/ngn/" 2>/dev/null; then \
		echo "Copied to C:\\\\Users\\\\behrlich\\\\Desktop\\\\ngn\\\\$(BINARY_NAME).exe for Arena"; \
	else \
		echo "Note: Copy $(BUILD_DIR)/$(BINARY_NAME).exe to your Arena Chess directory manually"; \
	fi

.PHONY: build-release
build-release: ## Build optimized release binary
	@echo "Building $(BINARY_NAME) (release)..."
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "Built release: $(BUILD_DIR)/$(BINARY_NAME)"

.PHONY: build-windows
build-windows: ## Build Windows executable for Arena Chess
	@echo "Building $(BINARY_NAME) for Windows..."
	@mkdir -p "/mnt/c/Users/behrlich/Desktop/ngn"
	GOOS=windows GOARCH=amd64 GOAMD64=v3 go build -o "/mnt/c/Users/behrlich/Desktop/ngn/$(BINARY_NAME).exe" $(MAIN_FILE)
	@echo "Built Windows executable: C:\\\\Users\\\\behrlich\\\\Desktop\\\\ngn\\\\$(BINARY_NAME).exe"
	@echo "Ready for Arena Chess! Point Arena to the .exe file above."

# Test commands
.PHONY: test
test: ## Run all tests with coverage (excludes slow comprehensive evaluation)
	@echo "Running tests with coverage..."
	@go test -v -short -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	@echo "Coverage report: $(COVERAGE_FILE)"

.PHONY: test-short
test-short: ## Run tests without coverage (faster)
	@echo "Running tests (short)..."
	@go test -v -short ./...

.PHONY: test-verbose
test-verbose: ## Run tests with verbose output
	@echo "Running tests (verbose)..."
	go test -v -race ./...

.PHONY: coverage
coverage: test ## Generate and view test coverage report
	go tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: eval
eval: ## Run comprehensive 2-minute engine evaluation with deep search
	@echo "Running comprehensive engine evaluation (2 minutes)..."
	@echo "⚠️  This test allows 10-15 seconds per position for meaningful depth analysis"
	@RUN_COMPREHENSIVE_EVAL=1 go test -v ./engine -run "TestComprehensiveEngineEvaluation" -timeout 180s

# Benchmark commands
.PHONY: bench
bench: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

.PHONY: bench-cpu
bench-cpu: ## Run benchmarks with CPU profiling
	@echo "Running benchmarks with CPU profiling..."
	go test -bench=. -benchmem -cpuprofile=cpu.prof ./...

# Code quality commands
.PHONY: format
format: ## Format all Go code
	@echo "Formatting Go code..."
	gofmt -s -w .
	go mod tidy
	@echo "Code formatted"

.PHONY: lint
lint: check-golangci-lint ## Run linting and formatting checks
	@echo "Running linter..."
	golangci-lint run ./...
	@echo "Linting complete"

.PHONY: lint-fix
lint-fix: check-golangci-lint ## Run linter and fix issues automatically
	@echo "Running linter with auto-fix..."
	golangci-lint run --fix ./...
	@echo "Linting and fixes complete"

# Tool installation
.PHONY: install-tools
install-tools: ## Install required development tools
	@echo "Installing development tools..."
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	}
	@echo "Development tools installed"

# Download Stockfish for testing
.PHONY: download-stockfish
download-stockfish: ## Download Stockfish binary for move validation testing
	@echo "Downloading Stockfish 16..."
	@if [ -f "stockfish/stockfish-ubuntu-x86-64-avx2" ]; then \
		echo "Stockfish already downloaded"; \
	else \
		wget -q --show-progress https://github.com/official-stockfish/Stockfish/releases/download/sf_16/stockfish-ubuntu-x86-64-avx2.tar; \
		tar -xf stockfish-ubuntu-x86-64-avx2.tar; \
		rm stockfish-ubuntu-x86-64-avx2.tar; \
		echo "Stockfish 16 downloaded successfully"; \
	fi

.PHONY: check-golangci-lint
check-golangci-lint: ## Check if golangci-lint is installed
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not found. Installing..."; \
		$(MAKE) install-tools; \
	}

# Clean commands
.PHONY: clean
clean: ## Clean build artifacts and temporary files
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f $(COVERAGE_FILE) coverage.html
	rm -f *.prof
	go clean -cache -testcache -modcache
	@echo "Cleaned"

.PHONY: clean-light
clean-light: ## Clean only build artifacts (keep caches)
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f $(COVERAGE_FILE) coverage.html
	rm -f *.prof
	@echo "Cleaned"

# Development workflow commands
.PHONY: dev
dev: format test build ## Run full development cycle: format, test, build

.PHONY: check
check: lint test ## Run code quality checks: lint and test

.PHONY: ci
ci: install-tools check build ## Run continuous integration pipeline

# Run commands
.PHONY: run
run: build ## Build and run the engine
	@echo "Running $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME)

# Test harness commands
.PHONY: test-harness
test-harness: build ## Run automated UCI test harness with tournament positions
	@echo "Running UCI test harness..."
	cd cmd/test-harness && go run main.go

.PHONY: benchmark
benchmark: build ## Run deterministic benchmark with fixed FEN corpus and regression detection
	@echo "Running benchmark harness..."
	cd cmd/benchmark && go run main.go

.PHONY: tactical-test
tactical-test: build ## Run tactical test suite with EPD positions
	@echo "Running tactical test suite..."
	cd cmd/tactical-test && go run main.go

.PHONY: smoke
smoke: build ## Fast battery-friendly check: tactical suite + 5-game smoke (~5 min)
	@mkdir -p $(BUILD_DIR)
	@echo "Building smoke + elo-assess..."
	@go build -o $(BUILD_DIR)/smoke ./cmd/smoke
	@go build -o $(BUILD_DIR)/elo-assess ./cmd/elo-assess
	@./$(BUILD_DIR)/smoke

.PHONY: smoke-tactical
smoke-tactical: build ## Tactical-only smoke (no game phase, ~10 sec)
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/smoke ./cmd/smoke
	@./$(BUILD_DIR)/smoke -skip-games

.PHONY: build-anchors
build-anchors: ## Clone + build the CCRL-rated Blunder opponent ladder into opponents/
	@scripts/build-anchors.sh

.PHONY: gauntlet
gauntlet: build ## Measure NGN's absolute CCRL rating vs the Blunder anchor ladder (ARGS="-games 24")
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/gauntlet ./cmd/gauntlet
	@./$(BUILD_DIR)/gauntlet $(ARGS)

.PHONY: spsa
spsa: build ## Joint SPSA tune of the search-param vector (ARGS="-iters 8000 -tc 10+0.1"); use ARGS="-selftest" to validate the math without games
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/spsa ./cmd/spsa
	@./$(BUILD_DIR)/spsa $(ARGS)

oracle: build ## Fast deterministic best-move-agreement pre-filter vs a labeled EPD (ARGS="-new build/ngn_a2 -base build/ngn -nodes 2000 -v")
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/oracle ./cmd/oracle
	@./$(BUILD_DIR)/oracle $(ARGS)

.PHONY: acpl
acpl: ## External eval ruler (run BEFORE SPRT-ing an eval change): ACPL vs Stockfish over the committed corpus (usage: make acpl BIN=build/ngn_new BASE=build/ngn)
	@scripts/acpl.sh $(or $(BIN),build/ngn) $(BASE)

.PHONY: perft
perft: ## Run perft validation tests for move generation correctness
	@echo "Running perft validation tests..."
	@go test -v ./engine -run ".*[Pp]erft.*" -timeout 60s

.PHONY: eval-regression
eval-regression: ## Run position evaluation regression tests to detect eval function changes
	@echo "Running evaluation regression tests..."
	@go test -v ./engine -run "TestEvaluationRegression|TestEvaluationConsistency" -timeout 30s

.PHONY: combined-report
combined-report: ## Generate combined performance and tactical assessment
	@echo "Generating combined engine assessment..."
	cd cmd/combined-report && go run main.go

.PHONY: simple-test
simple-test: build ## Run simple UCI tests with basic positions
	@echo "Running simple UCI tests..."
	cd cmd/simple-test && go run main.go

.PHONY: test-engine
test-engine: simple-test ## Alias for simple-test (quick engine validation)

.PHONY: test-enhanced
test-enhanced: ## Test enhanced search algorithms (null move, LMR, futility, etc.)
	@echo "Testing enhanced search algorithms..."
	@go test -v ./engine -run "TestNullMovePruning|TestLateMovReductions|TestFutilityPruning|TestCheckExtensions|TestMateDistancePruning" -timeout 30s

.PHONY: e2e
e2e: test-enhanced ## Run end-to-end tests for enhanced search features

# Debug and profiling
.PHONY: debug
debug: ## Build with debug symbols and run with delve
	@echo "Building with debug symbols..."
	go build -gcflags="all=-N -l" -o $(BUILD_DIR)/$(BINARY_NAME)-debug $(MAIN_FILE)
	@echo "Use 'dlv exec ./$(BUILD_DIR)/$(BINARY_NAME)-debug' to debug"

.PHONY: profile-mem
profile-mem: ## Run with memory profiling
	@echo "Building for memory profiling..."
	go build -o $(BUILD_DIR)/$(BINARY_NAME)-profile $(MAIN_FILE)
	@echo "Run with: GODEBUG=memprofile=mem.prof ./$(BUILD_DIR)/$(BINARY_NAME)-profile"

# Info commands
.PHONY: info
info: ## Display project information
	@echo "Project: Chess Engine (ngn)"
	@echo "Go version: $(shell go version)"
	@echo "Module: $(shell go list -m)"
	@echo "Dependencies:"
	@go list -m all

.PHONY: deps
deps: ## Display dependency information
	@echo "Direct dependencies:"
	@go list -m -f '{{if not .Indirect}}{{.Path}} {{.Version}}{{end}}' all
	@echo ""
	@echo "All dependencies:"
	@go list -m all

# PGN analysis commands
.PHONY: pgn-to-fens
pgn-to-fens: ## Extract FEN positions from PGN file (usage: make pgn-to-fens PGN=game.pgn)
	@if [ -z "$(PGN)" ]; then \
		echo "Usage: make pgn-to-fens PGN=path/to/game.pgn"; \
		exit 1; \
	fi
	@echo "Extracting FENs from $(PGN)..."
	@go run cmd/pgn_to_fens/main.go $(PGN)

.PHONY: game-fens
game-fens: ## Extract FEN positions from the specific game file
	@echo "Extracting FENs from the game..."
	@go run cmd/pgn_to_fens/simple_main.go the_game.pgn

.PHONY: analyze-move37
analyze-move37: ## Analyze move 37 blunder position at depths 1-10
	@echo "Analyzing move 37 blunder position..."
	@go run cmd/analyze_position/main.go -fen "r3kn1Q/1p2b3/p1n1b3/2ppq3/8/3B4/PPPP1PPP/R1B3K1 w q - 0 1" -depth 10

.PHONY: stockfish-move37
stockfish-move37: ## Analyze move 37 position with Stockfish at depths 1-20
	@echo "Analyzing move 37 with Stockfish..."
	@chmod +x analyze_with_stockfish.sh
	@./analyze_with_stockfish.sh "r3kn1Q/1p2b3/p1n1b3/2ppq3/8/3B4/PPPP1PPP/R1B3K1 w q - 0 1" 20

.PHONY: compare-move37
compare-move37: ## Compare NGN and Stockfish analysis of move 37 position
	@echo "=== NGN Analysis ==="
	@go run cmd/analyze_position/main.go -fen "r3kn1Q/1p2b3/p1n1b3/2ppq3/8/3B4/PPPP1PPP/R1B3K1 w q - 0 1" -depth 10
	@echo ""
	@echo "=== Stockfish Analysis ==="
	@chmod +x analyze_with_stockfish.sh
	@./analyze_with_stockfish.sh "r3kn1Q/1p2b3/p1n1b3/2ppq3/8/3B4/PPPP1PPP/R1B3K1 w q - 0 1" 10

.PHONY: trace-h8e5
trace-h8e5: ## Trace what happens after h8e5 on move 37 position
	@echo "Tracing h8e5 move..."
	@go run cmd/trace_move/main.go -fen "r3kn1Q/1p2b3/p1n1b3/2ppq3/8/3B4/PPPP1PPP/R1B3K1 w q - 0 1" -move "h8e5" -depth 7

.PHONY: trace-d3g6
trace-d3g6: ## Trace what happens after d3g6 on move 37 position
	@echo "Tracing d3g6 move..."
	@go run cmd/trace_move/main.go -fen "r3kn1Q/1p2b3/p1n1b3/2ppq3/8/3B4/PPPP1PPP/R1B3K1 w q - 0 1" -move "d3g6" -depth 7

.PHONY: eval-after-h8e5
eval-after-h8e5: ## Show evaluation breakdown after h8e5
	@echo "Evaluation after h8e5..."
	@go run cmd/eval_breakdown/main.go -fen "r3kn2/1p2b3/p1n1b3/2ppQ3/8/3B4/PPPP1PPP/R1B3K1 b q - 0 1"

.PHONY: eval-move37-start
eval-move37-start: ## Show evaluation breakdown of move 37 starting position
	@echo "Evaluation of move 37 starting position..."
	@go run cmd/eval_breakdown/main.go -fen "r3kn1Q/1p2b3/p1n1b3/2ppq3/8/3B4/PPPP1PPP/R1B3K1 w q - 0 1"

.PHONY: compare-evals
compare-evals: ## Compare NGN vs Stockfish evaluations on multiple test positions
	@chmod +x compare_evals.sh
	@./compare_evals.sh

.PHONY: analyze-position
analyze-position: ## Analyze any FEN position (usage: make analyze-position FEN="..." DEPTH=10)
	@if [ -z "$(FEN)" ]; then \
		echo "Usage: make analyze-position FEN='<fen string>' [DEPTH=10]"; \
		exit 1; \
	fi
	@echo "Analyzing position..."
	@go run cmd/analyze_position/main.go -fen "$(FEN)" -depth $(or $(DEPTH),10)
