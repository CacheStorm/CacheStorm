#!/bin/bash
# Post-install script for CacheStorm

set -e

# Create cacheStorm user if not exists
if ! id -u cachestorm &>/dev/null; then
    useradd -r -s /bin/false cachestorm
fi

# Create data directory
mkdir -p /var/lib/cachestorm
chown cachestorm:cachestorm /var/lib/cachestorm

# Create log directory
mkdir -p /var/log/cachestorm
chown cachestorm:cachestorm /var/log/cachestorm

# Set permissions
chmod 755 /usr/bin/cachestorm

echo "CacheStorm installed successfully!"
echo "Start with: systemctl start cachestorm"
echo "Enable with: systemctl enable cachestorm"
