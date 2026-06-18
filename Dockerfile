# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build

RUN apk update && apk add --no-cache \
    git \
    gcc \
    musl-dev

ARG GITHUB_TOKEN
RUN echo "machine github.com login porebric password ${GITHUB_TOKEN}" > /root/.netrc && chmod 600 /root/.netrc

ENV GOPRIVATE=github.com

RUN git config --global url."https://github.com/healthstep/".insteadOf "https://github.com/helthtech/"

WORKDIR /app

COPY . .

RUN go mod tidy
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/core-health ./cmd/core-health

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata curl openssl
RUN set -eu; \
    for u in \
      "https://gu.gosuslugi.ru/crt/rootca_ssl_rsa2022.cer" \
      "https://gu.gosuslugi.ru/crt/subca_ssl_rsa2022.cer"; do \
        f="/usr/local/share/ca-certificates/$(basename "$u" .cer).crt"; \
        if curl -fsSLk --connect-timeout 10 "$u" -o /tmp/cert.in; then \
            openssl x509 -inform PEM -in /tmp/cert.in -out "$f" 2>/dev/null \
              || openssl x509 -inform DER -in /tmp/cert.in -out "$f" 2>/dev/null || true; \
        fi; \
    done; \
    update-ca-certificates || true; \
    rm -f /tmp/cert.in || true
WORKDIR /app
COPY --from=build /out/core-health .
COPY --from=build /app/config/configs_keys.yml ./config/configs_keys.yml
EXPOSE 5002 9002
ENV APP_ENV=production
CMD ["./core-health"]
