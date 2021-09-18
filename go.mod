module github.com/openshift/cluster-baremetal-operator

go 1.16

require (
	github.com/ghodss/yaml v1.0.0
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/golangci/golangci-lint v1.41.1
	github.com/google/go-cmp v0.5.6
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/metal3-io/baremetal-operator v0.0.0-00010101000000-000000000000
	github.com/openshift/api v0.0.0-20210915110300-3cd8091317c4
	github.com/openshift/client-go v0.0.0-20210916133943-9acee1a0fb83
	github.com/openshift/library-go v0.0.0-20210916194400-ae21aab32431
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.7.0
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
	golang.org/x/oauth2 v0.0.0-20210628180205-a41e5a781914 // indirect
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210802155522-efc7438f0176
	sigs.k8s.io/controller-runtime v0.9.6
	sigs.k8s.io/controller-tools v0.6.2
	sigs.k8s.io/kustomize/kustomize/v3 v3.8.5
)

replace (
	github.com/metal3-io/baremetal-operator => github.com/openshift/baremetal-operator v0.0.0-20210527161605-4e331bfd4b1d
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
)
