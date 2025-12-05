# Build stage for Lambda with CGO (required for SQLite/lrcat-go)
FROM public.ecr.aws/sam/build-provided.al2023:latest AS builder

# Install Go and SQLite development libraries
RUN dnf install -y golang sqlite-devel gcc

WORKDIR /build

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with CGO enabled
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o bootstrap main.go

# Final stage - just the binary
FROM scratch
COPY --from=builder /build/bootstrap /bootstrap
