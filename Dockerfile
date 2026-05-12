# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN go build -o tshongmart main.go

# Run stage
FROM alpine:latest

WORKDIR /app

# Copy binary and static files
COPY --from=builder /app/tshongmart .
COPY --from=builder /app/static ./static
COPY --from=builder /app/views ./views

EXPOSE 8080

CMD ["./tshongmart"]