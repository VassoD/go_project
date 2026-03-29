build:
	go build -o vtc-service .

# Run directly with Go
run:
	go run .

# Run all tests
test:
	go test ./...

# Run tests with verbose output
test-verbose:
	go test ./... -v

# Build Docker image
docker-build:
	docker build -t vtc-service .

# Run in Docker
docker-run:
	docker run --rm -p 8080:8080 vtc-service