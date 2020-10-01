module github.com/openshift/cluster-baremetal-operator

go 1.13

require (
	github.com/go-logr/logr v0.2.1-0.20200730175230-ee2de8da5be6
	github.com/go-logr/zapr v0.2.0 // indirect
	github.com/onsi/ginkgo v1.12.0 // indirect
	github.com/openshift/api v0.0.0-20200827090112-c05698d102cf
	github.com/openshift/client-go v0.0.0-20200827190008-3062137373b5
	github.com/openshift/library-go v0.0.0-20200910214143-887092e305c1
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.4.0
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	k8s.io/api v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v0.19.0
	sigs.k8s.io/controller-runtime v0.6.0
	sigs.k8s.io/controller-tools v0.3.0
)
