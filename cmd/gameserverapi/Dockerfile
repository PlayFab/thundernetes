# build stage
ARG revision
FROM thundernetes-src:$revision as builder

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o thundernetes-gameserver-api-service ./cmd/gameserverapi

# Use distroless as minimal base image to package the binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
ENV GIN_MODE release
WORKDIR /
COPY --from=builder /workspace/thundernetes-gameserver-api-service .
USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/thundernetes-gameserver-api-service"]
