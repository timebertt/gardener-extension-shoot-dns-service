module github.com/gardener/gardener-extension-shoot-dns-service

go 1.14

require (
	github.com/ahmetb/gen-crd-api-reference-docs v0.2.0
	github.com/gardener/external-dns-management v0.7.16
	github.com/gardener/gardener v1.8.0
	github.com/gardener/gardener-resource-manager v0.17.0
	github.com/go-logr/logr v0.1.0
	github.com/gobuffalo/packr/v2 v2.8.0
	github.com/golang/mock v1.4.4-0.20200731163441-8734ec565a4d
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v0.0.6
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.17.9
	k8s.io/apimachinery v0.17.9
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/code-generator v0.17.9
	k8s.io/component-base v0.17.9
	sigs.k8s.io/controller-runtime v0.5.5
)

replace (
	k8s.io/api => k8s.io/api v0.17.9
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.9
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.9
	k8s.io/apiserver => k8s.io/apiserver v0.17.9
	k8s.io/client-go => k8s.io/client-go v0.17.9
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.17.9
	k8s.io/code-generator => k8s.io/code-generator v0.17.9
	k8s.io/component-base => k8s.io/component-base v0.17.9
	k8s.io/helm => k8s.io/helm v2.13.1+incompatible
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.17.9
)
