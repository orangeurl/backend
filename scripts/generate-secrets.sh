#!/bin/bash

# Script to generate secure random passwords for production

set -e

echo "🔐 Generating secure random secrets..."
echo ""

generate_password() {
    openssl rand -base64 32 | tr -d "=+/" | cut -c1-32
}

echo "Copy these values to your .env file:"
echo "=========================================="
echo ""
echo "# PostgreSQL Password"
echo "POSTGRES_PASSWORD=$(generate_password)"
echo ""
echo "# Redis Password"
echo "REDIS_PASSWORD=$(generate_password)"
echo ""
echo "=========================================="
echo ""
echo "⚠️  Keep these secrets secure and never commit them to git!"
