# syntax=docker/dockerfile:1

# Benchmark Dockerfile for Go version comparison
# Usage:
#   docker build --build-arg GO_VERSION=1.25.5 -t openfero-bench:go1.25.5 -f hack/benchmark.dockerfile .
#   docker build --build-arg GO_VERSION=1.26 -t openfero-bench:go1.26 -f hack/benchmark.dockerfile .

ARG GO_VERSION=1.25.5

FROM golang:${GO_VERSION}-bookworm AS builder

WORKDIR /workspace

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Pre-compile test binaries to validate the build
RUN go test -c -o /dev/null ./pkg/services/ 2>&1 || true
RUN go test -c -o /dev/null ./pkg/alertstore/memory/ 2>&1 || true

# Default entrypoint runs benchmarks
ENTRYPOINT ["go", "test"]
CMD ["-bench=.", "-benchmem", "-benchtime=5s", "-count=5", "-timeout=30m", "./pkg/services/", "./pkg/alertstore/memory/"]
