# build stage
# build stage
ARG revision
FROM thundernetes-src:$revision as builder

RUN go build -o initcontainer ./cmd/initcontainer/

# final stage
FROM alpine:3.12
WORKDIR /app
COPY --from=builder /workspace/initcontainer /app/
ENTRYPOINT ./initcontainer