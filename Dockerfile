# syntax=docker/dockerfile:1

# Stage 1: Build binary
FROM golang:alpine AS builder

WORKDIR /src

# Install CA certificates for SSL connections
RUN apk add --no-cache ca-certificates git

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/contractlens ./cmd/contractlens

# Stage 2: Minimal runtime image
FROM alpine:3.21

RUN apk add --no-cache ca-certificates && \
    addgroup -S contractlens && \
    adduser -S -G contractlens -u 10001 contractlens

USER 10001:10001

COPY --from=builder /bin/contractlens /usr/local/bin/contractlens

WORKDIR /workspace

ENTRYPOINT ["/usr/local/bin/contractlens"]
CMD ["help"]
