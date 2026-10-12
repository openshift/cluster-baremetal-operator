# When bumping the Go version, don't forget to update the configuration of the
# CI jobs in openshift/release.
FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.26-openshift-5.0 AS builder
WORKDIR /go/src/github.com/openshift/cluster-baremetal-operator
COPY . .
RUN make build

# Test extension builder stage (added by ote-migration)
FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.26-openshift-5.0 AS test-extension-builder
WORKDIR /go/src/github.com/openshift/cluster-baremetal-operator
COPY tests-extension/ ./tests-extension/
RUN cd tests-extension && \
    GOMAXPROCS=1 make build && \
    cd bin && \
    gzip cluster-baremetal-operator-tests-ext

FROM registry.ci.openshift.org/ocp/5.0:base-rhel9
COPY --from=builder /go/src/github.com/openshift/cluster-baremetal-operator/bin/cluster-baremetal-operator /usr/bin/cluster-baremetal-operator
COPY --from=builder /go/src/github.com/openshift/cluster-baremetal-operator/manifests /manifests

# Copy test extension binary (added by ote-migration)
COPY --from=test-extension-builder /go/src/github.com/openshift/cluster-baremetal-operator/tests-extension/bin/cluster-baremetal-operator-tests-ext.gz /usr/bin/

LABEL io.openshift.release.operator=true
ENTRYPOINT ["/usr/bin/cluster-baremetal-operator"]
