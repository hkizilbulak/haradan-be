.PHONY: generate fmt test vet run check

generate:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 --config=api/oapi-codegen.yaml api/openapi.yaml

fmt:
	gofmt -w cmd internal

test:
	go test ./...

vet:
	go vet ./...

run:
	go run ./cmd/api

check: generate fmt test vet
