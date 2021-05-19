all: fmt vet lint test

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Run the linter against code
lint:
	golangci-lint run -v

# Run tests
test:
	@go test ./... -coverprofile cover.out
