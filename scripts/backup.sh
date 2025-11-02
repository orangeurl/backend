#!/bin/bash

# OrangeURL Production Backup Script
# Backs up PostgreSQL and Redis databases
# Run daily via cron: 0 2 * * * /home/yusyii/orangeurl/backend/scripts/backup.sh

set -e

# Configuration
BACKEND_DIR="/home/yusyii/orangeurl/backend"
BACKUP_DIR="/home/yusyii/orangeurl/backups"
DATE=$(date +%Y%m%d_%H%M%S)
KEEP_DAYS=7

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}OrangeURL Backup Script${NC}"
echo -e "${GREEN}Started: $(date)${NC}"
echo -e "${GREEN}========================================${NC}"

# Create backup directory if it doesn't exist
mkdir -p "$BACKUP_DIR"

# Get Redis password from .env file
if [ -f "$BACKEND_DIR/.env" ]; then
    REDIS_PASSWORD=$(grep "^REDIS_PASSWORD=" "$BACKEND_DIR/.env" | cut -d '=' -f2)
else
    echo -e "${RED}ERROR: .env file not found at $BACKEND_DIR/.env${NC}"
    exit 1
fi

# Backup PostgreSQL
echo -e "${YELLOW}Backing up PostgreSQL...${NC}"
docker exec orangeurl_postgres pg_dump -U orangeurl orangeurl | gzip > "$BACKUP_DIR/postgres_$DATE.sql.gz"

if [ $? -eq 0 ]; then
    POSTGRES_SIZE=$(du -h "$BACKUP_DIR/postgres_$DATE.sql.gz" | cut -f1)
    echo -e "${GREEN}✓ PostgreSQL backup successful: postgres_$DATE.sql.gz ($POSTGRES_SIZE)${NC}"
else
    echo -e "${RED}✗ PostgreSQL backup failed${NC}"
    exit 1
fi

# Backup Redis
echo -e "${YELLOW}Backing up Redis...${NC}"

# Trigger Redis save
if [ -n "$REDIS_PASSWORD" ]; then
    docker exec backend-db-1 redis-cli --no-auth-warning -a "$REDIS_PASSWORD" SAVE
else
    docker exec backend-db-1 redis-cli SAVE
fi

# Copy Redis dump file
if [ -f "$BACKEND_DIR/data/dump.rdb" ]; then
    cp "$BACKEND_DIR/data/dump.rdb" "$BACKUP_DIR/redis_$DATE.rdb"
    REDIS_SIZE=$(du -h "$BACKUP_DIR/redis_$DATE.rdb" | cut -f1)
    echo -e "${GREEN}✓ Redis backup successful: redis_$DATE.rdb ($REDIS_SIZE)${NC}"
else
    echo -e "${RED}✗ Redis backup failed: dump.rdb not found${NC}"
fi

# Clean up old backups (keep last 7 days)
echo -e "${YELLOW}Cleaning up old backups (keeping last $KEEP_DAYS days)...${NC}"

DELETED_COUNT=0
for file in $(find "$BACKUP_DIR" -name "postgres_*.sql.gz" -mtime +$KEEP_DAYS); do
    rm "$file"
    DELETED_COUNT=$((DELETED_COUNT + 1))
    echo -e "  Deleted: $(basename $file)"
done

for file in $(find "$BACKUP_DIR" -name "redis_*.rdb" -mtime +$KEEP_DAYS); do
    rm "$file"
    DELETED_COUNT=$((DELETED_COUNT + 1))
    echo -e "  Deleted: $(basename $file)"
done

if [ $DELETED_COUNT -eq 0 ]; then
    echo -e "${GREEN}✓ No old backups to delete${NC}"
else
    echo -e "${GREEN}✓ Deleted $DELETED_COUNT old backup(s)${NC}"
fi

# Show backup summary
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Backup Summary${NC}"
echo -e "${GREEN}========================================${NC}"
echo -e "Backup location: $BACKUP_DIR"
echo -e "Total backups: $(ls -1 "$BACKUP_DIR" | wc -l) files"
echo -e "Disk usage: $(du -sh "$BACKUP_DIR" | cut -f1)"
echo -e ""
echo -e "Latest backups:"
ls -lht "$BACKUP_DIR" | head -5
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Backup completed successfully!${NC}"
echo -e "${GREEN}Finished: $(date)${NC}"
echo -e "${GREEN}========================================${NC}"
