# build stage
ARG revision
ENV UDP_SERVER_PORT=3075
ENV METRICS_SERVER_PORT=8080
FROM thundernetes-src:$revision as builder

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o thundernetes-qosserver ./cmd/qosserver

# Use distroless as minimal base image to package the binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/thundernetes-qosserver .
USER 65532:65532
EXPOSE ${UDP_SERVER_PORT}/udp
EXPOSE ${METRICS_SERVER_PORT}/tcp
ENTRYPOINT ["/thundernetes-qosserver"]
