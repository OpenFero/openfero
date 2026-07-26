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

// This suite deploys a REAL kube-prometheus-stack (Prometheus Operator +
// Alertmanager) into the E2E Kind cluster and lets its actual PrometheusRules
// fire, routed through a real Alertmanager to OpenFero - as opposed to the
// synthetic webhook JSON posted directly to /alerts elsewhere in this package.
//
// Most of these alerts default to a 15m-1h `for:` duration, which would make
// a full real-Prometheus e2e run impractically slow. kube-prometheus-stack
// exposes a `customRules.<AlertName>.for` Helm value baked into its own rule
// templates, so test/e2e/testdata/kube-prometheus-stack-values.yaml overrides
// only that wait duration (down to 10s) while leaving the real PromQL
// expression untouched. KubeJobNotCompleted has no `for:` and a hardcoded 12h
// threshold in its expr, so it is disabled and replaced with a test-only
// PrometheusRule fixture (testdata/kubejobnotcompleted-test-rule.yaml) that
// mirrors the same alert with a much smaller threshold.
//
// This is heavier than anything else in the e2e suite (installing a full
// Prometheus Operator stack), so it is gated behind Label("kube-prometheus-stack")
// and only runs via `make test-e2e-kube-prometheus-stack`, not the default
// `make test-e2e`.
var _ = Describe("kube-prometheus-stack Real Alert Evaluation", Ordered, Label("kube-prometheus-stack"), func() {
	const kpsRelease = "kube-prometheus-stack"

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

		By("applying the KubeJobNotCompleted test-only PrometheusRule fixture")
		fixture := withNamespace(readRepoFile("test/e2e/testdata/kubejobnotcompleted-test-rule.yaml"))
		Expect(utils.ApplyYAML(fixture)).To(Succeed())

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

		By("allowing Prometheus Operator to reload the test-only PrometheusRule fixture")
		time.Sleep(30 * time.Second)
	})

	AfterAll(func() {
		By("removing the KubeJobNotCompleted test-only PrometheusRule fixture")
		fixture := withNamespace(readRepoFile("test/e2e/testdata/kubejobnotcompleted-test-rule.yaml"))
		_ = utils.DeleteYAML(fixture)

		By("uninstalling kube-prometheus-stack")
		cmd := exec.Command("helm", "uninstall", kpsRelease, "--namespace", namespace)
		_, _ = utils.Run(cmd)

		By("cleaning up any leftover remediation jobs and Operarii")
		cmd = exec.Command("kubectl", "delete", "jobs", "-l", "openfero.io/managed-by=openfero", "-n", namespace, "--ignore-not-found")
		_, _ = utils.Run(cmd)
		cmd = exec.Command("kubectl", "delete", "operarius", "--all", "-n", namespace, "--ignore-not-found")
		_, _ = utils.Run(cmd)
	})

	Context("KubeContainerWaiting", func() {
		const testNs = "e2e-kps-containerwaiting-test"

		BeforeEach(func() { createKPSTestNamespace(testNs) })
		AfterEach(func() { deleteKPSTestNamespace(testNs) })

		It("should restart a pod stuck waiting on a bad image when the real alert fires", func() {
			var originalUID string

			runKPSScenario(testNs, "KubeContainerWaiting",
				func(ns string) {
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
`, ns)
					Expect(utils.ApplyYAML(podYAML)).To(Succeed())

					By("waiting for the pod to actually enter a waiting state (ImagePullBackOff/ErrImagePull)")
					Eventually(func() string {
						cmd := exec.Command("kubectl", "get", "pod", "imagepull-victim", "-n", ns,
							"-o", "jsonpath={.status.containerStatuses[0].state.waiting.reason}")
						output, err := utils.Run(cmd)
						if err != nil {
							return ""
						}
						return strings.TrimSpace(output)
					}, 60*time.Second, 2*time.Second).Should(Or(Equal("ImagePullBackOff"), Equal("ErrImagePull")))

					cmd := exec.Command("kubectl", "get", "pod", "imagepull-victim", "-n", ns, "-o", "jsonpath={.metadata.uid}")
					uid, err := utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred())
					originalUID = strings.TrimSpace(uid)
				},
				func(ns string) {
					waitForRemediationJobSuccess("KubeContainerWaiting")

					By("verifying the original pod was deleted")
					Eventually(func() bool {
						cmd := exec.Command("kubectl", "get", "pod", "imagepull-victim", "-n", ns,
							"-o", "jsonpath={.metadata.uid}", "--ignore-not-found")
						newUID, err := utils.Run(cmd)
						if err != nil {
							return false
						}
						trimmed := strings.TrimSpace(newUID)
						return trimmed == "" || trimmed != originalUID
					}, 30*time.Second, 2*time.Second).Should(BeTrue(), "original imagepull-victim pod should be deleted")
				},
			)
		})
	})

	Context("KubePodNotReady", func() {
		const testNs = "e2e-kps-podnotready-test"

		BeforeEach(func() { createKPSTestNamespace(testNs) })
		AfterEach(func() { deleteKPSTestNamespace(testNs) })

		It("should restart a pod whose readiness probe never succeeds when the real alert fires", func() {
			var originalUID string

			runKPSScenario(testNs, "KubePodNotReady",
				func(ns string) {
					podYAML := fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: notready-victim
  namespace: %s
spec:
  containers:
  - name: main
    image: busybox:latest
    command: ["sleep", "3600"]
    readinessProbe:
      exec:
        command: ["false"]
      periodSeconds: 2
      failureThreshold: 1
  restartPolicy: Always
`, ns)
					Expect(utils.ApplyYAML(podYAML)).To(Succeed())

					By("waiting for the pod to be Running but never Ready")
					Eventually(func() bool {
						cmd := exec.Command("kubectl", "get", "pod", "notready-victim", "-n", ns,
							"-o", "jsonpath={.status.phase}:{.status.conditions[?(@.type=='Ready')].status}")
						output, err := utils.Run(cmd)
						if err != nil {
							return false
						}
						return strings.TrimSpace(output) == "Running:False"
					}, 60*time.Second, 2*time.Second).Should(BeTrue(), "pod should be Running but not Ready")

					cmd := exec.Command("kubectl", "get", "pod", "notready-victim", "-n", ns, "-o", "jsonpath={.metadata.uid}")
					uid, err := utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred())
					originalUID = strings.TrimSpace(uid)
				},
				func(ns string) {
					waitForRemediationJobSuccess("KubePodNotReady")

					By("verifying the original pod was deleted")
					Eventually(func() bool {
						cmd := exec.Command("kubectl", "get", "pod", "notready-victim", "-n", ns,
							"-o", "jsonpath={.metadata.uid}", "--ignore-not-found")
						newUID, err := utils.Run(cmd)
						if err != nil {
							return false
						}
						trimmed := strings.TrimSpace(newUID)
						return trimmed == "" || trimmed != originalUID
					}, 30*time.Second, 2*time.Second).Should(BeTrue(), "original notready-victim pod should be deleted")
				},
			)
		})
	})

	Context("KubeJobNotCompleted", func() {
		const testNs = "e2e-kps-jobnotcompleted-test"

		BeforeEach(func() { createKPSTestNamespace(testNs) })
		AfterEach(func() { deleteKPSTestNamespace(testNs) })

		It("should clean up a job stuck active past the (shortened) threshold when the real alert fires", func() {
			runKPSScenario(testNs, "KubeJobNotCompleted",
				func(ns string) {
					jobYAML := fmt.Sprintf(`
apiVersion: batch/v1
kind: Job
metadata:
  name: stuck-job
  namespace: %s
spec:
  backoffLimit: 0
  template:
    spec:
      containers:
      - name: main
        image: busybox:latest
        command: ["sleep", "3600"]
      restartPolicy: Never
`, ns)
					Expect(utils.ApplyYAML(jobYAML)).To(Succeed())

					By("waiting for the stuck job's pod to actually be running")
					Eventually(func() bool {
						cmd := exec.Command("kubectl", "get", "pods", "-n", ns,
							"-l", "job-name=stuck-job",
							"-o", "jsonpath={.items[0].status.phase}")
						output, err := utils.Run(cmd)
						if err != nil {
							return false
						}
						return strings.TrimSpace(output) == "Running"
					}, 60*time.Second, 2*time.Second).Should(BeTrue(), "stuck-job's pod should be running")
				},
				func(ns string) {
					waitForRemediationJobSuccess("KubeJobNotCompleted")

					By("verifying the stuck job was deleted")
					Eventually(func() bool {
						cmd := exec.Command("kubectl", "get", "job", "stuck-job", "-n", ns, "--ignore-not-found")
						output, err := utils.Run(cmd)
						if err != nil {
							return false
						}
						return strings.TrimSpace(output) == ""
					}, 30*time.Second, 2*time.Second).Should(BeTrue(), "stuck-job should have been deleted")
				},
			)
		})
	})

	Context("KubeHpaMaxedOut", func() {
		const testNs = "e2e-kps-hpamaxedout-test"

		BeforeEach(func() { createKPSTestNamespace(testNs) })
		AfterEach(func() { deleteKPSTestNamespace(testNs) })

		It("should increase maxReplicas when an HPA is genuinely pinned at max by the real alert", func() {
			var originalMax string

			runKPSScenario(testNs, "KubeHpaMaxedOut",
				func(ns string) {
					deployYAML := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hpa-maxedout-victim
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: hpa-maxedout-victim
  template:
    metadata:
      labels:
        app: hpa-maxedout-victim
    spec:
      containers:
      - name: main
        image: busybox:latest
        command: ["sleep", "3600"]
`, ns)
					Expect(utils.ApplyYAML(deployYAML)).To(Succeed())

					hpaYAML := fmt.Sprintf(`
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: hpa-maxedout-victim
  namespace: %s
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: hpa-maxedout-victim
  minReplicas: 1
  maxReplicas: 2
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 50
`, ns)
					Expect(utils.ApplyYAML(hpaYAML)).To(Succeed())

					By("forcing the deployment to its max replica count")
					// The container has no CPU request, so the HPA cannot compute
					// utilization and won't fight this manual scale - it will just
					// mirror whatever replica count the scale target reports.
					cmd := exec.Command("kubectl", "scale", "deployment/hpa-maxedout-victim", "-n", ns, "--replicas=2")
					_, err := utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred())

					By("waiting for the HPA to observe currentReplicas == maxReplicas")
					Eventually(func() string {
						cmd := exec.Command("kubectl", "get", "hpa", "hpa-maxedout-victim", "-n", ns,
							"-o", "jsonpath={.status.currentReplicas}")
						output, err := utils.Run(cmd)
						if err != nil {
							return ""
						}
						return strings.TrimSpace(output)
					}, 60*time.Second, 3*time.Second).Should(Equal("2"))

					cmd = exec.Command("kubectl", "get", "hpa", "hpa-maxedout-victim", "-n", ns, "-o", "jsonpath={.spec.maxReplicas}")
					max, err := utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred())
					originalMax = strings.TrimSpace(max)
				},
				func(ns string) {
					waitForRemediationJobSuccess("KubeHpaMaxedOut")

					By("verifying maxReplicas was increased")
					Eventually(func() string {
						cmd := exec.Command("kubectl", "get", "hpa", "hpa-maxedout-victim", "-n", ns,
							"-o", "jsonpath={.spec.maxReplicas}")
						output, err := utils.Run(cmd)
						if err != nil {
							return ""
						}
						return strings.TrimSpace(output)
					}, 30*time.Second, 2*time.Second).ShouldNot(Equal(originalMax), "maxReplicas should have been increased")
				},
			)
		})
	})

	// KubeDeploymentReplicasMismatch and KubeDaemonSetRolloutStuck are remediated
	// in production by a `kubectl rollout restart` + `kubectl rollout status
	// --timeout=300s` (see the respective operarius.yaml). A rollout restart can
	// only resolve *transient* problems; deliberately provoking a real,
	// Prometheus-evaluated version of these alerts requires a *persistent*
	// mismatch (e.g. a permanently-failing readiness probe), which a restart -
	// recreating the exact same broken pod template - cannot fix. Waiting for
	// job success here would mean waiting out the full 300s rollout-status
	// timeout (twice, given backoffLimit: 1) for no useful signal. Instead,
	// these two scenarios verify the remediation Job was created by a real
	// alert AND that it actually attempted the rollout restart (via its logs),
	// without requiring the underlying (unfixable-by-restart) condition to
	// resolve.

	Context("KubeDeploymentReplicasMismatch", func() {
		const testNs = "e2e-kps-deploymentmismatch-test"

		BeforeEach(func() { createKPSTestNamespace(testNs) })
		AfterEach(func() { deleteKPSTestNamespace(testNs) })

		It("should attempt a rollout restart when the real alert fires", func() {
			runKPSScenario(testNs, "KubeDeploymentReplicasMismatch",
				func(ns string) {
					deployYAML := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: replica-mismatch-victim
  namespace: %s
spec:
  replicas: 2
  selector:
    matchLabels:
      app: replica-mismatch-victim
  template:
    metadata:
      labels:
        app: replica-mismatch-victim
    spec:
      containers:
      - name: main
        image: busybox:latest
        command: ["sleep", "3600"]
        readinessProbe:
          exec:
            command: ["false"]
          periodSeconds: 2
          failureThreshold: 1
`, ns)
					Expect(utils.ApplyYAML(deployYAML)).To(Succeed())

					By("waiting for both replicas to be Running but never Available")
					Eventually(func() bool {
						cmd := exec.Command("kubectl", "get", "pods", "-n", ns,
							"-l", "app=replica-mismatch-victim",
							"-o", "jsonpath={.items[*].status.phase}")
						output, err := utils.Run(cmd)
						if err != nil {
							return false
						}
						phases := strings.Fields(strings.TrimSpace(output))
						if len(phases) != 2 {
							return false
						}
						return phases[0] == "Running" && phases[1] == "Running"
					}, 60*time.Second, 2*time.Second).Should(BeTrue(), "both replicas should be running (but not ready)")
				},
				func(ns string) {
					By("verifying the remediation job actually attempted a rollout restart")
					Eventually(func() string {
						return remediationJobLogs("KubeDeploymentReplicasMismatch")
					}, 60*time.Second, 3*time.Second).Should(ContainSubstring("Triggering rollout restart"))
				},
			)
		})
	})

	Context("KubeDaemonSetRolloutStuck", func() {
		const testNs = "e2e-kps-daemonsetstuck-test"

		BeforeEach(func() { createKPSTestNamespace(testNs) })
		AfterEach(func() { deleteKPSTestNamespace(testNs) })

		It("should attempt a rollout restart when the real alert fires", func() {
			runKPSScenario(testNs, "KubeDaemonSetRolloutStuck",
				func(ns string) {
					dsYAML := fmt.Sprintf(`
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: daemonset-stuck-victim
  namespace: %s
spec:
  selector:
    matchLabels:
      app: daemonset-stuck-victim
  updateStrategy:
    type: RollingUpdate
  template:
    metadata:
      labels:
        app: daemonset-stuck-victim
    spec:
      containers:
      - name: main
        image: busybox:latest
        command: ["sleep", "3600"]
        readinessProbe:
          exec:
            command: ["false"]
          periodSeconds: 2
          failureThreshold: 1
`, ns)
					Expect(utils.ApplyYAML(dsYAML)).To(Succeed())

					By("waiting for the DaemonSet pod to be scheduled but never Available")
					Eventually(func() bool {
						cmd := exec.Command("kubectl", "get", "pods", "-n", ns,
							"-l", "app=daemonset-stuck-victim",
							"-o", "jsonpath={.items[0].status.phase}")
						output, err := utils.Run(cmd)
						if err != nil {
							return false
						}
						return strings.TrimSpace(output) == "Running"
					}, 60*time.Second, 2*time.Second).Should(BeTrue(), "daemonset pod should be running (but not ready)")
				},
				func(ns string) {
					By("verifying the remediation job actually attempted a rollout restart")
					Eventually(func() string {
						return remediationJobLogs("KubeDaemonSetRolloutStuck")
					}, 60*time.Second, 3*time.Second).Should(ContainSubstring("Triggering rollout restart"))
				},
			)
		})
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

// runKPSScenario applies the REAL, shipped Operarius+RBAC for alertName from
// operarios/kube-prometheus-stack/<alertName>/ (rather than a hand-duplicated
// inline copy), provokes the real underlying Kubernetes condition, waits for
// the real Prometheus-evaluated alert to reach OpenFero and create a
// remediation Job, and then hands off to verify for scenario-specific checks.
func runKPSScenario(testNs, alertName string, provoke func(testNs string), verify func(testNs string)) {
	operariusDir := filepath.Join("operarios/kube-prometheus-stack", alertName)

	By("applying the real " + alertName + " Operarius + RBAC")
	rbacYAML := withNamespace(readRepoFile(filepath.Join(operariusDir, "rbac.yaml")))
	Expect(utils.ApplyYAML(rbacYAML)).To(Succeed())
	defer func() { _ = utils.DeleteYAML(rbacYAML) }()

	// The shipped operarius.yaml is a template with spec.enabled: false (see
	// operarios/kube-prometheus-stack/README.md) - flip it on here to
	// simulate a user who has reviewed and enabled it for their environment.
	operariusYAML := withEnabled(withNamespace(readRepoFile(filepath.Join(operariusDir, "operarius.yaml"))))
	Expect(utils.ApplyYAML(operariusYAML)).To(Succeed())
	defer func() { _ = utils.DeleteYAML(operariusYAML) }()

	// allow the Operarius controller to register the new resource
	time.Sleep(2 * time.Second)

	By("provoking a real " + alertName + " condition")
	provoke(testNs)

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
		"a remediation Job for %s should be created from a real, Prometheus-evaluated alert", alertName)

	By("verifying the remediation effect for " + alertName)
	verify(testNs)
}

// waitForRemediationJobSuccess waits for the (already-created) remediation
// Job for alertName to complete successfully. Only used by scenarios whose
// production remediation script always exits 0 regardless of whether the
// synthetic test condition is actually resolvable by that remediation
// (deleting a pod/job, or patching a field) - see the comment above the
// KubeDeploymentReplicasMismatch/KubeDaemonSetRolloutStuck Contexts for why
// those two scenarios don't use this.
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
// for alertName, or "" if it can't be found yet.
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
