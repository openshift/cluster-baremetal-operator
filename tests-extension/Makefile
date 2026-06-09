BINARY := bin/cluster-baremetal-operator-tests-ext

.PHONY: build
build:
	@echo "Building extension binary..."
	@cd openshift/test/e2e && $(MAKE) -f bindata.mk update-bindata
	@mkdir -p bin
	@echo "Building with reduced memory footprint (disabled optimizations)..."
	GOMEMLIMIT=2GiB GOTOOLCHAIN=auto GOSUMDB=sum.golang.org go build -mod=vendor -p=1 -gcflags="all=-N -l" -o $(BINARY) ./cmd
	@echo "✅ Binary built: $(BINARY)"

.PHONY: clean
clean:
	@rm -f $(BINARY)
	@cd openshift/test/e2e && $(MAKE) -f bindata.mk clean-bindata

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build  - Build extension binary"
	@echo "  clean  - Remove binaries and bindata"
