package baremetal

import (
	"context"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[OTP][sig-baremetal] INSTALLER IPI for INSTALLER_GENERAL job on BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc           = compat_otp.NewCLI("ironic-prometheus-exporter", compat_otp.KubeConfigPath())
		iaasPlatform string
	)
	g.BeforeEach(func() {
		compat_otp.SkipForSNOCluster(oc)
		iaasPlatform = compat_otp.CheckPlatform(oc)
		if !(iaasPlatform == "baremetal") {
			e2e.Logf("Cluster is: %s", iaasPlatform)
			g.Skip("For Non-baremetal cluster , this is not supported!")
		}
	})

	// author: jhajyahy@redhat.com
	// port=unknown - no data in BigQuery last 60 days
	g.It("Author:jhajyahy-Medium-88191-Verify Ironic Prometheus Exporter can be enabled and metrics are exposed[Serial]", func() {
		g.By("Save current provisioning-configuration")
		_, err := oc.AsAdmin().Run("get").Args("provisioning", "provisioning-configuration", "-o=yaml").OutputToFile("prov-backup.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())

		// Save original prometheusExporter state
		originalExporterState, err := oc.AsAdmin().Run("get").Args("provisioning", "provisioning-configuration", "-o=jsonpath={.spec.prometheusExporter.enabled}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Original prometheusExporter enabled state: %s", originalExporterState)

		defer func() {
			g.By("Restore original provisioning configuration")
			// Restore to original state - either enabled or disabled
			if originalExporterState == "" || originalExporterState == "false" {
				e2e.Logf("Restoring prometheusExporter to disabled state")
				patchConfig := `{"spec":{"prometheusExporter":{"enabled":false}}}`
				restoreErr := oc.AsAdmin().Run("patch").Args("provisioning", "provisioning-configuration", "--type=merge", "-p", patchConfig).Execute()
				if restoreErr != nil {
					e2e.Logf("Warning: Failed to restore prometheusExporter state: %v", restoreErr)
				} else {
					// Wait for metal3 deployment to be available after restore
					e2e.Logf("Waiting for metal3 deployment to be available after restore")
					waitForDeployStatus(oc, "metal3", machineAPINamespace, "True")
				}
			} else if originalExporterState == "true" {
				e2e.Logf("Restoring prometheusExporter to enabled state")
				patchConfig := `{"spec":{"prometheusExporter":{"enabled":true}}}`
				restoreErr := oc.AsAdmin().Run("patch").Args("provisioning", "provisioning-configuration", "--type=merge", "-p", patchConfig).Execute()
				if restoreErr != nil {
					e2e.Logf("Warning: Failed to restore prometheusExporter state: %v", restoreErr)
				} else {
					// Wait for metal3 deployment to be available after restore
					e2e.Logf("Waiting for metal3 deployment to be available after restore")
					waitForDeployStatus(oc, "metal3", machineAPINamespace, "True")
				}
			}

			// Verify cluster baremetal operator is still available
			cboStatus, err := checkOperator(oc, "baremetal")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(cboStatus).To(o.BeTrue())
		}()

		g.By("Get current metal3 pod name before enabling IPE")
		oldMetal3Pod, err := oc.AsAdmin().Run("get").Args("pods", "-n", machineAPINamespace, "-l", "baremetal.openshift.io/cluster-baremetal-operator=metal3-state", "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Current metal3 pod before enabling IPE: %s", oldMetal3Pod)

		g.By("Enable Ironic Prometheus Exporter via provisioning configuration patch")
		patchConfig := `{"spec":{"prometheusExporter":{"enabled":true}}}`
		patchErr := oc.AsAdmin().Run("patch").Args("provisioning", "provisioning-configuration", "--type=merge", "-p", patchConfig).Execute()
		o.Expect(patchErr).NotTo(o.HaveOccurred())

		g.By("Verify prometheusExporter is enabled in provisioning configuration")
		exporterEnabled, err := oc.AsAdmin().Run("get").Args("provisioning", "provisioning-configuration", "-o=jsonpath={.spec.prometheusExporter.enabled}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(exporterEnabled).Should(o.Equal("true"))

		g.By("Wait for metal3 pod to be recreated after enabling IPE")
		var metal3Pod string
		err = wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 300*time.Second, true, func(context.Context) (bool, error) {
			newPod, err := oc.AsAdmin().Run("get").Args("pods", "-n", machineAPINamespace, "-l", "baremetal.openshift.io/cluster-baremetal-operator=metal3-state", "-o=jsonpath={.items[0].metadata.name}").Output()
			if err != nil {
				e2e.Logf("Failed to get metal3 pod: %v", err)
				return false, nil
			}

			// Check if pod name changed (recreated) and is Running
			if newPod != oldMetal3Pod && newPod != "" {
				podStatus := getPodStatus(oc, machineAPINamespace, newPod)
				if podStatus == "Running" {
					metal3Pod = newPod
					e2e.Logf("New metal3 pod %s is Running after IPE enabled", metal3Pod)
					return true, nil
				}
				e2e.Logf("New metal3 pod %s status: %s, waiting for Running state", newPod, podStatus)
				return false, nil
			}

			e2e.Logf("Metal3 pod not recreated yet (current: %s, old: %s), waiting...", newPod, oldMetal3Pod)
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "Metal3 pod was not recreated after enabling IPE")
		e2e.Logf("Metal3 pod recreated: %s", metal3Pod)

		g.By("Verify metal3-state service exists")
		metal3Svc, err := oc.AsAdmin().Run("get").Args("service", "-n", machineAPINamespace, "metal3-state", "-o=jsonpath={.metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(metal3Svc).Should(o.Equal("metal3-state"))
		e2e.Logf("Metal3 service: %s", metal3Svc)

		g.By("Verify Ironic Prometheus Exporter metrics endpoint is accessible")
		err = wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 120*time.Second, true, func(context.Context) (bool, error) {
			metricsCmd := "curl -s http://metal3-state.openshift-machine-api.svc:9608/metrics"
			cmd := []string{"-n", machineAPINamespace, metal3Pod, "-c", "metal3-ironic", "--", "/bin/sh", "-c", metricsCmd}
			metrics, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(cmd...).Output()
			if err != nil {
				e2e.Logf("Failed to curl metrics endpoint: %v", err)
				return false, nil
			}

			// Check if we got any metrics
			if metrics == "" {
				e2e.Logf("Metrics endpoint returned empty response, retrying...")
				return false, nil
			}

			// Verify we get Prometheus format metrics (should contain "# HELP" or metric names)
			if !strings.Contains(metrics, "# HELP") && !strings.Contains(metrics, "# TYPE") {
				e2e.Logf("Metrics endpoint did not return Prometheus format, retrying...")
				return false, nil
			}

			e2e.Logf("Ironic Prometheus Exporter metrics endpoint is accessible and returning data")
			return true, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "Failed to access metrics endpoint or metrics not available")

		g.By("Verify Ironic-specific metrics are exposed")
		metricsCmd := "curl -s http://metal3-state.openshift-machine-api.svc:9608/metrics | grep -E 'baremetal_|ironic_' | head -20"
		cmd := []string{"-n", machineAPINamespace, metal3Pod, "-c", "metal3-ironic", "--", "/bin/sh", "-c", metricsCmd}
		ironicMetrics, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(cmd...).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ironicMetrics).ShouldNot(o.BeEmpty(), "Expected to find Ironic-specific metrics (baremetal_* or ironic_*)")
		e2e.Logf("Sample Ironic metrics found:\n%s", ironicMetrics)

		g.By("Verify cluster baremetal operator is still available after enabling exporter")
		cboStatus, err := checkOperator(oc, "baremetal")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cboStatus).To(o.BeTrue())

		g.By("Test disabling Ironic Prometheus Exporter")
		oldMetal3PodBeforeDisable := metal3Pod
		e2e.Logf("Current metal3 pod before disabling IPE: %s", oldMetal3PodBeforeDisable)

		disablePatch := `{"spec":{"prometheusExporter":{"enabled":false}}}`
		disableErr := oc.AsAdmin().Run("patch").Args("provisioning", "provisioning-configuration", "--type=merge", "-p", disablePatch).Execute()
		o.Expect(disableErr).NotTo(o.HaveOccurred())

		g.By("Verify prometheusExporter is disabled in provisioning configuration")
		exporterDisabled, err := oc.AsAdmin().Run("get").Args("provisioning", "provisioning-configuration", "-o=jsonpath={.spec.prometheusExporter.enabled}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(exporterDisabled).Should(o.Equal("false"))

		g.By("Wait for metal3 pod to be recreated after disabling IPE")
		var newMetal3PodAfterDisable string
		err = wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 300*time.Second, true, func(context.Context) (bool, error) {
			newPod, err := oc.AsAdmin().Run("get").Args("pods", "-n", machineAPINamespace, "-l", "baremetal.openshift.io/cluster-baremetal-operator=metal3-state", "-o=jsonpath={.items[0].metadata.name}").Output()
			if err != nil {
				e2e.Logf("Failed to get metal3 pod: %v", err)
				return false, nil
			}

			// Check if pod name changed (recreated) and is Running
			if newPod != oldMetal3PodBeforeDisable && newPod != "" {
				podStatus := getPodStatus(oc, machineAPINamespace, newPod)
				if podStatus == "Running" {
					newMetal3PodAfterDisable = newPod
					e2e.Logf("New metal3 pod %s is Running after IPE disabled", newMetal3PodAfterDisable)
					return true, nil
				}
				e2e.Logf("New metal3 pod %s status: %s, waiting for Running state", newPod, podStatus)
				return false, nil
			}

			e2e.Logf("Metal3 pod not recreated yet (current: %s, old: %s), waiting...", newPod, oldMetal3PodBeforeDisable)
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "Metal3 pod was not recreated after disabling IPE")

		g.By("Verify metrics endpoint is no longer accessible after disabling")
		// Poll for the endpoint to become inaccessible (accounts for brief window during config rollout)
		err = wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 60*time.Second, true, func(context.Context) (bool, error) {
			metricsCmd := "curl -s -o /dev/null -w '%{http_code}' http://metal3-state.openshift-machine-api.svc:9608/metrics"
			cmd := []string{"-n", machineAPINamespace, newMetal3PodAfterDisable, "-c", "metal3-ironic", "--", "/bin/sh", "-c", metricsCmd}
			httpCode, execErr := oc.AsAdmin().WithoutNamespace().Run("exec").Args(cmd...).Output()

			// Success condition: endpoint returns non-200 or command fails
			if execErr != nil || httpCode != "200" {
				e2e.Logf("Metrics endpoint is no longer accessible (HTTP code: %s, error: %v) - expected", httpCode, execErr)
				return true, nil
			}

			e2e.Logf("Metrics endpoint still accessible (HTTP %s), waiting for it to become inaccessible...", httpCode)
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "Metrics endpoint is still accessible after disabling IPE - expected it to become inaccessible")
		e2e.Logf("Metrics endpoint is no longer accessible after disabling IPE (expected)")

		g.By("Verify cluster baremetal operator is still available after disabling exporter")
		cboStatus, err = checkOperator(oc, "baremetal")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cboStatus).To(o.BeTrue())
	})
})
