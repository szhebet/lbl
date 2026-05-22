#!/bin/bash

echo "Setting up debug test environment..."

# Ensure PostgreSQL is running
sudo service postgresql status || sudo service postgresql start

# Wait a moment for PostgreSQL to be ready
sleep 2

# Run database initialization as postgres user
echo "Initializing database..."
sudo -u postgres psql -f db/scripts/init_db.sql

# Check if initialization succeeded
if [ $? -ne 0 ]; then
    echo "Failed to initialize database"
    exit 1
fi

# Test database connection directly
echo "Testing direct database connection..."
sudo -u postgres psql -d library -c "SELECT COUNT(*) FROM works;" || echo "Direct DB query failed"
sudo -u postgres psql -d library -c "SELECT COUNT(*) FROM editions;" || echo "Direct DB query failed"

# Start the Go application in background with logging
echo "Starting Go application with logging..."
go run src/main.go 2>&1 &
APP_PID=$!
echo "Application PID: $APP_PID"

# Give the app time to start
sleep 3

# Check if the application is listening on port 8080
echo "Checking if application is listening on port 8080..."
netstat -tlnp | grep :8080 || echo "Port 8080 not found in listening ports"

# Test the API endpoints with verbose output
echo "Testing API endpoints with verbose output..."
echo "GET /api/v1/books:"
curl -v http://localhost:8080/api/v1/books 2>&1 | head -20
echo ""

# Clean up
kill $APP_PID 2>/dev/null || true
echo "Debug test completed"
EOF