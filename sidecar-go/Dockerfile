# build stage
FROM golang:1.16-alpine as builder
ADD . /src
RUN cd /src && go build -o thundernetes-sidecar-go

# final stage
FROM alpine:3.12
WORKDIR /app
COPY --from=builder /src/thundernetes-sidecar-go /app/
ENTRYPOINT ./thundernetes-sidecar-go