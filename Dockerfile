# Use the lightweight Go Alpine image
FROM golang:1.27-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy the module files and download the fsnotify/rate-limiting dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy your Go files and the config.json file
COPY . .

# Build the final compiled Go binary
RUN go build -o api-gateway .

# Expose port 8080 so traffic can reach the Gateway
EXPOSE 8080

# Run the compiled binary when the container starts
CMD ["./api-gateway"]