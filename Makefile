SHELL := /bin/bash

.PHONY: all build test images import up down demo clean

all: build test

build:
	@echo "==> Building Go services..."
	cd services/utils-mcp && CGO_ENABLED=0 go build -o ../../bin/utils-mcp ./cmd/utils-mcp
	cd services/woo-mcp && CGO_ENABLED=0 go build -o ../../bin/woo-mcp ./cmd/woo-mcp
	cd services/gateway && CGO_ENABLED=0 go build -o ../../bin/gateway ./cmd/gateway
	@echo "==> Building Dashboard frontend..."
	cd services/dashboard && npm run build
	@echo "==> Build complete."

test:
	@echo "==> Running unit tests..."
	cd services/utils-mcp && go test -v ./...
	cd services/gateway && go test -v ./internal/crypto ./internal/aggregate
	@echo "==> All unit tests passed!"

images:
	@echo "==> Building Docker images..."
	docker build -t urumi/utils-mcp:latest services/utils-mcp
	docker build -t urumi/woo-mcp:latest services/woo-mcp
	docker build -t urumi/gateway:latest services/gateway
	docker build -t urumi/dashboard:latest services/dashboard
	docker build -t urumi/wordpress-custom:latest deploy/wordpress
	@echo "==> Docker images built successfully."

import:
	@echo "==> Importing Docker images into k3d cluster 'urumi'..."
	k3d image import -c urumi \
		urumi/utils-mcp:latest \
		urumi/woo-mcp:latest \
		urumi/gateway:latest \
		urumi/dashboard:latest \
		urumi/wordpress-custom:latest
	@echo "==> Images imported into k3d cluster."

up:
	@echo "==> Deploying Store layer into 'store' namespace..."
	helm dependency update charts/store
	helm upgrade --install store charts/store -n store --create-namespace \
		-f charts/store/values.yaml -f charts/store/values-local.yaml --wait --timeout 5m
	@echo "==> Deploying Platform layer into 'platform' namespace..."
	helm dependency update charts/platform
	helm upgrade --install platform charts/platform -n platform --create-namespace \
		-f charts/platform/values.yaml -f charts/platform/values-local.yaml --wait --timeout 5m
	@echo "==> Urumi Gateway and Store deployed successfully!"
	@echo "Store:     http://store.localtest.me"
	@echo "Dashboard: http://app.localtest.me"
	@echo "MCP Door:  http://app.localtest.me/mcp"

down:
	@echo "==> Tearing down releases..."
	helm uninstall platform -n platform || true
	helm uninstall store -n store || true
	@echo "==> Releases uninstalled."

demo:
	@echo "==> Executing Definition-of-Done End-to-End Verification..."
	bash deploy/demo.sh
