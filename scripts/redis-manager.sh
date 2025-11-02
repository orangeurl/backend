#!/bin/bash

# Redis Manager - View and Delete Keys

echo "================================================"
echo "           Redis Manager"
echo "================================================"
echo ""

# Function to display all keys from a database
show_database() {
    local DB=$1
    local DB_NAME=$2

    echo "--- Database $DB: $DB_NAME ---"
    echo ""

    # Get all keys using SCAN
    KEYS=$(docker-compose exec -T db redis-cli -n $DB --scan 2>/dev/null)

    if [ -z "$KEYS" ]; then
        echo "  (empty)"
        echo ""
        return
    fi

    # Display each key with its value
    while IFS= read -r key; do
        if [ -n "$key" ]; then
            # Get the value
            VALUE=$(docker-compose exec -T db redis-cli -n $DB --raw GET "$key" 2>/dev/null)

            # Get TTL
            TTL=$(docker-compose exec -T db redis-cli -n $DB TTL "$key" 2>/dev/null | tr -d '\r')

            echo "  Key: $key"
            echo "  Value: $VALUE"
            if [ "$TTL" = "-1" ]; then
                echo "  TTL: Never expires"
            elif [ "$TTL" = "-2" ]; then
                echo "  TTL: Key doesn't exist"
            else
                echo "  TTL: ${TTL}s"
            fi
            echo ""
        fi
    done <<< "$KEYS"
}

# Display all databases
echo "📋 Listing all Redis keys and values..."
echo ""

show_database 0 "URL Mappings"
show_database 1 "Rate Limiting"

echo "================================================"
echo ""
echo "Options:"
echo "  1. Delete a specific key"
echo "  2. Clear database 0 (all URL mappings)"
echo "  3. Clear database 1 (all rate limits)"
echo "  4. Clear ALL databases"
echo "  5. Exit"
echo ""
read -p "Choose option (1-5): " OPTION

case $OPTION in
    1)
        echo ""
        read -p "Enter the key to delete: " KEY_TO_DELETE
        read -p "Which database? (0 or 1): " DB_NUM

        if [ "$DB_NUM" != "0" ] && [ "$DB_NUM" != "1" ]; then
            echo "❌ Invalid database number"
            exit 1
        fi

        # Check if key exists
        EXISTS=$(docker-compose exec -T db redis-cli -n $DB_NUM EXISTS "$KEY_TO_DELETE" 2>/dev/null | tr -d '\r')

        if [ "$EXISTS" = "0" ]; then
            echo "❌ Key '$KEY_TO_DELETE' not found in database $DB_NUM"
        else
            docker-compose exec -T db redis-cli -n $DB_NUM DEL "$KEY_TO_DELETE" > /dev/null 2>&1
            echo "✅ Deleted key: $KEY_TO_DELETE from database $DB_NUM"
        fi
        ;;

    2)
        echo ""
        read -p "⚠️  Delete ALL URL mappings? (yes/no): " CONFIRM
        if [ "$CONFIRM" = "yes" ]; then
            docker-compose exec -T db redis-cli -n 0 FLUSHDB > /dev/null 2>&1
            echo "✅ Database 0 cleared (all URL mappings deleted)"
        else
            echo "❌ Cancelled"
        fi
        ;;

    3)
        echo ""
        read -p "⚠️  Delete ALL rate limits? (yes/no): " CONFIRM
        if [ "$CONFIRM" = "yes" ]; then
            docker-compose exec -T db redis-cli -n 1 FLUSHDB > /dev/null 2>&1
            echo "✅ Database 1 cleared (all rate limits deleted)"
        else
            echo "❌ Cancelled"
        fi
        ;;

    4)
        echo ""
        read -p "⚠️  Delete EVERYTHING from Redis? (yes/no): " CONFIRM
        if [ "$CONFIRM" = "yes" ]; then
            docker-compose exec -T db redis-cli FLUSHALL > /dev/null 2>&1
            echo "✅ All Redis databases cleared"
        else
            echo "❌ Cancelled"
        fi
        ;;

    5)
        echo ""
        echo "👋 Goodbye!"
        exit 0
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
