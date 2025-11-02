#!/bin/bash

# Script to view all Redis URLs and optionally delete them

echo "================================================"
echo "     Redis URL Viewer and Cleaner"
echo "================================================"
echo ""

# Get all keys from Redis
echo "📋 Fetching all keys from Redis..."
REDIS_KEYS=$(docker-compose exec -T db redis-cli --raw KEYS "*" 2>/dev/null | grep -v '^$')

if [ -z "$REDIS_KEYS" ]; then
    echo "✅ No keys found in Redis"
    exit 0
fi

# Count total keys
TOTAL_KEYS=$(echo "$REDIS_KEYS" | wc -l)
echo "Found $TOTAL_KEYS keys in Redis"
echo ""

# Display all keys with their values
echo "================================================"
echo "Current Redis Entries:"
echo "================================================"

COUNT=1
while IFS= read -r key; do
    if [ -n "$key" ]; then
        # Get the value (URL)
        VALUE=$(docker-compose exec -T db redis-cli --raw GET "$key" 2>/dev/null)
        echo "$COUNT. Key: '$key'"
        echo "   Value: '$VALUE'"
        echo ""
        COUNT=$((COUNT + 1))
    fi
done <<< "$REDIS_KEYS"

echo "================================================"
echo ""

# Ask user what to do
read -p "Do you want to DELETE ALL these keys? (yes/no): " CONFIRM

if [ "$CONFIRM" = "yes" ] || [ "$CONFIRM" = "YES" ]; then
    echo ""
    echo "🗑️  Deleting all keys..."

    DELETED_COUNT=0
    while IFS= read -r key; do
        if [ -n "$key" ]; then
            docker-compose exec -T db redis-cli DEL "$key" > /dev/null 2>&1
            echo "   Deleted: $key"
            DELETED_COUNT=$((DELETED_COUNT + 1))
        fi
    done <<< "$REDIS_KEYS"

    echo ""
    echo "✅ Successfully deleted $DELETED_COUNT keys"
else
    echo ""
    echo "❌ Deletion cancelled. No keys were deleted."
fi

echo ""
echo "================================================"
echo "Done!"
echo "================================================"
