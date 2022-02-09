//go:build tools
// +build tools

package main

import (
	_ "github.com/onsi/ginkgo/v2/ginkgo"
)

// required because of ...
// https://onsi.github.io/ginkgo/#recommended-continuous-integration-configuration
