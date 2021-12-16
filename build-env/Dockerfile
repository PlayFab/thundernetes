# build stage
FROM golang:1.16 as builder

WORKDIR /workspace
# Copy the Go Module code
COPY . ./
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download
