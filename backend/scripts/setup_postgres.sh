#!/bin/bash
set -e

# Start PostgreSQL service
service postgresql start

# Configure postgresql.conf to listen on all network interfaces
PG_CONF="/etc/postgresql/16/main/postgresql.conf"
PG_HBA="/etc/postgresql/16/main/pg_hba.conf"

sed -i "s/#listen_addresses = 'localhost'/listen_addresses = '*'/" "$PG_CONF"

# Allow password and local trust authentication for development
if ! grep -q "host all all 0.0.0.0/0 md5" "$PG_HBA"; then
    echo "host all all 0.0.0.0/0 md5" >> "$PG_HBA"
    echo "host all all ::/0 md5" >> "$PG_HBA"
fi

# Reload/restart PostgreSQL
service postgresql restart

# Set password for postgres and create eva_media database
su - postgres -c "psql -c \"ALTER USER postgres WITH PASSWORD 'postgres';\""
su - postgres -c "psql -tc \"SELECT 1 FROM pg_database WHERE datname = 'eva_media'\" | grep -q 1 || psql -c \"CREATE DATABASE eva_media OWNER postgres;\""

echo "PostgreSQL setup complete. Status:"
service postgresql status
