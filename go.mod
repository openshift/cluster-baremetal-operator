module github.com/openshift/cluster-baremetal-operator

go 1.15

require (
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/golangci/golangci-lint v1.32.0
	github.com/google/go-cmp v0.5.2
	github.com/openshift/api v0.0.0-20201216151826-78a19e96f9eb
	github.com/openshift/client-go v0.0.0-20201214125552-e615e336eb49
	github.com/openshift/library-go v0.0.0-20201215165635-4ee79b1caed5
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/stew v0.0.0-20130812190256-80ef0842b48b
	github.com/stretchr/testify v1.6.1
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0
	k8s.io/api v0.20.0
	k8s.io/apimachinery v0.20.0
	k8s.io/client-go v0.20.0
	k8s.io/code-generator v0.20.0
	k8s.io/component-base v0.20.0
	k8s.io/klog/v2 v2.4.0
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
	sigs.k8s.io/controller-runtime v0.7.0
	sigs.k8s.io/controller-tools v0.3.0
	sigs.k8s.io/kustomize/kustomize/v3 v3.8.5
)

replace github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.1
