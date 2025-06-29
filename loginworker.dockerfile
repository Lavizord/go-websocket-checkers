# Stage 1: Build the application
FROM golang:1.23.6-alpine AS builder

# Set the working directory
WORKDIR /app/

# Copy shared code
COPY go.mod go.sum /app/
COPY models /app/models
COPY logger /app/logger
COPY config /app/config
COPY postgrescli /app/postgrescli

COPY ./loginworker /app/

# Download dependencies and build the application
RUN go mod tidy 
RUN go mod download
RUN go build -o loginworker .

# Stage 2: Create the final image with only the binary
FROM alpine:latest
WORKDIR /root/

# Copy the built binary from the builder stage
COPY --from=builder /app/ .

# Set environment variables
ENV CONFIG_PATH=/root/config/config.json

# Run the loginworker service
CMD ["./loginworker"]