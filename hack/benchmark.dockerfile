# syntax=docker/dockerfile:1

# Benchmark Dockerfile for Go version comparison
# This is a development-only container for running Go benchmarks in isolation.
# Not intended for production use.
#
# Usage:
#   docker build --build-arg GO_VERSION=1.25.6 -t openfero-bench:go1.25.6 -f hack/benchmark.dockerfile .
#   docker build --build-arg GO_VERSION=1.26 -t openfero-bench:go1.26 -f hack/benchmark.dockerfile .

ARG GO_VERSION=1.25.6

FROM golang:${GO_VERSION}-bookworm

WORKDIR /workspace

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Pre-compile test binaries to validate the build
RUN go test -c -o /dev/null ./pkg/services/ 2>&1 || true && \
    go test -c -o /dev/null ./pkg/alertstore/memory/ 2>&1 || true

# Create non-root user for running benchmarks
RUN useradd --uid 10001 --no-create-home --shell /sbin/nologin benchuser && \
    chown -R benchuser:benchuser /workspace
USER 10001

# Healthcheck not applicable for short-lived benchmark containers
# trivy:ignore:DS004
HEALTHCHECK NONE

# Default entrypoint runs benchmarks
ENTRYPOINT ["go", "test"]
CMD ["-bench=.", "-benchmem", "-benchtime=5s", "-count=5", "-timeout=30m", "-run=^$", "./pkg/services/", "./pkg/alertstore/memory/"]
