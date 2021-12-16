# build stage
ARG revision
FROM thundernetes-src:$revision as builder

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o thundernetes-nodeagent ./cmd/nodeagent

# Use distroless as minimal base image to package the binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/thundernetes-nodeagent .
USER 65532:65532
EXPOSE 56001
ENTRYPOINT ["/thundernetes-nodeagent"]
