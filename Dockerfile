# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy source code first
COPY . .

# Download dependencies and generate go.sum
RUN go mod download && go mod tidy

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bot cmd/bot/main.go

# Final stage
FROM alpine:3.18

# Install ca-certificates
RUN apk add --no-cache ca-certificates

# Copy timezone data and CA certificates
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Set timezone
ENV TZ=Asia/Shanghai

# Copy the binary
COPY --from=builder /build/bot /bot

# Copy config files
COPY --from=builder /build/configs /configs

# Create volume for logs and knowledge
VOLUME ["/var/log/cf-ai-tgbot", "/knowledge"]

# Expose metrics port
EXPOSE 9090

# Run the bot
ENTRYPOINT ["/bot"]