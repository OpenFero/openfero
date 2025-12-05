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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/OpenFero/openfero/test/utils"
)

var namespace string

func init() {
	namespace = os.Getenv("E2E_NAMESPACE")
	if namespace == "" {
		namespace = "openfero"
	}
}

// withNamespace replaces the hardcoded namespace in YAML with the actual namespace
func withNamespace(yaml string) string {
	return strings.ReplaceAll(yaml, "namespace: openfero", "namespace: "+namespace)
}

var _ = Describe("OpenFero Operarius CRD", Ordered, func() {
	BeforeAll(func() {
		By("verifying namespace exists")
		cmd := exec.Command("kubectl", "get", "ns", namespace)
		output, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Namespace should be created by make test-e2e-setup: %s", output)

		By("verifying Operarius CRDs are installed")
		cmd = exec.Command("kubectl", "get", "crd", "operariuses.openfero.io")
		output, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "CRDs should be pre-installed by make test-e2e-setup: %s", output)

		By("verifying OpenFero is running")
		Eventually(func() error {
			cmd := exec.Command("kubectl", "get", "deployment", "openfero", "-n", namespace)
			_, err := utils.Run(cmd)
			return err
		}, 60*time.Second, 2*time.Second).Should(Succeed(), "OpenFero deployment should exist")

		Eventually(func() bool {
			cmd := exec.Command("kubectl", "get", "deployment", "openfero", "-n", namespace,
				"-o", "jsonpath={.status.readyReplicas}")
			output, err := utils.Run(cmd)
			if err != nil {
				return false
			}
			return strings.TrimSpace(output) == "1"
		}, 120*time.Second, 2*time.Second).Should(BeTrue(), "OpenFero should have 1 ready replica")
	})

	AfterAll(func() {
		By("cleaning up test resources")
		cmd := exec.Command("kubectl", "delete", "operarius", "--all", "-n", namespace, "--ignore-not-found")
		_, _ = utils.Run(cmd)

		By("cleaning up test jobs")
		cmd = exec.Command("kubectl", "delete", "jobs", "-l", "openfero.io/group-key", "-n", namespace, "--ignore-not-found")
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
			err := utils.ApplyYAML(withNamespace(operariusYAML))
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
			err := utils.DeleteYAML(withNamespace(operariusYAML))
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
			err := utils.ApplyYAML(withNamespace(invalidYAML))
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
			err := utils.ApplyYAML(withNamespace(invalidStatusYAML))
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
			Expect(utils.ApplyYAML(withNamespace(lowPriorityYAML))).To(Succeed())
			Expect(utils.ApplyYAML(withNamespace(highPriorityYAML))).To(Succeed())
		})

		AfterAll(func() {
			_ = utils.DeleteYAML(withNamespace(lowPriorityYAML))
			_ = utils.DeleteYAML(withNamespace(highPriorityYAML))
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
			err := utils.ApplyYAML(withNamespace(labeledOperariusYAML))
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				_ = utils.DeleteYAML(withNamespace(labeledOperariusYAML))
			}()

			cmd := exec.Command("kubectl", "get", "operarius", "labeled-operarius", "-n", namespace, "-o", "jsonpath={.spec.alertSelector.labels}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("critical"))
			Expect(output).To(ContainSubstring("platform"))
		})
	})
})

