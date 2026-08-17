# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder

RUN apk add --no-cache build-base git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=1
RUN go build -ldflags="-s -w" -o /out/mig ./cmd/mig

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/mig /usr/local/bin/mig

WORKDIR /workspace
VOLUME ["/workspace"]

ENTRYPOINT ["mig"]
CMD ["--help"]
