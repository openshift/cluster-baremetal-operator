ifeq (/,${HOME})
GOLANGCI_LINT_CACHE=/tmp/golangci-lint-cache/
else
GOLANGCI_LINT_CACHE=${HOME}/.cache/golangci-lint
endif

# Image URL to use all building/pushing image targets
IMG ?= controller:latest

CONTROLLER_GEN ?= go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go
CRD_OPTIONS="crd:trivialVersions=true,crdVersions=v1"
GOLANGCI_LINT ?= GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) go run vendor/github.com/golangci/golangci-lint/cmd/golangci-lint/main.go
KUSTOMIZE ?= go run sigs.k8s.io/kustomize/kustomize/v3
MANIFEST_PROFILE ?= default
TMP_DIR := $(shell mktemp -d -t manifests-$(date +%Y-%m-%d-%H-%M-%S)-XXXXXXXXXX)

IMAGES_JSON ?= /etc/cluster-baremetal-operator/images/images.json

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
	kubectl apply -f manifests/0000_31_cluster-baremetal-operator_02_metal3provisioning.crd.yaml

# Uninstall CRDs from a cluster
uninstall: manifests
	kubectl delete -f manifests/0000_31_cluster-baremetal-operator_02_metal3provisioning.crd.yaml

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: generate
	cd config/cluster-baremetal-operator && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/profiles/$(MANIFEST_PROFILE) | kubectl apply -f -

# this is to just get the order right
RBAC_LIST = rbac.authorization.k8s.io_v1_role_cluster-baremetal-operator.yaml \
	rbac.authorization.k8s.io_v1_clusterrole_cluster-baremetal-operator.yaml \
	rbac.authorization.k8s.io_v1_rolebinding_cluster-baremetal-operator.yaml \
	rbac.authorization.k8s.io_v1_clusterrolebinding_cluster-baremetal-operator.yaml

PROMETHEUS_RBAC_LIST = rbac.authorization.k8s.io_v1_rolebinding_prometheus-k8s-cluster-baremetal-operator.yaml \
	rbac.authorization.k8s.io_v1_role_prometheus-k8s-cluster-baremetal-operator.yaml

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: generate
	$(KUSTOMIZE) build config/profiles/$(MANIFEST_PROFILE) -o $(TMP_DIR)/
	ls $(TMP_DIR)

	# now rename/join the output files into the files we expect
	mv $(TMP_DIR)/apiextensions.k8s.io_v1_customresourcedefinition_provisionings.metal3.io.yaml manifests/0000_31_cluster-baremetal-operator_02_metal3provisioning.crd.yaml
	mv $(TMP_DIR)/apps_v1_deployment_cluster-baremetal-operator.yaml manifests/0000_31_cluster-baremetal-operator_06_deployment.yaml

	# manifests needed for monitoring
	mv $(TMP_DIR)/monitoring.coreos.com_v1_servicemonitor_cluster-baremetal-operator-servicemonitor.yaml manifests/0000_90_cluster-baremetal-operator_03_servicemonitor.yaml
	mv $(TMP_DIR)/v1_service_cluster-baremetal-operator-service.yaml manifests/0000_31_cluster-baremetal-operator_03_service.yaml
	mv $(TMP_DIR)/v1_configmap_kube-rbac-proxy.yaml manifests/0000_31_cluster-baremetal-operator_05_kube-rbac-proxy-config.yaml

	# manifests needed for the webhook
	mv $(TMP_DIR)/v1_service_cluster-baremetal-webhook-service.yaml manifests/0000_31_cluster-baremetal-operator_03_webhookservice.yaml

	# cluster-baremetal-operator rbacs
	rm -f manifests/0000_31_cluster-baremetal-operator_05_rbac.yaml
	for rbac in $(RBAC_LIST) ; do \
	cat $(TMP_DIR)/$${rbac} >> manifests/0000_31_cluster-baremetal-operator_05_rbac.yaml ;\
	echo '---' >> manifests/0000_31_cluster-baremetal-operator_05_rbac.yaml ;\
	done

	# prometheus rbacs
	rm -rf manifests/0000_31_cluster-baremetal-operator_05_prometheus_rbac.yaml
	for rbac in $(PROMETHEUS_RBAC_LIST) ; do \
	cat $(TMP_DIR)/$${rbac} >> manifests/0000_31_cluster-baremetal-operator_05_prometheus_rbac.yaml ;\
	echo '---' >> manifests/0000_31_cluster-baremetal-operator_05_prometheus_rbac.yaml ;\
	done
	rm -rf $(TMP_DIR)

# Run go fmt against code
.PHONY: fmt
fmt:

# Run go lint against code
.PHONY: lint
lint:
	$(GOLANGCI_LINT) run

# Run go vet against code
.PHONY: vet
vet: lint

# Generate code
.PHONY: generate
generate:
	go generate -x ./...
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=cluster-baremetal-operator webhook paths=./... output:crd:artifacts:config=config/crd/bases
	sed -i '/^    controller-gen.kubebuilder.io\/version: (devel)/d' config/crd/bases/*
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths="./..."
	$(GOLANGCI_LINT) run --fix

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
