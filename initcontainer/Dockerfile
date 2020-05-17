# build stage
FROM golang:1.16-alpine as builder
ADD . /src
RUN cd /src && go build -o initcontainer

# final stage
FROM alpine:3.12
WORKDIR /app
COPY --from=builder /src/initcontainer /app/
ENTRYPOINT ./initcontainer