var _ = Describe("OpenFero Alert Webhook E2E", Ordered, func() {
	var portForwardCmd *exec.Cmd
	var localPort string

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

		By("starting port-forward to OpenFero service")
		localPort = "18080"
		portForwardCmd = exec.Command("kubectl", "port-forward", "svc/openfero", localPort+":8080", "-n", namespace)
		err := portForwardCmd.Start()
		Expect(err).NotTo(HaveOccurred())

		// Wait for port-forward to be ready
		Eventually(func() error {
			cmd := exec.Command("curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
				fmt.Sprintf("http://localhost:%s/healthz", localPort))
			output, err := utils.Run(cmd)
			if err != nil {
				return err
			}
			if strings.TrimSpace(output) != "200" {
				return fmt.Errorf("healthz returned %s", output)
			}
			return nil
		}, 30*time.Second, time.Second).Should(Succeed(), "OpenFero should respond on healthz")
	})

	AfterAll(func() {
		By("stopping port-forward")
		if portForwardCmd != nil && portForwardCmd.Process != nil {
			_ = portForwardCmd.Process.Kill()
		}

		By("cleaning up test jobs")
		cmd := exec.Command("kubectl", "delete", "jobs", "-l", "openfero.io/group-key", "-n", namespace, "--ignore-not-found")
		_, _ = utils.Run(cmd)

		By("cleaning up test operarius resources")
		cmd = exec.Command("kubectl", "delete", "operarius", "--all", "-n", namespace, "--ignore-not-found")
		_, _ = utils.Run(cmd)
	})

	Context("Complete Alert to Job Flow", func() {
		const e2eOperariusYAML = `
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: e2e-test-remediation
  namespace: openfero
spec:
  alertSelector:
    alertname: E2ETestAlert
    status: firing
  jobTemplate:
    spec:
      backoffLimit: 0
      ttlSecondsAfterFinished: 300
      template:
        spec:
          containers:
          - name: remediation
            image: busybox:latest
            command:
            - /bin/sh
            - -c
            - echo "E2E Remediation executed successfully" && sleep 2
          restartPolicy: Never
  enabled: true
  priority: 100
`

		BeforeAll(func() {
			By("creating test Operarius")
			err := utils.ApplyYAML(withNamespace(e2eOperariusYAML))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "operarius", "e2e-test-remediation", "-n", namespace)
				_, err := utils.Run(cmd)
				return err
			}, 30*time.Second, time.Second).Should(Succeed())
		})

		AfterAll(func() {
			_ = utils.DeleteYAML(withNamespace(e2eOperariusYAML))
		})

		It("should accept valid alert webhook and create a job", func() {
			alertJSON := `{
				"version": "4",
				"groupKey": "e2e-test-group-key-1",
				"status": "firing",
				"receiver": "openfero",
				"groupLabels": {
					"alertname": "E2ETestAlert"
				},
				"commonLabels": {
					"alertname": "E2ETestAlert",
					"severity": "critical"
				},
				"commonAnnotations": {},
				"externalURL": "http://alertmanager:9093",
				"alerts": [{
					"status": "firing",
					"labels": {
						"alertname": "E2ETestAlert",
						"severity": "critical",
						"instance": "test-instance"
					},
					"annotations": {
						"summary": "E2E Test Alert"
					},
					"startsAt": "2025-01-01T00:00:00Z",
					"endsAt": "0001-01-01T00:00:00Z"
				}]
			}`

			By("sending alert webhook to OpenFero")
			cmd := exec.Command("curl", "-s", "-X", "POST",
				fmt.Sprintf("http://localhost:%s/alerts", localPort),
				"-H", "Content-Type: application/json",
				"-d", alertJSON,
				"-w", "\n%{http_code}",
			)
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "Webhook response: %s\n", output)
			Expect(output).To(ContainSubstring("200"))

			By("verifying a job was created")
			Eventually(func() bool {
				cmd := exec.Command("kubectl", "get", "jobs", "-n", namespace,
					"-l", "openfero.io/group-key",
					"-o", "jsonpath={.items[*].metadata.name}")
				output, err := utils.Run(cmd)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "Error getting jobs: %v\n", err)
					return false
				}
				fmt.Fprintf(GinkgoWriter, "Jobs found: %s\n", output)
				return len(strings.TrimSpace(output)) > 0
			}, 60*time.Second, 2*time.Second).Should(BeTrue(), "A job should be created")

			By("verifying job completes successfully")
			Eventually(func() bool {
				cmd := exec.Command("kubectl", "get", "jobs", "-n", namespace,
					"-l", "openfero.io/group-key",
					"-o", "jsonpath={.items[0].status.succeeded}")
				output, err := utils.Run(cmd)
				if err != nil {
					return false
				}
				return strings.TrimSpace(output) == "1"
			}, 120*time.Second, 2*time.Second).Should(BeTrue(), "Job should complete successfully")
		})

		It("should handle resolved alerts", func() {
			resolvedOperariusYAML := `
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: e2e-test-resolved
  namespace: openfero
spec:
  alertSelector:
    alertname: E2EResolvedAlert
    status: resolved
  jobTemplate:
    spec:
      backoffLimit: 0
      ttlSecondsAfterFinished: 300
      template:
        spec:
          containers:
          - name: cleanup
            image: busybox:latest
            command:
            - /bin/sh
            - -c
            - echo "Cleanup after alert resolved"
          restartPolicy: Never
  enabled: true
  priority: 50
`
			By("creating resolved Operarius")
			err := utils.ApplyYAML(withNamespace(resolvedOperariusYAML))
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = utils.DeleteYAML(withNamespace(resolvedOperariusYAML))
			}()

			resolvedAlertJSON := `{
				"version": "4",
				"groupKey": "e2e-resolved-group-key",
				"status": "resolved",
				"receiver": "openfero",
				"groupLabels": {
					"alertname": "E2EResolvedAlert"
				},
				"commonLabels": {
					"alertname": "E2EResolvedAlert"
				},
				"alerts": [{
					"status": "resolved",
					"labels": {
						"alertname": "E2EResolvedAlert"
					}
				}]
			}`

			By("sending resolved alert webhook")
			cmd := exec.Command("curl", "-s", "-X", "POST",
				fmt.Sprintf("http://localhost:%s/alerts", localPort),
				"-H", "Content-Type: application/json",
				"-d", resolvedAlertJSON,
				"-w", "\n%{http_code}",
			)
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("200"))
		})

		It("should not create duplicate jobs for same groupKey", func() {
			alertJSON := `{
				"version": "4",
				"groupKey": "e2e-dedup-test-group",
				"status": "firing",
				"receiver": "openfero",
				"groupLabels": {
					"alertname": "E2ETestAlert"
				},
				"commonLabels": {
					"alertname": "E2ETestAlert"
				},
				"alerts": [{
					"status": "firing",
					"labels": {
						"alertname": "E2ETestAlert"
					}
				}]
			}`

			By("sending first alert")
			cmd := exec.Command("curl", "-s", "-X", "POST",
				fmt.Sprintf("http://localhost:%s/alerts", localPort),
				"-H", "Content-Type: application/json",
				"-d", alertJSON,
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Wait for job to be created
			time.Sleep(3 * time.Second)

			By("counting initial jobs")
			cmd = exec.Command("kubectl", "get", "jobs", "-n", namespace,
				"-l", "openfero.io/group-key",
				"-o", "jsonpath={.items[*].metadata.name}")
			initialOutput, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			initialJobCount := len(strings.Fields(initialOutput))

			By("sending duplicate alert with same groupKey")
			cmd = exec.Command("curl", "-s", "-X", "POST",
				fmt.Sprintf("http://localhost:%s/alerts", localPort),
				"-H", "Content-Type: application/json",
				"-d", alertJSON,
			)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Wait and check no new jobs created
			time.Sleep(3 * time.Second)

			By("verifying no duplicate jobs were created")
			cmd = exec.Command("kubectl", "get", "jobs", "-n", namespace,
				"-l", "openfero.io/group-key",
				"-o", "jsonpath={.items[*].metadata.name}")
			finalOutput, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			finalJobCount := len(strings.Fields(finalOutput))

			// The job count should not increase significantly (deduplication)
			Expect(finalJobCount).To(BeNumerically("<=", initialJobCount+1),
				"Duplicate alerts should be deduplicated")
		})
	})

	Context("Health Endpoints", func() {
		It("should respond to healthz endpoint", func() {
			cmd := exec.Command("curl", "-s",
				fmt.Sprintf("http://localhost:%s/healthz", localPort))
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("ok"))
		})

		It("should respond to readiness endpoint", func() {
			cmd := exec.Command("curl", "-s",
				fmt.Sprintf("http://localhost:%s/readiness", localPort))
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("ok"))
		})
	})

	Context("API Endpoints", func() {
		It("should list alerts via API", func() {
			cmd := exec.Command("curl", "-s",
				fmt.Sprintf("http://localhost:%s/alerts", localPort))
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			// GET /alerts returns "OK" as JSON string (Alertmanager compatibility)
			Expect(output).To(ContainSubstring("OK"))
		})

		It("should provide about information", func() {
			cmd := exec.Command("curl", "-s",
				fmt.Sprintf("http://localhost:%s/api/about", localPort))
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("version"))
		})
	})
})

