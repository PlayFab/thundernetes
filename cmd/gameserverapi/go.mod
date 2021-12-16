module github.com/playfab/thundernetes/tools/gameserverapi

go 1.16

require (
	github.com/gin-gonic/gin v1.7.4
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/playfab/thundernetes/operator v0.0.0-20211119172533-f8e29f6b7145
	github.com/sirupsen/logrus v1.8.1
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	sigs.k8s.io/controller-runtime v0.9.2
)

replace github.com/playfab/thundernetes/operator => ../../operator
