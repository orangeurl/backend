FROM golang:alpine as builder

RUN mkdir /build

ADD . /build

WORKDIR /build

RUN go build -o main ./cmd/server

FROM alpine

RUN adduser -S -D -H -h /app appuser

# Create app directory and copy files as root first
RUN mkdir -p /app

COPY . /app

COPY --from=builder /build/main /app/

# Change ownership to appuser
RUN chown -R appuser /app

# Now switch to appuser
USER appuser

WORKDIR /app

EXPOSE 3000

CMD ["./main"]