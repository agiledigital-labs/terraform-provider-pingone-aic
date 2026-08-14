PROVIDER_NAME := terraform-provider-pingone-aic
HOSTNAME      := registry.terraform.io
NAMESPACE     := agiledigital-labs
NAME          := pingone-aic
VERSION       := 0.1.0
OS_ARCH       := $(shell go env GOOS)_$(shell go env GOARCH)
INSTALL_PATH  := $(HOME)/.terraform.d/plugins/$(HOSTNAME)/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)

.PHONY: build install generate-cli test fmt tidy

build:
	go build -o bin/$(PROVIDER_NAME) .

generate-cli:
	go build -o bin/pingoneaic-tf ./cmd/generate

install: build
	mkdir -p "$(INSTALL_PATH)"
	cp bin/$(PROVIDER_NAME) "$(INSTALL_PATH)/"

test:
	go test ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy
