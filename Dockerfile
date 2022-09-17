# syntax=docker/dockerfile:1
FROM golang:1.18-alpine as builder
WORKDIR /build
ADD . /build
RUN apk add --no-cache gcc musl-dev linux-headers git make
RUN make build-for-docker

FROM alpine:latest
WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/rpc-endpoint /app/rpc-endpoint
ENV LISTEN_ADDR=":8080"
EXPOSE 8080
CMD ["/app/rpc-endpoint"]
