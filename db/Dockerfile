FROM redis:alpine

# Copy custom redis.conf if needed
COPY redis.conf /usr/local/etc/redis/redis.conf

# Run Redis with authentication
CMD ["redis-server", "/usr/local/etc/redis/redis.conf"]

EXPOSE 6379