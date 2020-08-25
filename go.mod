module github.com/openshift/cluster-baremetal-operator

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.8.1
	github.com/openshift/api v0.0.0-20200424083944-0422dc17083e
	k8s.io/apimachinery v0.18.2
	k8s.io/api v0.18.2
	k8s.io/client-go v0.18.2
	sigs.k8s.io/controller-runtime v0.6.0
)
