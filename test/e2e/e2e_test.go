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
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/OpenFero/openfero/test/utils"
)

const (
	namespace = "openfero"
)

var _ = Describe("OpenFero Operarius CRD", Ordered, func() {
	BeforeAll(func() {
		By("creating test namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace, "--dry-run=client", "-o", "yaml")
		output, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())

		cmd = exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(output)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())

		By("installing Operarius CRDs")
		cmd = exec.Command("kubectl", "apply", "-f", "charts/openfero/crds/")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		By("cleaning up test resources")
		cmd := exec.Command("kubectl", "delete", "operarius", "--all", "-n", namespace, "--ignore-not-found")
		_, _ = utils.Run(cmd)

		By("cleaning up test namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace, "--ignore-not-found")
		_, _ = utils.Run(cmd)
	})

	Context("CRD Installation", func() {
		It("should have Operarius CRD installed", func() {
			cmd := exec.Command("kubectl", "get", "crd", "operariuses.openfero.io")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("operariuses.openfero.io"))
		})

		It("should support short name 'op'", func() {
			cmd := exec.Command("kubectl", "api-resources", "--api-group=openfero.io")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("op"))
		})
	})

	Context("Operarius Resource Creation", func() {
		const operariusYAML = `
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: test-pod-restart
  namespace: openfero
spec:
  alertSelector:
    alertname: PodCrashLooping
    status: firing
  jobTemplate:
    spec:
      backoffLimit: 0
      template:
        spec:
          containers:
          - name: remediation
            image: alpine:latest
            command:
            - /bin/sh
            - -c
            - echo "Remediation executed"
          restartPolicy: Never
  enabled: true
  priority: 50
`

		It("should create an Operarius resource", func() {
			By("applying Operarius YAML")
			err := utils.ApplyYAML(operariusYAML)
			Expect(err).NotTo(HaveOccurred())

			By("verifying Operarius was created")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "operarius", "test-pod-restart", "-n", namespace)
				_, err := utils.Run(cmd)
				return err
			}, 30*time.Second, time.Second).Should(Succeed())
		})

		It("should show correct columns in kubectl get", func() {
			cmd := exec.Command("kubectl", "get", "operarius", "test-pod-restart", "-n", namespace, "-o", "wide")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Check for expected column headers and values
			Expect(output).To(ContainSubstring("PodCrashLooping"))
			Expect(output).To(ContainSubstring("firing"))
			Expect(output).To(ContainSubstring("true"))
		})

		It("should allow getting Operarius by short name", func() {
			cmd := exec.Command("kubectl", "get", "op", "test-pod-restart", "-n", namespace)
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("test-pod-restart"))
		})

		AfterAll(func() {
			err := utils.DeleteYAML(operariusYAML)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Operarius Validation", func() {
		It("should reject Operarius without required fields", func() {
			invalidYAML := `
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: invalid-operarius
  namespace: openfero
spec:
  alertSelector:
    alertname: ""
    status: invalid
  jobTemplate:
    spec:
      template:
        spec:
          containers: []
`
			err := utils.ApplyYAML(invalidYAML)
			// This should fail validation
			Expect(err).To(HaveOccurred())
		})

		It("should only accept firing or resolved status", func() {
			invalidStatusYAML := `
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: invalid-status
  namespace: openfero
spec:
  alertSelector:
    alertname: TestAlert
    status: pending
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: test
            image: alpine:latest
          restartPolicy: Never
`
			err := utils.ApplyYAML(invalidStatusYAML)
			Expect(err).To(HaveOccurred())
			// Error should mention status validation
		})
	})

	Context("Multiple Operarius with Priority", func() {
		const lowPriorityYAML = `
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: low-priority
  namespace: openfero
spec:
  alertSelector:
    alertname: DiskFull
    status: firing
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: low
            image: alpine:latest
            command: ["echo", "low priority"]
          restartPolicy: Never
  priority: 10
`

		const highPriorityYAML = `
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: high-priority
  namespace: openfero
spec:
  alertSelector:
    alertname: DiskFull
    status: firing
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: high
            image: alpine:latest
            command: ["echo", "high priority"]
          restartPolicy: Never
  priority: 100
`

		BeforeAll(func() {
			Expect(utils.ApplyYAML(lowPriorityYAML)).To(Succeed())
			Expect(utils.ApplyYAML(highPriorityYAML)).To(Succeed())
		})

		AfterAll(func() {
			_ = utils.DeleteYAML(lowPriorityYAML)
			_ = utils.DeleteYAML(highPriorityYAML)
		})

		It("should have both Operarius resources", func() {
			cmd := exec.Command("kubectl", "get", "operarius", "-n", namespace)
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("low-priority"))
			Expect(output).To(ContainSubstring("high-priority"))
		})
	})

	Context("Operarius with Labels Selector", func() {
		const labeledOperariusYAML = `
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: labeled-operarius
  namespace: openfero
spec:
  alertSelector:
    alertname: HighMemory
    status: firing
    labels:
      severity: critical
      team: platform
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: remediate
            image: alpine:latest
            command: ["echo", "handling critical alert"]
          restartPolicy: Never
  priority: 75
`

		It("should create Operarius with label selectors", func() {
			err := utils.ApplyYAML(labeledOperariusYAML)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				_ = utils.DeleteYAML(labeledOperariusYAML)
			}()

			cmd := exec.Command("kubectl", "get", "operarius", "labeled-operarius", "-n", namespace, "-o", "jsonpath={.spec.alertSelector.labels}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("critical"))
			Expect(output).To(ContainSubstring("platform"))
		})
	})
})

var _ = Describe("OpenFero Alert Webhook", Ordered, func() {
	// These tests require OpenFero to be running
	// Skip if not deployed

	BeforeAll(func() {
		Skip("OpenFero deployment tests - run manually with deployed instance")
	})

	Context("Webhook Processing", func() {
		It("should accept valid alert webhook", func() {
			alertJSON := `{
				"version": "4",
				"status": "firing",
				"commonLabels": {
					"alertname": "TestAlert"
				},
				"alerts": [{
					"labels": {
						"alertname": "TestAlert",
						"severity": "warning"
					}
				}]
			}`

			cmd := exec.Command("curl", "-X", "POST",
				"http://localhost:8080/alerts",
				"-H", "Content-Type: application/json",
				"-d", alertJSON,
			)
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "Webhook response: %s\n", output)
		})
	})
})
