# OpenFero Makefile for CRD and API management

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Kubernetes/Controller parameters
CONTROLLER_GEN=$(GOCMD) run sigs.k8s.io/controller-tools/cmd/controller-gen
CRD_OPTIONS=crd:headerFile="hack/boilerplate.yaml.txt"
OBJECT_OPTIONS=object:headerFile="hack/boilerplate.go.txt"

# Directories
API_DIR=./api/...
CRD_DIR=charts/openfero/crds

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: generate
generate: ## Generate CRDs and DeepCopy methods
	$(CONTROLLER_GEN) $(OBJECT_OPTIONS) paths=$(API_DIR)
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths=$(API_DIR) output:crd:artifacts:config=$(CRD_DIR)

.PHONY: manifests
manifests: generate ## Generate CRD manifests (alias for generate)

.PHONY: test
test: ## Run tests
	$(GOTEST) -v ./pkg/...

.PHONY: test-operarius
test-operarius: ## Run Operarius-specific tests
	$(GOTEST) -v ./pkg/services -run TestOperarius

.PHONY: build
build: ## Build the OpenFero binary
	$(GOBUILD) -o openfero .

.PHONY: clean
clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -f openfero

.PHONY: deps
deps: ## Download and verify dependencies
	$(GOMOD) download
	$(GOMOD) verify

.PHONY: tidy
tidy: ## Clean up go.mod and go.sum
	$(GOMOD) tidy

.PHONY: fmt
fmt: ## Format Go code
	$(GOCMD) fmt ./...

.PHONY: vet
vet: ## Run go vet
	$(GOCMD) vet ./...

.PHONY: lint
lint: fmt vet ## Run formatting and vetting

.PHONY: install-crds
install-crds: manifests ## Install CRDs into the current Kubernetes cluster
	kubectl apply --server-side -f $(CRD_DIR)/

.PHONY: uninstall-crds
uninstall-crds: ## Uninstall CRDs from the current Kubernetes cluster
	kubectl delete -f $(CRD_DIR)/ --ignore-not-found=true

.PHONY: sample-operarius
sample-operarius: ## Apply sample Operarius resources
	kubectl apply -f $(SAMPLE_DIR)/openfero_v1alpha1_operarius_kubequota.yaml
	kubectl apply -f $(SAMPLE_DIR)/openfero_v1alpha1_operarius_podrestart.yaml

.PHONY: delete-samples
delete-samples: ## Delete sample Operarius resources
	kubectl delete -f $(SAMPLE_DIR)/ --ignore-not-found=true

.PHONY: validate-crds
validate-crds: ## Validate CRD YAML files
	@echo "Validating CRD files..."
	@for file in $(CRD_DIR)/*.yaml; do \
		echo "Validating $$file"; \
		kubectl --dry-run=client apply -f $$file; \
	done

.PHONY: docs
docs: ## Generate documentation for APIs
	@echo "API documentation is available in the generated CRD files"
	@echo "Use 'kubectl explain operarius' after installing CRDs"

.PHONY: dev-setup
dev-setup: deps generate ## Set up development environment
	@echo "Development environment set up!"
	@echo "Run 'make install-crds' to install CRDs in your cluster"

.PHONY: ci
ci: lint test build ## Run CI pipeline locally

# Show CRD status in cluster
.PHONY: crd-status
crd-status: ## Show status of OpenFero CRDs in cluster
	@echo "OpenFero CRDs in cluster:"
	@kubectl get crd | grep openfero.io || echo "No OpenFero CRDs found"
	@echo ""
	@echo "Operarius resources:"
	@kubectl get operarius -A || echo "No Operarius resources found"

# Debug target to show generated CRDs
.PHONY: show-crds
show-crds: ## Show generated CRD content
	@echo "Generated CRD files:"
	@ls -la $(CRD_DIR)/
	@echo ""
	@echo "Sample files:"
	@ls -la $(SAMPLE_DIR)/

# Migration helper
.PHONY: migration-check
migration-check: ## Check for ConfigMap-based operarios that need migration
	@echo "Checking for existing ConfigMap-based operarios..."
	@kubectl get configmap -l app=openfero -A || echo "No openfero ConfigMaps found"