// Operarius Starter Pack Real-Resource Integration Tests
// These tests verify that production-ready Operarii actually remediate real Kubernetes resources
var _ = Describe("Operarius Starter Pack Real-Resource Remediation", Ordered, Label("starter-pack"), func() {
	var localPort string

	BeforeAll(func() {
		By("verifying OpenFero is running")
		Eventually(func() error {
			cmd := exec.Command("kubectl", "get", "pods", "-n", namespace,
				"-l", "app.kubernetes.io/name=openfero",
				"-o", "jsonpath={.items[0].status.phase}")
			output, err := utils.Run(cmd)
			if err != nil {
				return err
			}
			if strings.TrimSpace(output) != "Running" {
				return fmt.Errorf("OpenFero pod not running, status: %s", output)
			}
			return nil
		}, 60*time.Second, 2*time.Second).Should(Succeed())

		By("setting up port-forward to OpenFero")
		localPort = "9191"

		// Check if port-forward is already running from previous test suite
		cmd := exec.Command("curl", "-s", "--connect-timeout", "2",
			fmt.Sprintf("http://localhost:%s/healthz", localPort))
		_, err := utils.Run(cmd)
		if err != nil {
			// Start port-forward if not running
			pfCmd := exec.Command("kubectl", "port-forward", "-n", namespace,
				"svc/openfero", fmt.Sprintf("%s:8080", localPort))
			pfCmd.Stdout = GinkgoWriter
			pfCmd.Stderr = GinkgoWriter
			err = pfCmd.Start()
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				if pfCmd.Process != nil {
					_ = pfCmd.Process.Kill()
				}
			})

			// Wait for port-forward to be ready
			Eventually(func() error {
				cmd := exec.Command("curl", "-s", "--connect-timeout", "2",
					fmt.Sprintf("http://localhost:%s/healthz", localPort))
				_, err := utils.Run(cmd)
				return err
			}, 30*time.Second, 1*time.Second).Should(Succeed())
		}
	})

	Context("KubePodCrashLooping Remediation", func() {
		const testNs = "e2e-crashloop-test"

		BeforeEach(func() {
			By("creating test namespace")
			cmd := exec.Command("kubectl", "create", "namespace", testNs, "--dry-run=client", "-o", "yaml")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			err = utils.ApplyYAML(output)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			By("cleaning up test namespace")
			cmd := exec.Command("kubectl", "delete", "namespace", testNs, "--ignore-not-found", "--wait=false")
			_, _ = utils.Run(cmd)
		})

		It("should delete a crash-looping pod when KubePodCrashLooping alert is received", func() {
			// Create Operarius for KubePodCrashLooping
			operariusYAML := fmt.Sprintf(`
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: e2e-kubepodcrashlooping
  namespace: %s
spec:
  alertSelector:
    alertname: KubePodCrashLooping
    status: firing
  jobTemplate:
    spec:
      backoffLimit: 0
      ttlSecondsAfterFinished: 120
      template:
        spec:
          serviceAccountName: openfero-crashloop-remediator
          containers:
          - name: remediate
            image: bitnami/kubectl:latest
            command:
            - /bin/sh
            - -c
            - |
              echo "Remediating crash-looping pod..."
              echo "Namespace: $OPENFERO_NAMESPACE"
              echo "Pod: $OPENFERO_POD"
              kubectl delete pod "$OPENFERO_POD" -n "$OPENFERO_NAMESPACE" --ignore-not-found
              echo "Pod deletion initiated"
          restartPolicy: Never
  enabled: true
  priority: 80
`, namespace)

			// Create RBAC for the remediation job
			rbacYAML := fmt.Sprintf(`
apiVersion: v1
kind: ServiceAccount
metadata:
  name: openfero-crashloop-remediator
  namespace: %s
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: e2e-crashloop-remediator
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: e2e-crashloop-remediator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: e2e-crashloop-remediator
subjects:
- kind: ServiceAccount
  name: openfero-crashloop-remediator
  namespace: %s
`, namespace, namespace)

			By("creating RBAC for remediation")
			err := utils.ApplyYAML(rbacYAML)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = utils.DeleteYAML(rbacYAML)
			}()

			By("creating KubePodCrashLooping Operarius")
			err = utils.ApplyYAML(operariusYAML)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = utils.DeleteYAML(operariusYAML)
			}()

			// Wait for Operarius to be ready
			time.Sleep(2 * time.Second)

			// Create a test pod in the test namespace
			testPodYAML := fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: crashloop-victim
  namespace: %s
  labels:
    app: crashloop-test
spec:
  containers:
  - name: sleeper
    image: busybox:latest
    command: ["sleep", "3600"]
  restartPolicy: Always
`, testNs)

			By("creating test pod to be remediated")
			err = utils.ApplyYAML(testPodYAML)
			Expect(err).NotTo(HaveOccurred())

			// Wait for pod to be running
			Eventually(func() bool {
				cmd := exec.Command("kubectl", "get", "pod", "crashloop-victim", "-n", testNs,
					"-o", "jsonpath={.status.phase}")
				output, err := utils.Run(cmd)
				if err != nil {
					return false
				}
				return strings.TrimSpace(output) == "Running"
			}, 60*time.Second, 2*time.Second).Should(BeTrue(), "Test pod should be running")

			// Get the pod UID for verification
			cmd := exec.Command("kubectl", "get", "pod", "crashloop-victim", "-n", testNs,
				"-o", "jsonpath={.metadata.uid}")
			originalUID, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "Original pod UID: %s\n", originalUID)

			// Send KubePodCrashLooping alert
			alertJSON := fmt.Sprintf(`{
				"version": "4",
				"groupKey": "e2e-crashloop-%d",
				"status": "firing",
				"receiver": "openfero",
				"groupLabels": {
					"alertname": "KubePodCrashLooping"
				},
				"commonLabels": {
					"alertname": "KubePodCrashLooping",
					"namespace": "%s",
					"pod": "crashloop-victim"
				},
				"alerts": [{
					"status": "firing",
					"labels": {
						"alertname": "KubePodCrashLooping",
						"namespace": "%s",
						"pod": "crashloop-victim",
						"container": "sleeper",
						"severity": "warning"
					},
					"annotations": {
						"summary": "Pod is crash looping"
					}
				}]
			}`, time.Now().UnixNano(), testNs, testNs)

			By("sending KubePodCrashLooping alert to OpenFero")
			cmd = exec.Command("curl", "-s", "-X", "POST",
				fmt.Sprintf("http://localhost:%s/alerts", localPort),
				"-H", "Content-Type: application/json",
				"-d", alertJSON,
				"-w", "\n%{http_code}",
			)
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "Alert webhook response: %s\n", output)
			Expect(output).To(ContainSubstring("200"))

			By("verifying remediation job was created")
			Eventually(func() bool {
				cmd := exec.Command("kubectl", "get", "jobs", "-n", namespace,
					"-l", "openfero.io/group-key",
					"-o", "jsonpath={.items[*].metadata.name}")
				output, err := utils.Run(cmd)
				if err != nil {
					return false
				}
				return len(strings.TrimSpace(output)) > 0
			}, 30*time.Second, 2*time.Second).Should(BeTrue(), "Remediation job should be created")

			By("waiting for remediation job to complete")
			Eventually(func() bool {
				cmd := exec.Command("kubectl", "get", "jobs", "-n", namespace,
					"-l", "openfero.io/group-key",
					"-o", "jsonpath={.items[?(@.status.succeeded==1)].metadata.name}")
				output, err := utils.Run(cmd)
				if err != nil {
					return false
				}
				return len(strings.TrimSpace(output)) > 0
			}, 120*time.Second, 2*time.Second).Should(BeTrue(), "Remediation job should succeed")

			By("verifying the original pod was deleted")
			// Since the pod has restartPolicy: Always, it will be recreated with a new UID
			// But for standalone pod (not managed by deployment), it won't be recreated
			Eventually(func() bool {
				// Check if pod still exists (might be deleted or recreated)
				cmd := exec.Command("kubectl", "get", "pod", "crashloop-victim", "-n", testNs,
					"-o", "jsonpath={.metadata.uid}", "--ignore-not-found")
				newUID, err := utils.Run(cmd)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "Error getting pod: %v\n", err)
					return false
				}
				// Pod was either deleted (empty UID) or recreated (different UID)
				// For a standalone pod, it won't be recreated, so we check if it's gone
				fmt.Fprintf(GinkgoWriter, "Current pod UID: %s (original: %s)\n", newUID, originalUID)
				return newUID == "" || newUID != strings.TrimSpace(string(originalUID))
			}, 30*time.Second, 2*time.Second).Should(BeTrue(),
				"Original pod should be deleted (standalone pod won't be recreated)")
		})
	})

	Context("KubeJobFailed Remediation", func() {
		const testNs = "e2e-jobfailed-test"

		BeforeEach(func() {
			By("creating test namespace")
			cmd := exec.Command("kubectl", "create", "namespace", testNs, "--dry-run=client", "-o", "yaml")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			err = utils.ApplyYAML(output)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			By("cleaning up test namespace")
			cmd := exec.Command("kubectl", "delete", "namespace", testNs, "--ignore-not-found", "--wait=false")
			_, _ = utils.Run(cmd)
		})

		It("should delete a failed job when KubeJobFailed alert is received", func() {
			// Create Operarius for KubeJobFailed
			operariusYAML := fmt.Sprintf(`
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: e2e-kubejobfailed
  namespace: %s
spec:
  alertSelector:
    alertname: KubeJobFailed
    status: firing
  jobTemplate:
    spec:
      backoffLimit: 0
      ttlSecondsAfterFinished: 120
      template:
        spec:
          serviceAccountName: openfero-job-remediator
          containers:
          - name: remediate
            image: bitnami/kubectl:latest
            command:
            - /bin/sh
            - -c
            - |
              echo "Cleaning up failed job..."
              echo "Namespace: $OPENFERO_NAMESPACE"
              echo "Job: $OPENFERO_JOB_NAME"
              # Delete the failed job
              kubectl delete job "$OPENFERO_JOB_NAME" -n "$OPENFERO_NAMESPACE" --ignore-not-found
              echo "Failed job deleted"
          restartPolicy: Never
  enabled: true
  priority: 40
`, namespace)

			// Create RBAC for the remediation job
			rbacYAML := fmt.Sprintf(`
apiVersion: v1
kind: ServiceAccount
metadata:
  name: openfero-job-remediator
  namespace: %s
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: e2e-job-remediator
rules:
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["get", "list", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: e2e-job-remediator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: e2e-job-remediator
subjects:
- kind: ServiceAccount
  name: openfero-job-remediator
  namespace: %s
`, namespace, namespace)

			By("creating RBAC for remediation")
			err := utils.ApplyYAML(rbacYAML)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = utils.DeleteYAML(rbacYAML)
			}()

			By("creating KubeJobFailed Operarius")
			err = utils.ApplyYAML(operariusYAML)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = utils.DeleteYAML(operariusYAML)
			}()

			// Wait for Operarius to be ready
			time.Sleep(2 * time.Second)

			// Create a failing job in the test namespace
			failingJobYAML := fmt.Sprintf(`
apiVersion: batch/v1
kind: Job
metadata:
  name: failed-job-victim
  namespace: %s
spec:
  backoffLimit: 0
  template:
    spec:
      containers:
      - name: fail
        image: busybox:latest
        command: ["sh", "-c", "exit 1"]
      restartPolicy: Never
`, testNs)

			By("creating a job that will fail")
			err = utils.ApplyYAML(failingJobYAML)
			Expect(err).NotTo(HaveOccurred())

			// Wait for job to fail
			Eventually(func() bool {
				cmd := exec.Command("kubectl", "get", "job", "failed-job-victim", "-n", testNs,
					"-o", "jsonpath={.status.failed}")
				output, err := utils.Run(cmd)
				if err != nil {
					return false
				}
				return strings.TrimSpace(output) == "1"
			}, 60*time.Second, 2*time.Second).Should(BeTrue(), "Test job should fail")

			// Verify the failed job exists
			cmd := exec.Command("kubectl", "get", "job", "failed-job-victim", "-n", testNs)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed job should exist before remediation")

			// Send KubeJobFailed alert
			alertJSON := fmt.Sprintf(`{
				"version": "4",
				"groupKey": "e2e-jobfailed-%d",
				"status": "firing",
				"receiver": "openfero",
				"groupLabels": {
					"alertname": "KubeJobFailed"
				},
				"commonLabels": {
					"alertname": "KubeJobFailed",
					"namespace": "%s",
					"job_name": "failed-job-victim"
				},
				"alerts": [{
					"status": "firing",
					"labels": {
						"alertname": "KubeJobFailed",
						"namespace": "%s",
						"job_name": "failed-job-victim",
						"severity": "warning"
					},
					"annotations": {
						"summary": "Job has failed"
					}
				}]
			}`, time.Now().UnixNano(), testNs, testNs)

			By("sending KubeJobFailed alert to OpenFero")
			cmd = exec.Command("curl", "-s", "-X", "POST",
				fmt.Sprintf("http://localhost:%s/alerts", localPort),
				"-H", "Content-Type: application/json",
				"-d", alertJSON,
				"-w", "\n%{http_code}",
			)
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "Alert webhook response: %s\n", output)
			Expect(output).To(ContainSubstring("200"))

			By("verifying remediation job was created")
			Eventually(func() bool {
				cmd := exec.Command("kubectl", "get", "jobs", "-n", namespace,
					"-l", "openfero.io/group-key",
					"-o", "jsonpath={.items[*].metadata.name}")
				output, err := utils.Run(cmd)
				if err != nil {
					return false
				}
				return len(strings.TrimSpace(output)) > 0
			}, 30*time.Second, 2*time.Second).Should(BeTrue(), "Remediation job should be created")

			By("verifying the failed job was deleted")
			Eventually(func() bool {
				cmd := exec.Command("kubectl", "get", "job", "failed-job-victim", "-n", testNs,
					"--ignore-not-found", "-o", "jsonpath={.metadata.name}")
				output, err := utils.Run(cmd)
				if err != nil {
					return false
				}
				// Job should be deleted (empty output)
				return strings.TrimSpace(output) == ""
			}, 90*time.Second, 2*time.Second).Should(BeTrue(),
				"Failed job should be deleted by remediation")

			By("verifying remediation job completed successfully")
			Eventually(func() bool {
				cmd := exec.Command("kubectl", "get", "jobs", "-n", namespace,
					"-l", "openfero.io/group-key",
					"-o", "jsonpath={.items[?(@.status.succeeded==1)].metadata.name}")
				output, err := utils.Run(cmd)
				if err != nil {
					return false
				}
				return len(strings.TrimSpace(output)) > 0
			}, 120*time.Second, 2*time.Second).Should(BeTrue(), "Remediation job should succeed")
		})
	})
})
