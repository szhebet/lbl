#!/bin/bash

echo "Setting up test environment..."

# Ensure PostgreSQL is running
sudo service postgresql status || sudo service postgresql start

# Wait a moment for PostgreSQL to be ready
sleep 2

# Run database initialization as postgres user
echo "Initializing database..."
sudo -u postgres psql -f db/scripts/init_db.sql

# Check if initialization succeeded
if [ $? -eq 0 ]; then
    echo "Database initialized successfully"
else
    echo "Failed to initialize database"
    exit 1
fi

# Start the Go application in background
echo "Starting Go application..."
go run src/main.go &
APP_PID=$!

# Give the app time to start
sleep 3

# Test the API endpoints
echo "Testing API endpoints..."
echo "GET /api/v1/books:"
curl -s http://localhost:8080/api/v1/books | jq .
echo ""

echo "GET /api/v1/books/1:"
curl -s http://localhost:8080/api/v1/books/1 | jq .
echo ""

# Test creating a book
echo "POST /api/v1/books (create book):"
curl -s -X POST http://localhost:8080/api/v1/books \
  -H "Content-Type: application/json" \
  -d '{"title":"API Test Book","author":"API Tester","published_year":2023,"genre":"Testing","description":"A book created via API test","language":"eng"}' | jq .
echo ""

# Test getting all books after creation
echo "GET /api/v1/books after creation:"
curl -s http://localhost:8080/api/v1/books | jq .
echo ""

# Clean up
kill $APP_PID 2>/dev/null || true
echo "Test completed"