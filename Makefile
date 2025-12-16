SHELL := /bin/bash

BIN ?= lbctl
OUT_DIR ?= bin

.PHONY: build test clean docker-test docker-build docker-run docker-artifact \
        deploy-test deploy-test-dry deploy-test-full
IMAGE_TEST ?= lbctl-test
IMAGE_BUILD ?= lbctl-build
IMAGE_RUN ?= lbctl-alma
IMAGE_DEPLOY_TEST ?= lbctl-deploy-test
DOCKER_ARGS ?= --help

build:
	@mkdir -p "$(OUT_DIR)"
	go build -o "$(OUT_DIR)/$(BIN)" ./cmd/lbctl

test:
	go test ./... -v

clean:
	rm -rf "$(OUT_DIR)"

docker-test:
	docker build --target lbctl-test -t "$(IMAGE_TEST)" .
	docker run --rm "$(IMAGE_TEST)" make test

docker-build:
	docker build --target lbctl-runtime -t "$(IMAGE_RUN)" .

docker-run: docker-build
	docker run --rm "$(IMAGE_RUN)" $(DOCKER_ARGS)

docker-artifact:
	docker build --target lbctl-build -t "$(IMAGE_BUILD)" .
	@id="$$(docker create "$(IMAGE_BUILD)")"; \
		docker cp "$$id:/out/lbctl" "./$(BIN)"; \
		docker rm -f "$$id" >/dev/null; \
		chmod +x "./$(BIN)"; \
		echo "Wrote ./$(BIN)"

# Deploy script testing (local Docker)
deploy-test-build:
	docker build -f scripts/Dockerfile.deploy-test -t "$(IMAGE_DEPLOY_TEST)" .

deploy-test-dry: deploy-test-build
	docker run --rm "$(IMAGE_DEPLOY_TEST)" /scripts/deploy.sh --dry-run

deploy-test-full: deploy-test-build
	docker run --privileged --rm "$(IMAGE_DEPLOY_TEST)" /scripts/deploy.sh --skip-frr-start

deploy-test: deploy-test-dry
	@echo ""
	@echo "Dry-run passed. Run 'make deploy-test-full' for full install test."
