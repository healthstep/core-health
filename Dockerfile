# Build with monorepo layout (like public-api): from repo root use
#   docker build -f core-health/Dockerfile .
# with ./core-health and ./creteria_parser present. See .github/workflows/docker-image.yml
FROM golang:1.25-alpine AS builder
WORKDIR /build/core-health
RUN apk add --no-cache git

COPY core-health/go.mod core-health/go.sum ./
COPY creteria_parser /build/creteria_parser
RUN go mod edit -replace github.com/porebric/creteria_parser=/build/creteria_parser
RUN go mod download

COPY core-health/ ./
RUN go mod edit -replace github.com/porebric/creteria_parser=/build/creteria_parser
RUN CGO_ENABLED=0 go build -o /app/core-health ./cmd/core-health

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/core-health .
COPY --from=builder /build/core-health/config/configs_keys.yml ./config/configs_keys.yml
EXPOSE 5002 9002
CMD ["./core-health"]
