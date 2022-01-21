module github.com/openshift/cluster-baremetal-operator

go 1.15

require (
	github.com/ghodss/yaml v1.0.0
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/golangci/golangci-lint v1.32.0
	github.com/google/go-cmp v0.5.5
	github.com/metal3-io/baremetal-operator v0.0.0-00010101000000-000000000000
	github.com/openshift/api v0.0.0-20211012185411-2e1b88be96db
	github.com/openshift/client-go v0.0.0-20201020074620-f8fd44879f7c
	github.com/openshift/library-go v0.0.0-20201203122949-352bc2d14339
	github.com/pkg/errors v0.9.1
	github.com/stretchr/stew v0.0.0-20130812190256-80ef0842b48b
	github.com/stretchr/testify v1.7.0
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
	sigs.k8s.io/controller-runtime v0.10.2
	sigs.k8s.io/controller-tools v0.4.1
	sigs.k8s.io/kustomize/kustomize/v3 v3.9.4
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/metal3-io/baremetal-operator => github.com/openshift/baremetal-operator v0.0.0-20210527161605-4e331bfd4b1d
