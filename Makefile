.PHONY: generate fmt test vet run api worker check migrate-up migrate-status migrate-version migrate-down

generate:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 --config=api/oapi-codegen.yaml api/openapi.yaml

fmt:
	gofmt -w cmd internal migrations

test:
	go test ./...

vet:
	go vet ./...

run:
	go run ./cmd/api

api:
	@set -a; . ./.env; set +a; HTTP_ADDR=:3001 go run ./cmd/api

worker:
	@set -a; . ./.env; set +a; go run ./cmd/worker

migrate-up:
	go run ./cmd/migrate up

migrate-status:
	go run ./cmd/migrate status

migrate-version:
	go run ./cmd/migrate version

migrate-down:
	ALLOW_DESTRUCTIVE_MIGRATIONS=true go run ./cmd/migrate down

check: generate fmt test vet
