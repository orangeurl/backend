FROM alpine:latest

# Install dependencies for PostgreSQL connection
RUN apk add --no-cache bash curl postgresql-client

# Download Goose binary directly
RUN curl -Lo /usr/local/bin/goose https://github.com/pressly/goose/releases/download/v3.26.0/goose.linux.amd64 \
    && chmod +x /usr/local/bin/goose

WORKDIR /migrations
ENTRYPOINT ["goose"]

