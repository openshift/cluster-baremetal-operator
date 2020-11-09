
# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Controller-gen tool
CONTROLLER_GEN ?= go run ./vendor/sigs.k8s.io/controller-tools/cmd/controller-gen
BIN_DIR := bin

IMAGES_JSON := /etc/cluster-baremetal-operator/images/images.json

ifeq (/,${HOME})
GOLANGCI_LINT_CACHE=/tmp/golangci-lint-cache/
else
GOLANGCI_LINT_CACHE=${HOME}/.cache/golangci-lint
endif

# Set VERBOSE to -v to make tests produce more output
VERBOSE ?= ""

all: cluster-baremetal-operator

# Run tests
test: generate lint manifests
	go test $(VERBOSE) ./... -coverprofile cover.out

# Alias for CI
unit: test

# Build cluster-baremetal-operator binary
cluster-baremetal-operator: generate lint
	go build -o bin/cluster-baremetal-operator main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate lint manifests
	go run ./main.go -images-json $(IMAGES_JSON)

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/cluster-baremetal-operator && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests:
	hack/gen-crd.sh

# Run go fmt against code
.PHONY: fmt
fmt:

# Run go lint against code
.PHONY: lint
lint: $(BIN_DIR)/golangci-lint
	GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) $(BIN_DIR)/golangci-lint run

$(BIN_DIR)/golangci-lint:
	go build -o "${BIN_DIR}/golangci-lint" ./vendor/github.com/golangci/golangci-lint/cmd/golangci-lint

# Run go vet against code
.PHONY: vet
vet: lint

# Generate code
.PHONY: generate
generate: $(BIN_DIR)/golangci-lint
	go generate -x ./...
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths="./..."
	GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) $(BIN_DIR)/golangci-lint run --fix

# Build the docker image
docker-build: test
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

vendor: lint
	go mod tidy
	go mod vendor
	go mod verify

.PHONY: generate-check
generate-check:
	./hack/generate.sh

.PHONY: bmh-crd
bmh-crd:
	./hack/get-bmh-crd.sh
