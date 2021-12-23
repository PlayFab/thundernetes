# build stage
FROM golang:1.17 as builder

WORKDIR /workspace
# Copy the Go Module code
COPY go.mod ./go.mod
COPY go.sum ./go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download
COPY cmd/ ./cmd
COPY e2e/ ./e2e
COPY pkg/ ./pkg

