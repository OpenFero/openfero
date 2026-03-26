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

var _ = Describe("OpenFero Prometheus Metrics", Ordered, func() {
	var portForwardCmd *exec.Cmd
	var metricsLocalPort string

	BeforeAll(func() {
		By("verifying OpenFero is running")
		Eventually(func() bool {
			cmd := exec.Command("kubectl", "get", "deployment", "openfero", "-n", namespace,
				"-o", "jsonpath={.status.readyReplicas}")
			output, err := utils.Run(cmd)
			if err != nil {
				return false
			}
			return strings.TrimSpace(output) == "1"
		}, 120*time.Second, 2*time.Second).Should(BeTrue(), "OpenFero should have 1 ready replica")

		By("starting port-forward to OpenFero metrics endpoint")
		metricsLocalPort = "18081"
		portForwardCmd = exec.Command("kubectl", "port-forward", "svc/openfero", metricsLocalPort+":8080", "-n", namespace)
		err := portForwardCmd.Start()
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() error {
			cmd := exec.Command("curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
				fmt.Sprintf("http://localhost:%s/healthz", metricsLocalPort))
			output, err := utils.Run(cmd)
			if err != nil {
				return err
			}
			if strings.TrimSpace(output) != "200" {
				return fmt.Errorf("healthz returned %s", output)
			}
			return nil
		}, 30*time.Second, time.Second).Should(Succeed(), "OpenFero should respond on port-forward")
	})

	AfterAll(func() {
		By("stopping port-forward")
		if portForwardCmd != nil && portForwardCmd.Process != nil {
			_ = portForwardCmd.Process.Kill()
		}
	})

	scrapeMetrics := func() string {
		cmd := exec.Command("curl", "-s",
			fmt.Sprintf("http://localhost:%s/metrics", metricsLocalPort))
		output, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Scraping /metrics should succeed")
		return output
	}

	Context("Operarius sync error metric", func() {
		It("should expose openfero_operarius_sync_errors_total", func() {
			metrics := scrapeMetrics()
			Expect(metrics).To(ContainSubstring("openfero_operarius_sync_errors_total"),
				"openfero_operarius_sync_errors_total counter must be present in /metrics output")
		})

		It("should have zero sync errors when informer started successfully", func() {
			metrics := scrapeMetrics()
			// On a healthy startup the counter must be 0 (no error increments)
			Expect(metrics).To(MatchRegexp("openfero_operarius_sync_errors_total\\s+0"),
				"openfero_operarius_sync_errors_total should be 0 after a clean startup")
		})
	})

	Context("Operarius items loaded metric", func() {
		It("should expose openfero_operarius_items_loaded", func() {
			metrics := scrapeMetrics()
			Expect(metrics).To(ContainSubstring("openfero_operarius_items_loaded"),
				"openfero_operarius_items_loaded gauge must be present in /metrics output")
		})

		It("should reflect newly created Operarius CRDs in the gauge", func() {
			metricsOperariusYAML := strings.Join([]string{
				"apiVersion: openfero.io/v1alpha1",
				"kind: Operarius",
				"metadata:",
				"  name: metrics-test-operarius",
				"  namespace: openfero",
				"spec:",
				"  alertSelector:",
				"    alertname: MetricsTestAlert",
				"    status: firing",
				"  jobTemplate:",
				"    spec:",
				"      backoffLimit: 0",
				"      template:",
				"        spec:",
				"          containers:",
				"          - name: remediation",
				"            image: alpine:latest",
				`            command: ["echo", "metrics test"]`,
				"          restartPolicy: Never",
				"  enabled: true",
				"  priority: 1",
			}, "\n")

			By("recording current gauge value before creating a new Operarius")
			metricsBefore := scrapeMetrics()
			beforeValue := extractGaugeValue(metricsBefore, "openfero_operarius_items_loaded")

			By("creating a new Operarius CRD")
			Expect(utils.ApplyYAML(withNamespace(metricsOperariusYAML))).To(Succeed())
			defer func() {
				_ = utils.DeleteYAML(withNamespace(metricsOperariusYAML))
			}()

			By("verifying the gauge increases")
			Eventually(func() float64 {
				return extractGaugeValue(scrapeMetrics(), "openfero_operarius_items_loaded")
			}, 30*time.Second, time.Second).Should(BeNumerically(">", beforeValue),
				"openfero_operarius_items_loaded should increase after an Operarius is created")

			By("deleting the Operarius CRD and verifying the gauge decreases")
			Expect(utils.DeleteYAML(withNamespace(metricsOperariusYAML))).To(Succeed())

			Eventually(func() float64 {
				return extractGaugeValue(scrapeMetrics(), "openfero_operarius_items_loaded")
			}, 30*time.Second, time.Second).Should(BeNumerically("==", beforeValue),
				"openfero_operarius_items_loaded should return to the original value after the Operarius is deleted")
		})
	})
})

// extractGaugeValue returns the float64 value of a gauge metric from a raw Prometheus text
// exposition. Returns -1 when the metric line is not found or cannot be parsed.
func extractGaugeValue(metricsOutput, metricName string) float64 {
	for _, line := range strings.Split(metricsOutput, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if strings.HasPrefix(line, metricName+" ") || strings.HasPrefix(line, metricName+"\t") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				var value float64
				if _, err := fmt.Sscanf(parts[1], "%f", &value); err == nil {
					return value
				}
			}
		}
	}
	return -1
}
