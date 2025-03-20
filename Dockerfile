FROM golang:1.18 as builder

# Disable cgo to remove gcc dependency
ENV CGO_ENABLED=0

WORKDIR /go/src/cql-proxy

# Grab the dependencies
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

EXPOSE 9042

# Copy in the binary
COPY --from=builder /go/bin/cql-proxy .

ENTRYPOINT ["/cql-proxy"]
