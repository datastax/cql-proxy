FROM golang:1.16.7-alpine3.14 as builder

RUN apk add --no-cache git

# Disable cgo to remove gcc dependency
ENV CGO_ENABLED=0

ARG GITHUB_TOKEN
RUN git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"

WORKDIR /go/src/github.com/datastax/cql-proxy

# Grab the dependencies
ENV GOPRIVATE=github.com/datastax
COPY go.mod go.sum ./
RUN go mod download

# Copy in source
COPY . ./

# Build and install binary
RUN go install github.com/datastax/cql-proxy

# Run unit tests
RUN go test -short -v ./...

# a new clean image with just the binary
FROM alpine:3.14
RUN apk add --no-cache ca-certificates

# Copy in the binary
COPY --from=builder /go/bin/cql-proxy .
