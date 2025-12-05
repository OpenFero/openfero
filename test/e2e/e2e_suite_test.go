//go:build e2e
// +build e2e

/*
Copyright 2025 OpenFero.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/OpenFero/openfero/test/utils"
)

// TestE2E runs the end-to-end test suite for OpenFero
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting OpenFero E2E test suite\n")
	RunSpecs(t, "OpenFero E2E Suite")
}

var _ = BeforeSuite(func() {
	By("ensuring Kind cluster exists")
	if !utils.KindClusterExists() {
		_, _ = fmt.Fprintf(GinkgoWriter, "Creating Kind cluster...\n")
		Expect(utils.CreateKindCluster()).To(Succeed(), "Failed to create Kind cluster")
	} else {
		_, _ = fmt.Fprintf(GinkgoWriter, "Kind cluster already exists\n")
	}
})

var _ = AfterSuite(func() {
	// Keep the cluster for debugging; delete manually if needed
	_, _ = fmt.Fprintf(GinkgoWriter, "E2E tests completed. Cluster kept for inspection.\n")
	_, _ = fmt.Fprintf(GinkgoWriter, "Run 'kind delete cluster --name openfero-test-e2e' to clean up.\n")
})
