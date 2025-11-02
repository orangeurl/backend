#!/bin/bash

# Script to remove Redis entries for URLs that no longer exist in Postgres

echo "🔍 Finding URLs in Redis that don't exist in Postgres..."

# Get all keys from Redis (database 0)
echo "Fetching all keys from Redis..."
REDIS_KEYS=$(docker-compose exec -T db redis-cli KEYS "*" 2>/dev/null)

if [ -z "$REDIS_KEYS" ]; then
    echo "✅ No keys found in Redis"
    exit 0
fi

# Connect to Postgres and check each key
DELETED_COUNT=0
CHECKED_COUNT=0

while IFS= read -r key; do
    # Skip counter and other non-URL keys
    if [[ "$key" == "counter" ]] || [[ "$key" == *":"* ]]; then
        continue
    fi

    CHECKED_COUNT=$((CHECKED_COUNT + 1))

    # Check if URL exists in Postgres
    EXISTS=$(docker-compose exec -T postgres psql -U orangeurl -d orangeurl -tAc "SELECT COUNT(*) FROM urls WHERE short_id = '$key' AND is_active = TRUE;")

    if [ "$EXISTS" -eq 0 ]; then
        echo "🗑️  Deleting orphaned key: $key"
        docker-compose exec -T db redis-cli DEL "$key" > /dev/null
        DELETED_COUNT=$((DELETED_COUNT + 1))
    else
        echo "✅ Valid key: $key"
    fi
done <<< "$REDIS_KEYS"

echo ""
echo "📊 Summary:"
echo "   Checked: $CHECKED_COUNT keys"
echo "   Deleted: $DELETED_COUNT orphaned keys"
echo "   Remaining: $((CHECKED_COUNT - DELETED_COUNT)) valid keys"
