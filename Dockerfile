# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first for dependency caching
COPY go.mod ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build args for versioning
ARG VERSION=development
ARG BUILD_DATE=unknown
ARG GIT_COMMIT=unknown

# Build the Go binary statically with version info
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=$VERSION -X main.buildDate=$BUILD_DATE -X main.gitCommit=$GIT_COMMIT" -o cookie-bearer cookie-bearer.go

# Final image
FROM scratch

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/cookie-bearer .

ENTRYPOINT ["./cookie-bearer"]
