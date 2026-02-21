#!/bin/bash
# Pre-remove script for CacheStorm

set -e

# Stop service if running
if systemctl is-active --quiet cachestorm; then
    systemctl stop cachestorm
fi

# Disable service
if systemctl is-enabled --quiet cachestorm 2>/dev/null; then
    systemctl disable cachestorm
fi

echo "CacheStorm stopped and disabled"
