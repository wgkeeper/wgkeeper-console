.PHONY: run build test clean migrate swagger swagger-install

run:
	go run cmd/server/main.go

build:
	go build -o server cmd/server/main.go

test:
	go test ./...

clean:
	rm -f server
	rm -f wgkeeper-console.db
	rm -rf data/
	rm -rf docs/

swagger-install:
	@echo "Installing swag..."
	@go install github.com/swaggo/swag/cmd/swag@latest
	@echo "Note: Make sure $(shell go env GOPATH)/bin is in your PATH"

swagger:
	@echo "Generating Swagger documentation..."
	@if command -v swag > /dev/null 2>&1; then \
		swag init -g cmd/server/main.go -o ./docs; \
	else \
		echo "swag not found in PATH, using go run..."; \
		go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/server/main.go -o ./docs; \
	fi
	@echo "Swagger docs generated in ./docs/"
	@echo "Don't forget to uncomment the docs import in cmd/server/main.go"

swagger-serve:
	@echo "Swagger UI available at http://localhost:8000/swagger/index.html"

migrate:
	@echo "Migrations are handled automatically by GORM AutoMigrate"
