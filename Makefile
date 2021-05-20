all: fmt vet lint doc

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Run the linter against code
lint:
	golangci-lint run -v

# Generate helm chart documentation
doc:
	@docker run --rm --volume "${PWD}/helm/node-drainer:/helm-docs" jnorwood/helm-docs:latest -s file