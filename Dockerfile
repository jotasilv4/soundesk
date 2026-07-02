# Stage 1: Build the Go binary
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /app

# Copy dependency definition files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Compile the application as a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o soundesk ./cmd/api

# Stage 2: Create a minimal production image with alpine
FROM alpine:latest

# Install basic dynamic utilities
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the compiled binary from builder
COPY --from=builder /app/soundesk .

# Copy the static web files
COPY web/ ./web/

# Create directory for audio uploads
RUN mkdir -p audios

# Expose port
EXPOSE 8080

# Configure volume for persistent audio data
VOLUME ["/app/audios"]

# Define command to run the service
CMD ["./soundesk"]
