# build stage
FROM golang:1.16 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod ./
COPY go.sum ./
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY *.go ./

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o thundernetes-nodeagent 

# Use distroless as minimal base image to package the sidecar binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/thundernetes-nodeagent .
USER 65532:65532
EXPOSE 56001
ENTRYPOINT ["/thundernetes-nodeagent"]
