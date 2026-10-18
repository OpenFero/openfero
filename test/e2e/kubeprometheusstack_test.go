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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/OpenFero/openfero/test/utils"
)

// This is a SMOKE TEST: it deploys a REAL kube-prometheus-stack (Prometheus
// Operator + Alertmanager) into the E2E Kind cluster and proves that the
// Prometheus -> Alertmanager -> OpenFero webhook pipeline is genuinely wired
// up - real rule evaluation, real Alertmanager routing config, real webhook
// delivery - using a single alert (KubeContainerWaiting) as the example.
//
// This is deliberately narrow. The Operarii catalog under
// operarios/kube-prometheus-stack/ is a set of *templates* (see that
// directory's README), not production-ready jobs matched blindly cluster-wide
// - so exhaustively proving that every alert's real, upstream PromQL
// expression fires under some contrived condition adds cost without much
// value: it mostly tests "does kube-prometheus-stack's own alerting rule do
// what its docs say", not OpenFero's logic. What actually only a real
// Prometheus/Alertmanager install can catch - config/webhook wiring being
// correct - is covered here with one alert. The other 7 alerts' matching,
// job-creation, and remediation-effect logic is covered more cheaply via
// synthetic Alertmanager-webhook JSON in the "Operarius Starter Pack
// Real-Resource Remediation" suite in e2e_test.go.
//
// Installing a full Prometheus Operator stack is heavier than anything else
// in the e2e suite, so this is gated behind Label("kube-prometheus-stack")
// and only runs via `make test-e2e-kube-prometheus-stack`, not the default
// `make test-e2e`.
var _ = Describe("kube-prometheus-stack Real Alert Evaluation", Ordered, Label("kube-prometheus-stack"), func() {
	const kpsRelease = "kube-prometheus-stack"
	const testNs = "e2e-kps-containerwaiting-test"

	BeforeAll(func() {
		By("adding/updating the prometheus-community Helm repo")
		cmd := exec.Command("helm", "repo", "add", "prometheus-community", "https://prometheus-community.github.io/helm-charts")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())

		cmd = exec.Command("helm", "repo", "update")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())

		By("installing kube-prometheus-stack (real Prometheus Operator + Alertmanager)")
		cmd = exec.Command("helm", "upgrade", "--install", kpsRelease, "prometheus-community/kube-prometheus-stack",
			"--namespace", namespace,
			"-f", "test/e2e/testdata/kube-prometheus-stack-values.yaml",
			"--wait", "--timeout", "5m",
		)
		output, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "kube-prometheus-stack install failed: %s", output)

		By("waiting for the Prometheus pod to be running")
		Eventually(func() bool {
			cmd := exec.Command("kubectl", "get", "pods", "-n", namespace,
				"-l", "app.kubernetes.io/name=prometheus",
				"-o", "jsonpath={.items[0].status.phase}")
			output, err := utils.Run(cmd)
			if err != nil {
				return false
			}
			return strings.TrimSpace(output) == "Running"
		}, 180*time.Second, 3*time.Second).Should(BeTrue(), "Prometheus pod should be running")

		By("waiting for the Alertmanager pod to be running")
		Eventually(func() bool {
			cmd := exec.Command("kubectl", "get", "pods", "-n", namespace,
				"-l", "app.kubernetes.io/name=alertmanager",
				"-o", "jsonpath={.items[0].status.phase}")
			output, err := utils.Run(cmd)
			if err != nil {
				return false
			}
			return strings.TrimSpace(output) == "Running"
		}, 180*time.Second, 3*time.Second).Should(BeTrue(), "Alertmanager pod should be running")

		By("creating test namespace")
		createKPSTestNamespace(testNs)
	})

	AfterAll(func() {
		By("cleaning up test namespace")
		deleteKPSTestNamespace(testNs)

		By("uninstalling kube-prometheus-stack")
		cmd := exec.Command("helm", "uninstall", kpsRelease, "--namespace", namespace)
		_, _ = utils.Run(cmd)

		By("cleaning up any leftover remediation jobs and Operarii")
		cmd = exec.Command("kubectl", "delete", "jobs", "-l", "openfero.io/managed-by=openfero", "-n", namespace, "--ignore-not-found")
		_, _ = utils.Run(cmd)
		cmd = exec.Command("kubectl", "delete", "operarius", "--all", "-n", namespace, "--ignore-not-found")
		_, _ = utils.Run(cmd)
	})

	It("should remediate a real, Prometheus-evaluated KubeContainerWaiting alert end to end", func() {
		const alertName = "KubeContainerWaiting"
		operariusDir := filepath.Join("operarios/kube-prometheus-stack", alertName)

		By("applying the real " + alertName + " Operarius + RBAC (enabled for this test)")
		rbacYAML := withNamespace(readRepoFile(filepath.Join(operariusDir, "rbac.yaml")))
		Expect(utils.ApplyYAML(rbacYAML)).To(Succeed())
		defer func() { _ = utils.DeleteYAML(rbacYAML) }()

		// The shipped operarius.yaml is a template with spec.enabled: false
		// (see operarios/kube-prometheus-stack/README.md) - flip it on here
		// to simulate a user who has reviewed and enabled it.
		operariusYAML := withEnabled(withNamespace(readRepoFile(filepath.Join(operariusDir, "operarius.yaml"))))
		Expect(utils.ApplyYAML(operariusYAML)).To(Succeed())
		defer func() { _ = utils.DeleteYAML(operariusYAML) }()

		// allow the Operarius controller to register the new resource
		time.Sleep(2 * time.Second)

		By("provoking a real KubeContainerWaiting condition (bad image tag)")
		podYAML := fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: imagepull-victim
  namespace: %s
spec:
  containers:
  - name: main
    image: busybox:e2e-nonexistent-tag
    command: ["sleep", "3600"]
  restartPolicy: Always
`, testNs)
		Expect(utils.ApplyYAML(podYAML)).To(Succeed())

		By("waiting for the pod to actually enter a waiting state (ImagePullBackOff/ErrImagePull)")
		Eventually(func() string {
			cmd := exec.Command("kubectl", "get", "pod", "imagepull-victim", "-n", testNs,
				"-o", "jsonpath={.status.containerStatuses[0].state.waiting.reason}")
			output, err := utils.Run(cmd)
			if err != nil {
				return ""
			}
			return strings.TrimSpace(output)
		}, 60*time.Second, 2*time.Second).Should(Or(Equal("ImagePullBackOff"), Equal("ErrImagePull")))

		cmd := exec.Command("kubectl", "get", "pod", "imagepull-victim", "-n", testNs, "-o", "jsonpath={.metadata.uid}")
		uid, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())
		originalUID := strings.TrimSpace(uid)

		By("waiting for the real Prometheus alert to reach OpenFero and create a remediation Job")
		Eventually(func() bool {
			cmd := exec.Command("kubectl", "get", "jobs", "-n", namespace,
				"-l", "openfero.io/alert="+alertName,
				"-o", "jsonpath={.items[*].metadata.name}")
			output, err := utils.Run(cmd)
			if err != nil {
				return false
			}
			return len(strings.TrimSpace(output)) > 0
		}, 180*time.Second, 3*time.Second).Should(BeTrue(),
			"a remediation Job should be created from a real, Prometheus-evaluated %s alert", alertName)

		waitForRemediationJobSuccess(alertName)

		By("verifying the original pod was deleted")
		Eventually(func() bool {
			cmd := exec.Command("kubectl", "get", "pod", "imagepull-victim", "-n", testNs,
				"-o", "jsonpath={.metadata.uid}", "--ignore-not-found")
			newUID, err := utils.Run(cmd)
			if err != nil {
				return false
			}
			trimmed := strings.TrimSpace(newUID)
			return trimmed == "" || trimmed != originalUID
		}, 30*time.Second, 2*time.Second).Should(BeTrue(), "original imagepull-victim pod should be deleted")
	})
})

// readRepoFile reads a file relative to the project root. utils.Run changes
// the process's working directory as a side effect, so this always resolves
// via utils.GetProjectDir() rather than a plain relative path.
func readRepoFile(relPath string) string {
	dir, err := utils.GetProjectDir()
	Expect(err).NotTo(HaveOccurred())
	data, err := os.ReadFile(filepath.Join(dir, relPath))
	Expect(err).NotTo(HaveOccurred())
	return string(data)
}

// withEnabled flips a shipped Operarius template's spec.enabled from its
// disabled-by-default "enabled: false" to "enabled: true".
func withEnabled(yaml string) string {
	return strings.Replace(yaml, "enabled: false", "enabled: true", 1)
}

func createKPSTestNamespace(testNs string) {
	cmd := exec.Command("kubectl", "create", "namespace", testNs, "--dry-run=client", "-o", "yaml")
	output, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())
	Expect(utils.ApplyYAML(output)).To(Succeed())
}

func deleteKPSTestNamespace(testNs string) {
	cmd := exec.Command("kubectl", "delete", "namespace", testNs, "--ignore-not-found", "--wait=false")
	_, _ = utils.Run(cmd)
}

// waitForRemediationJobSuccess waits for the (already-created) remediation
// Job for alertName to complete successfully. Shared with the synthetic
// starter-pack tests in e2e_test.go.
func waitForRemediationJobSuccess(alertName string) {
	By("waiting for the remediation Job to succeed")
	Eventually(func() bool {
		cmd := exec.Command("kubectl", "get", "jobs", "-n", namespace,
			"-l", "openfero.io/alert="+alertName,
			"-o", "jsonpath={.items[?(@.status.succeeded==1)].metadata.name}")
		output, err := utils.Run(cmd)
		if err != nil {
			return false
		}
		return len(strings.TrimSpace(output)) > 0
	}, 120*time.Second, 3*time.Second).Should(BeTrue(), "the remediation Job for %s should succeed", alertName)
}

// remediationJobLogs returns the logs of the (first) remediation Job pod
// for alertName, or "" if it can't be found yet. Shared with the synthetic
// starter-pack tests in e2e_test.go.
func remediationJobLogs(alertName string) string {
	cmd := exec.Command("kubectl", "get", "pods", "-n", namespace,
		"-l", "openfero.io/alert="+alertName,
		"-o", "jsonpath={.items[0].metadata.name}")
	podName, err := utils.Run(cmd)
	if err != nil || strings.TrimSpace(podName) == "" {
		return ""
	}

	cmd = exec.Command("kubectl", "logs", strings.TrimSpace(podName), "-n", namespace)
	output, _ := utils.Run(cmd)
	return output
}
