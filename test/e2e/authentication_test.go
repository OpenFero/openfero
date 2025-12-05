//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/OpenFero/openfero/test/utils"
)

var _ = Describe("Authentication", Ordered, func() {
	BeforeAll(func() {
		By("Installing Alertmanager")
		// Add repo
		cmd := exec.Command("helm", "repo", "add", "prometheus-community", "https://prometheus-community.github.io/helm-charts")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())

		cmd = exec.Command("helm", "repo", "update")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())

		// Install Alertmanager (minimal)
		// We use a simple values file or set flags
		cmd = exec.Command("helm", "upgrade", "--install", "alertmanager", "prometheus-community/alertmanager",
			"--namespace", namespace,
			"--set", "service.type=ClusterIP",
			"--set", "persistentVolume.enabled=false",
			"--wait")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())

		// Create a curl pod for triggering alerts
		cmd = exec.Command("kubectl", "run", "curl", "--image=curlimages/curl", "--restart=Never", "-n", namespace, "--", "sleep", "3600")
		utils.Run(cmd) // Ignore error if already exists

		// Wait for curl pod
		Eventually(func() error {
			cmd := exec.Command("kubectl", "wait", "--for=condition=Ready", "pod/curl", "-n", namespace, "--timeout=60s")
			_, err := utils.Run(cmd)
			return err
		}, 120*time.Second, 5*time.Second).Should(Succeed())
	})

	AfterAll(func() {
		// Uninstall Alertmanager
		cmd := exec.Command("helm", "uninstall", "alertmanager", "--namespace", namespace)
		_, _ = utils.Run(cmd)

		// Delete curl pod
		cmd = exec.Command("kubectl", "delete", "pod", "curl", "-n", namespace, "--ignore-not-found")
		_, _ = utils.Run(cmd)

		// Restore OpenFero to no auth
		upgradeOpenFero("none", "", "", "")
	})

	Context("No Authentication", func() {
		It("should accept alerts without auth", func() {
			upgradeOpenFero("none", "", "", "")
			configureAlertmanager("http://openfero:8080/alerts", "", "", "")

			// Verify connectivity from curl pod
			cmd := exec.Command("kubectl", "exec", "curl", "-n", namespace, "--", "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "http://openfero:8080/healthz")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to reach OpenFero from curl pod: %s", output)
			Expect(output).To(Equal("200"), "OpenFero healthz check failed")

			// Test direct webhook to verify OpenFero is listening and logging
			By("Sending direct webhook to OpenFero")
			payload := fmt.Sprintf(`{
				"version": "4",
				"groupKey": "test",
				"status": "firing",
				"receiver": "openfero",
				"groupLabels": {"alertname": "DirectTestAlert"},
				"commonLabels": {"alertname": "DirectTestAlert"},
				"alerts": [{"status": "firing", "labels": {"alertname": "DirectTestAlert"}}]
			}`)
			cmd = exec.Command("kubectl", "exec", "curl", "-n", namespace, "--", "curl", "-XPOST", "-H", "Content-Type: application/json", "-d", payload, "http://openfero:8080/alerts")
			output, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to send direct webhook: %s", output)
			verifyAlertReceived("DirectTestAlert")

			triggerAlert("TestAlertNoAuth")

			// Debug: Check Alertmanager alerts
			cmd = exec.Command("kubectl", "exec", "curl", "-n", namespace, "--", "curl", "-s", "http://alertmanager:9093/api/v2/alerts")
			output, err = utils.Run(cmd)
			fmt.Printf("Alertmanager alerts: %s\n", output)

			verifyAlertReceived("TestAlertNoAuth")
		})
	})

	Context("Basic Authentication", func() {
		It("should accept alerts with correct basic auth", func() {
			user := "admin"
			pass := "secret"
			upgradeOpenFero("basic", user, pass, "")
			configureAlertmanager("http://openfero.openfero.svc:8080/alerts", user, pass, "")
			triggerAlert("TestAlertBasicAuth")
			verifyAlertReceived("TestAlertBasicAuth")
		})

		It("should reject alerts with incorrect basic auth", func() {
			upgradeOpenFero("basic", "admin", "secretpass", "")
			configureAlertmanager("http://openfero.openfero.svc:8080/alerts", "admin", "wrongpass", "")
			triggerAlert("TestAlertBasicAuthFail")
			verifyAlertRejected("TestAlertBasicAuthFail")
		})
	})

	Context("Bearer Token Authentication", func() {
		It("should accept alerts with correct bearer token", func() {
			token := "secret-token"
			upgradeOpenFero("bearer", "", "", token)
			configureAlertmanager("http://openfero.openfero.svc:8080/alerts", "", "", token)
			triggerAlert("TestAlertBearerAuth")
			verifyAlertReceived("TestAlertBearerAuth")
		})
	})
})

