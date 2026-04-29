FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install make and nodejs for frontend build
RUN apk add --no-cache make nodejs npm git

# Copy dependencies first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the code
COPY . .

# Build the frontend and backend
RUN make build

FROM alpine:latest

# Install CA certificates for proxying
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy the built binary
COPY --from=builder /app/bin/notion-manager /app/notion-manager
COPY --from=builder /app/bin/notion-manager-register /app/notion-manager-register

# Create accounts directory
RUN mkdir -p /app/accounts

# Set execution permissions
RUN chmod +x /app/notion-manager /app/notion-manager-register

# Environment variables
ENV NOTION_PORT=8081

# Expose port
EXPOSE 8081

CMD ["/app/notion-manager"]
