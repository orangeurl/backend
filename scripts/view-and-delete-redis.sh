#!/bin/bash

# Script to view all Redis URLs and optionally delete them

echo "================================================"
echo "     Redis URL Viewer and Cleaner"
echo "================================================"
echo ""

# Function to get all keys from a specific database
get_keys_from_db() {
    local DB=$1
    echo "📋 Checking Redis database $DB..."

    # Select database and get keys
    KEYS=$(docker-compose exec -T db redis-cli -n $DB --scan 2>/dev/null)

    if [ -n "$KEYS" ]; then
        echo "   Found keys in database $DB"
        echo "$KEYS"
    fi
}

# Check both database 0 and 1 (your app uses both)
echo "Scanning all Redis databases..."
echo ""

ALL_KEYS=""
for DB in 0 1; do
    DB_KEYS=$(get_keys_from_db $DB)
    if [ -n "$DB_KEYS" ]; then
        ALL_KEYS="${ALL_KEYS}${DB_KEYS}"$'\n'
    fi
done

# Remove empty lines and count
REDIS_KEYS=$(echo "$ALL_KEYS" | grep -v '^$' | grep -v 'Checking Redis' | grep -v 'Found keys')

if [ -z "$REDIS_KEYS" ]; then
    echo "✅ No keys found in Redis"
    exit 0
fi

# Count total keys
TOTAL_KEYS=$(echo "$REDIS_KEYS" | wc -l)
echo ""
echo "Found $TOTAL_KEYS keys total across all databases"
echo ""

# Display all keys with their values
echo "================================================"
echo "Current Redis Entries:"
echo "================================================"

COUNT=1
# Try database 0 first, then database 1
for DB in 0 1; do
    echo ""
    echo "--- Database $DB ---"
    echo ""

    DB_KEYS=$(docker-compose exec -T db redis-cli -n $DB --scan 2>/dev/null)

    while IFS= read -r key; do
        if [ -n "$key" ]; then
            # Get the value (URL) from the specific database
            VALUE=$(docker-compose exec -T db redis-cli -n $DB --raw GET "$key" 2>/dev/null)

            # Get key type
            KEY_TYPE=$(docker-compose exec -T db redis-cli -n $DB TYPE "$key" 2>/dev/null | tr -d '\r')

            echo "$COUNT. Key: '$key' (Type: $KEY_TYPE)"
            if [ "$KEY_TYPE" = "string" ]; then
                echo "   Value: '$VALUE'"
            else
                echo "   (Non-string type)"
            fi
            echo ""
            COUNT=$((COUNT + 1))
        fi
    done <<< "$DB_KEYS"
done

echo "================================================"
echo ""

# Ask user what to do
echo "================================================"
echo "Options:"
echo "  1. Delete ALL keys from ALL databases (FLUSHALL)"
echo "  2. Delete keys from database 0 only (URL cache)"
echo "  3. Delete keys from database 1 only (rate limiting)"
echo "  4. Delete specific keys one by one"
echo "  5. Cancel"
echo "================================================"
echo ""
read -p "Choose option (1-5): " OPTION

case $OPTION in
    1)
        echo ""
        read -p "⚠️  This will DELETE ALL DATA from Redis. Are you sure? (yes/no): " CONFIRM
        if [ "$CONFIRM" = "yes" ]; then
            echo "🗑️  Flushing all Redis databases..."
            docker-compose exec -T db redis-cli FLUSHALL > /dev/null 2>&1
            echo "✅ All Redis data deleted"
        else
            echo "❌ Cancelled"
        fi
        ;;
    2)
        echo ""
        read -p "Delete all keys from database 0 (URL cache)? (yes/no): " CONFIRM
        if [ "$CONFIRM" = "yes" ]; then
            echo "🗑️  Deleting database 0..."
            docker-compose exec -T db redis-cli -n 0 FLUSHDB > /dev/null 2>&1
            echo "✅ Database 0 cleared"
        else
            echo "❌ Cancelled"
        fi
        ;;
    3)
        echo ""
        read -p "Delete all keys from database 1 (rate limiting)? (yes/no): " CONFIRM
        if [ "$CONFIRM" = "yes" ]; then
            echo "🗑️  Deleting database 1..."
            docker-compose exec -T db redis-cli -n 1 FLUSHDB > /dev/null 2>&1
            echo "✅ Database 1 cleared"
        else
            echo "❌ Cancelled"
        fi
        ;;
    4)
        echo ""
        echo "Deleting keys one by one..."
        for DB in 0 1; do
            DB_KEYS=$(docker-compose exec -T db redis-cli -n $DB --scan 2>/dev/null)
            while IFS= read -r key; do
                if [ -n "$key" ]; then
                    VALUE=$(docker-compose exec -T db redis-cli -n $DB --raw GET "$key" 2>/dev/null)
                    echo ""
                    echo "Key: '$key' (DB: $DB)"
                    echo "Value: '$VALUE'"
                    read -p "Delete this key? (y/n): " DELETE_KEY
                    if [ "$DELETE_KEY" = "y" ] || [ "$DELETE_KEY" = "Y" ]; then
                        docker-compose exec -T db redis-cli -n $DB DEL "$key" > /dev/null 2>&1
                        echo "✅ Deleted"
                    else
                        echo "⏭️  Skipped"
                    fi
                fi
            done <<< "$DB_KEYS"
        done
        ;;
    5)
        echo ""
        echo "❌ Cancelled. No changes made."
        ;;
    *)
        echo ""
        echo "❌ Invalid option"
        ;;
esac

echo ""
echo "================================================"
echo "Done!"
echo "================================================"