func upgradeOpenFero(method, user, pass, token string) {
	By(fmt.Sprintf("Upgrading OpenFero with auth method: %s", method))
	args := []string{"upgrade", "--install", "openfero", "charts/openfero",
		"--namespace", namespace,
		"--set", "image.repository=openfero",
		"--set", "image.tag=e2e-test",
		"--set", "image.pullPolicy=Never",
		"--set", "serviceMonitor.enabled=false",
		"--set", fmt.Sprintf("auth.method=%s", method),
		"--set", "customArgs[0]=--logLevel=debug",
		"--wait",
	}

	if method == "none" {
		args = append(args, "--set", "auth.enabled=false")
	} else {
		args = append(args, "--set", "auth.enabled=true")
		if method == "basic" {
			args = append(args,
				"--set", fmt.Sprintf("auth.basic.username=%s", user),
				"--set", fmt.Sprintf("auth.basic.password=%s", pass),
			)
		} else if method == "bearer" {
			args = append(args,
				"--set", fmt.Sprintf("auth.bearer.token=%s", token),
			)
		}
	}

	cmd := exec.Command("helm", args...)
	output, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Helm upgrade failed: %s", output)
}

func configureAlertmanager(url, user, pass, token string) {
	By("Configuring Alertmanager")

	webhookConfig := fmt.Sprintf(`    - url: '%s'
      send_resolved: true`, url)

	if user != "" {
		webhookConfig += fmt.Sprintf(`
      http_config:
        basic_auth:
          username: '%s'
          password: '%s'`, user, pass)
	} else if token != "" {
		webhookConfig += fmt.Sprintf(`
      http_config:
        authorization:
          type: Bearer
          credentials: '%s'`, token)
	}

	config := fmt.Sprintf(`config:
  global:
    resolve_timeout: 5m
  route:
    group_by: ['alertname']
    group_wait: 0s
    group_interval: 1s
    repeat_interval: 1s
    receiver: 'openfero'
  receivers:
  - name: 'openfero'
    webhook_configs:
%s
`, webhookConfig)

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "am-values-*.yaml")
	Expect(err).NotTo(HaveOccurred())
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(config)
	Expect(err).NotTo(HaveOccurred())
	err = tmpFile.Close()
	Expect(err).NotTo(HaveOccurred())

	cmd := exec.Command("helm", "upgrade", "alertmanager", "prometheus-community/alertmanager",
		"--namespace", namespace,
		"--set", "service.type=ClusterIP",
		"--set", "persistentVolume.enabled=false",
		"-f", tmpFile.Name(),
		"--wait")
	output, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Alertmanager configuration failed: %s", output)

	// Wait a bit for Alertmanager to reload config
	time.Sleep(5 * time.Second)
}

func triggerAlert(alertName string) {
	By(fmt.Sprintf("Triggering alert: %s", alertName))
	// Use Alertmanager API to trigger an alert
	// POST /api/v2/alerts
	payload := fmt.Sprintf(`[{"labels":{"alertname":"%s","severity":"critical"}}]`, alertName)
	cmd := exec.Command("kubectl", "exec", "curl", "-n", namespace, "--", "curl", "-XPOST", "-H", "Content-Type: application/json", "-d", payload, "http://alertmanager:9093/api/v2/alerts")
	output, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to trigger alert: %s", output)
}

func dumpAlertmanagerLogs() string {
	cmd := exec.Command("kubectl", "logs", "-l", "app.kubernetes.io/name=alertmanager", "-n", namespace, "--tail=100")
	output, _ := utils.Run(cmd)
	return output
}

func verifyAlertReceived(alertName string) {
	By(fmt.Sprintf("Verifying alert received: %s", alertName))
	Eventually(func() string {
		cmd := exec.Command("kubectl", "logs", "-l", "app.kubernetes.io/name=openfero", "-n", namespace, "--tail=100")
		output, _ := utils.Run(cmd)
		return output
	}, 60*time.Second, 2*time.Second).Should(ContainSubstring(alertName), "Alertmanager logs:\n%s", dumpAlertmanagerLogs())
}

func verifyAlertRejected(alertName string) {
	By(fmt.Sprintf("Verifying alert rejected: %s", alertName))
	// We check that the alert does NOT appear in logs for a while
	Consistently(func() string {
		cmd := exec.Command("kubectl", "logs", "-l", "app.kubernetes.io/name=openfero", "-n", namespace, "--tail=100")
		out, _ := utils.Run(cmd)
		return out
	}, 10*time.Second, 1*time.Second).ShouldNot(ContainSubstring(alertName))

	// Also check for "Authentication failed" log
	Eventually(func() string {
		cmd := exec.Command("kubectl", "logs", "-l", "app.kubernetes.io/name=openfero", "-n", namespace, "--tail=100")
		out, _ := utils.Run(cmd)
		return out
	}, 10*time.Second, 1*time.Second).Should(ContainSubstring("Authentication failed"))
}
