BINARY_NAME := keda-ingress-nginx-scaler
OUTPUT_DIR := bin

.PHONY: build
build:
	go build -o $(OUTPUT_DIR)/$(BINARY_NAME) ./cmd/main.go  # ✔ Tab (fixed)

.PHONY: docker
docker:
	docker build -t keda-ingress-nginx-scaler:v0.0.1 .

.PHONY: generate
generate:
	protoc proto/externalscaler.proto --go_out=plugins=grpc:pkg/api

redeploy: docker
	kind load docker-image keda-ingress-nginx-scaler:v0.0.1
	kubectl delete pod -n keda -l app=ingress-nginx-external-scaler

log:
	kubectl logs -n keda -l app=ingress-nginx-external-scaler