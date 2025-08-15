# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application for Linux/AMD64 (Lambda requirement)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags lambda -ldflags="-w -s" -o bootstrap cmd/lambda/main.go

# Runtime stage
FROM public.ecr.aws/lambda/go:1

# Copy the binary from builder stage
COPY --from=builder /app/bootstrap ${LAMBDA_TASK_ROOT}

# Set the CMD to your handler
CMD [ "bootstrap" ]
