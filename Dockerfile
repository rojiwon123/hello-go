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
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o main cmd/main.go

# Runtime stage
FROM public.ecr.aws/lambda/provided:al2 AS runner

# Copy the binary from builder stage
COPY --from=builder /app/main ${LAMBDA_TASK_ROOT}

# Set the CMD to your handler
CMD [ "main" ]
