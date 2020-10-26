# When bumping the Go version, don't forget to update the configuration of the
# CI jobs in openshift/release.
FROM registry.svc.ci.openshift.org/ocp/builder:rhel-8-golang-1.15-openshift-4.6 AS builder
WORKDIR /go/src/github.com/openshift/cluster-baremetal-operator
COPY . .
RUN make cluster-baremetal-operator

FROM registry.svc.ci.openshift.org/ocp/4.6:base
COPY --from=builder /go/src/github.com/openshift/cluster-baremetal-operator/bin/cluster-baremetal-operator /usr/bin/cluster-baremetal-operator
COPY --from=builder /go/src/github.com/openshift/cluster-baremetal-operator/manifests /manifests
# Uncomment when ready for the release so that CVO could manage the operator
LABEL io.openshift.release.operator=true
ENTRYPOINT ["/usr/bin/cluster-baremetal-operator"]