FROM mcr.microsoft.com/devcontainers/go:1.27-trixie

RUN rm -f /etc/apt/sources.list.d/yarn.list \
 && apt-get update \
 && apt-get install -y --no-install-recommends postgresql-client \
 && rm -rf /var/lib/apt/lists/*

RUN go install github.com/pressly/goose/v3/cmd/goose@latest \
 && go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest \
 && go install github.com/bootdotdev/bootdev@latest
