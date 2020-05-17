module github.com/playfab/thundernetes/e2e/cmd

go 1.16

require (
	github.com/google/uuid v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/playfab/thundernetes/operator v0.0.0-20210706230151-28048dd54fdd
	k8s.io/api v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	sigs.k8s.io/controller-runtime v0.9.2
)

replace github.com/playfab/thundernetes/operator => ../../operator
