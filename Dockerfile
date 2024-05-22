# When bumping the Go version, don't forget to update the configuration of the
# CI jobs in openshift/release.
FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.21-openshift-4.17 AS builder
WORKDIR /go/src/github.com/openshift/cluster-baremetal-operator
COPY . .
RUN make build

FROM registry.ci.openshift.org/ocp/4.17:base-rhel9
COPY --from=builder /go/src/github.com/openshift/cluster-baremetal-operator/bin/cluster-baremetal-operator /usr/bin/cluster-baremetal-operator
COPY --from=builder /go/src/github.com/openshift/cluster-baremetal-operator/manifests /manifests
LABEL io.openshift.release.operator=true
ENTRYPOINT ["/usr/bin/cluster-baremetal-operator"]
