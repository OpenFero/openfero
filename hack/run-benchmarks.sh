#!/usr/bin/env bash
# run-benchmarks.sh - Run Go benchmarks in isolated Docker containers for version comparison
#
# Usage:
#   ./hack/run-benchmarks.sh                    # Run with defaults (Go 1.25.6 vs 1.26)
#   ./hack/run-benchmarks.sh 1.25.6 1.26        # Specify versions explicitly
#   ./hack/run-benchmarks.sh --count 10         # Override benchmark count
#   ./hack/run-benchmarks.sh --benchtime 10s    # Override benchmark time

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
RESULTS_DIR="${PROJECT_DIR}/benchmark-results"

# Default versions
GO_OLD="${1:-1.25.6}"
GO_NEW="${2:-1.26}"

# Parse optional flags (after version args)
BENCH_COUNT="${BENCH_COUNT:-5}"
BENCH_TIME="${BENCH_TIME:-5s}"
BENCH_TIMEOUT="${BENCH_TIMEOUT:-30m}"
BENCH_PATTERN="${BENCH_PATTERN:-.}"

shift 2 2>/dev/null || true

while [[ $# -gt 0 ]]; do
	case "$1" in
	--count)
		BENCH_COUNT="$2"
		shift 2
		;;
	--benchtime)
		BENCH_TIME="$2"
		shift 2
		;;
	--timeout)
		BENCH_TIMEOUT="$2"
		shift 2
		;;
	--pattern)
		BENCH_PATTERN="$2"
		shift 2
		;;
	*)
		echo "Unknown argument: $1"
		exit 1
		;;
	esac
done

# Timestamp for this run
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RUN_DIR="${RESULTS_DIR}/${TIMESTAMP}"

echo "=============================================="
echo " OpenFero Go Benchmark: ${GO_OLD} vs ${GO_NEW}"
echo "=============================================="
echo ""
echo "Configuration:"
echo "  Benchmark pattern:  ${BENCH_PATTERN}"
echo "  Benchmark count:    ${BENCH_COUNT}"
echo "  Benchmark time:     ${BENCH_TIME}"
echo "  Timeout:            ${BENCH_TIMEOUT}"
echo "  Results directory:  ${RUN_DIR}"
echo ""

mkdir -p "${RUN_DIR}"

# Save run metadata
cat >"${RUN_DIR}/metadata.json" <<EOF
{
  "timestamp": "${TIMESTAMP}",
  "go_old": "${GO_OLD}",
  "go_new": "${GO_NEW}",
  "bench_count": ${BENCH_COUNT},
  "bench_time": "${BENCH_TIME}",
  "bench_pattern": "${BENCH_PATTERN}",
  "hostname": "$(hostname)",
  "cpu": "$(grep -m1 'model name' /proc/cpuinfo 2>/dev/null | cut -d: -f2 | xargs || sysctl -n machdep.cpu.brand_string 2>/dev/null || echo 'unknown')",
  "os": "$(uname -s -r)"
}
EOF

run_benchmark() {
	local go_version="$1"
	local label="$2"
	local output_file="${RUN_DIR}/${label}.txt"
	local image_tag="openfero-bench:go${go_version}"

	echo "--- Building benchmark image for Go ${go_version} ---"
	docker build \
		--build-arg "GO_VERSION=${go_version}" \
		-t "${image_tag}" \
		-f "${PROJECT_DIR}/hack/benchmark.dockerfile" \
		"${PROJECT_DIR}" \
		2>&1 | tail -5

	echo ""
	echo "--- Running benchmarks with Go ${go_version} ---"
	echo ""

	# Run benchmarks (no --run flag to skip unit tests, only benchmarks)
	docker run --rm \
		--cpus=2 \
		--memory=4g \
		"${image_tag}" \
		-bench="${BENCH_PATTERN}" \
		-benchmem \
		-benchtime="${BENCH_TIME}" \
		-count="${BENCH_COUNT}" \
		-timeout="${BENCH_TIMEOUT}" \
		-run='^$' \
		./pkg/services/ ./pkg/alertstore/memory/ \
		2>&1 | tee "${output_file}"

	echo ""
	echo "Results saved to: ${output_file}"
	echo ""
}

# Run benchmarks for both versions
run_benchmark "${GO_OLD}" "go-${GO_OLD}"
run_benchmark "${GO_NEW}" "go-${GO_NEW}"

echo "=============================================="
echo " Benchmark runs complete"
echo "=============================================="
echo ""

# Check if benchstat is available
if command -v benchstat &>/dev/null; then
	echo "--- benchstat comparison ---"
	echo ""
	benchstat "${RUN_DIR}/go-${GO_OLD}.txt" "${RUN_DIR}/go-${GO_NEW}.txt" | tee "${RUN_DIR}/comparison.txt"
	echo ""
	echo "Comparison saved to: ${RUN_DIR}/comparison.txt"
elif command -v go &>/dev/null; then
	echo "Installing benchstat..."
	go install golang.org/x/perf/cmd/benchstat@latest
	echo ""
	echo "--- benchstat comparison ---"
	echo ""
	benchstat "${RUN_DIR}/go-${GO_OLD}.txt" "${RUN_DIR}/go-${GO_NEW}.txt" | tee "${RUN_DIR}/comparison.txt"
	echo ""
	echo "Comparison saved to: ${RUN_DIR}/comparison.txt"
else
	echo "benchstat not available. Install with: go install golang.org/x/perf/cmd/benchstat@latest"
	echo "Then run manually:"
	echo "  benchstat ${RUN_DIR}/go-${GO_OLD}.txt ${RUN_DIR}/go-${GO_NEW}.txt"
fi

echo ""

# Run the Go analysis tool if available
if command -v go &>/dev/null; then
	echo "--- Running detailed analysis ---"
	echo ""
	go run "${PROJECT_DIR}/hack/benchanalyze/main.go" \
		-old "${RUN_DIR}/go-${GO_OLD}.txt" \
		-new "${RUN_DIR}/go-${GO_NEW}.txt" \
		-output "${RUN_DIR}" \
		-old-label "Go ${GO_OLD}" \
		-new-label "Go ${GO_NEW}"
	echo ""
fi

# Generate charts if comparison.json exists and a Python with matplotlib is available
if [[ -f "${RUN_DIR}/comparison.json" ]]; then
	PYTHON_CMD=""

	# Check project venv first, then system python3
	if [[ -x "${PROJECT_DIR}/.venv/bin/python" ]] && "${PROJECT_DIR}/.venv/bin/python" -c "import matplotlib" 2>/dev/null; then
		PYTHON_CMD="${PROJECT_DIR}/.venv/bin/python"
	elif command -v python3 &>/dev/null && python3 -c "import matplotlib" 2>/dev/null; then
		PYTHON_CMD="python3"
	fi

	if [[ -n "${PYTHON_CMD}" ]]; then
		echo "--- Generating visualization charts ---"
		echo ""
		"${PYTHON_CMD}" "${PROJECT_DIR}/hack/visualize-benchmarks.py" "${RUN_DIR}/comparison.json"
		echo ""
	else
		echo "matplotlib not installed. Install with: pip3 install matplotlib"
		echo "Or create a venv: python3 -m venv .venv && .venv/bin/pip install matplotlib"
		echo "Then run manually:"
		echo "  make benchmark-visualize JSON=${RUN_DIR}/comparison.json"
		echo ""
	fi
else
	echo "Skipping visualization: comparison.json not found"
	echo ""
fi

echo "=============================================="
echo " All results saved to: ${RUN_DIR}"
echo "=============================================="
echo ""
echo "Files:"
ls -laR "${RUN_DIR}/"
