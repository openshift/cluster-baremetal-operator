package baremetal

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[OTP][sig-baremetal] IPI BareMetal", func() {
	defer g.GinkgoRecover()
	var (
		oc = compat_otp.NewCLI("virtual-media-server-tls", compat_otp.KubeConfigPath())
	)

	g.BeforeEach(func() {
		SkipIfNotBaremetalCluster(oc)
		if !virtualMediaTLSFieldAvailable(oc) {
			g.Skip("provisioning.spec.virtualMediaServerTLS is not in this cluster's CRD")
		}
	})

	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-High-91125-Virtual media TLS listener matches the Provisioning spec [Level0]", func() {
		doc := getProvisioningTLS(oc)
		if doc.Spec.DisableVirtualMediaTLS {
			g.Skip("virtual media TLS is disabled")
		}

		env := runningHTTPDEnv(oc)
		spec := decodeVirtualMediaTLS(doc.Spec.VirtualMediaServerTLS)
		if spec == nil {
			g.By("Unset override keeps the virtual-media listener on the cluster TLS profile")
			o.Expect(env[vmediaProtocolEnv]).To(o.Equal(env[ironicProtocolEnv]))
			o.Expect(env[vmediaTLS12Env]).To(o.Equal(env[ironicTLS12Env]))
			o.Expect(env[vmediaTLS13Env]).To(o.Equal(env[ironicTLS13Env]))
			o.Expect(env[vmediaCurvesEnv]).To(o.Equal(env[ironicCurvesEnv]))
			return
		}

		g.By("Configured override is applied only to the virtual-media listener variables")
		protocol, ok := apacheProtocolForMinTLS(spec.MinTLSVersion)
		o.Expect(ok).To(o.BeTrue(), "unsupported minTLSVersion %q", spec.MinTLSVersion)
		o.Expect(env[vmediaProtocolEnv]).To(o.Equal(protocol))
		if len(spec.Ciphers) > 0 {
			tls12, tls13 := splitTLSCiphers(spec.Ciphers)
			o.Expect(env[vmediaTLS12Env]).To(o.Equal(strings.Join(tls12, ":")))
			o.Expect(env[vmediaTLS13Env]).To(o.Equal(strings.Join(tls13, ":")))
		}
		if len(spec.Groups) > 0 {
			o.Expect(env[vmediaCurvesEnv]).To(o.Equal(strings.Join(spec.Groups, ":")))
		}
	})

	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-High-91126-Virtual media TLS override does not change Ironic API TLS [Serial][Disruptive]", func(ctx context.Context) {
		doc := getProvisioningTLS(oc)
		if doc.Spec.DisableVirtualMediaTLS {
			g.By("Reject virtualMediaServerTLS while virtual media TLS is already disabled")
			out, err := patchProvisioning(oc, `{"spec":{"virtualMediaServerTLS":{"minTLSVersion":"VersionTLS12"}}}`, true)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("virtualMediaServerTLS cannot be set when disableVirtualMediaTLS is true"))
			g.Skip("virtual media TLS is disabled")
		}

		g.By("Reject virtual-media TLS settings the API does not allow")
		expectProvisioningPatchRejected(oc, `{"spec":{"virtualMediaServerTLS":{"minTLSVersion":"TLSv1.2"}}}`)
		expectProvisioningPatchRejected(oc, `{"spec":{"virtualMediaServerTLS":{"minTLSVersion":"VersionTLS12","ciphers":["ECDHE RSA"]}}}`)
		expectProvisioningPatchRejected(oc, `{"spec":{"virtualMediaServerTLS":{"minTLSVersion":"VersionTLS12","groups":["not-a-group"]}}}`)
		out, err := patchProvisioning(oc, `{"spec":{"disableVirtualMediaTLS":true,"virtualMediaServerTLS":{"minTLSVersion":"VersionTLS12"}}}`, true)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("virtualMediaServerTLS cannot be set when disableVirtualMediaTLS is true"))

		baseline := runningHTTPDEnv(oc)
		bmoBefore, err := bmoContainerArgs(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		original := append(json.RawMessage(nil), doc.Spec.VirtualMediaServerTLS...)

		defer func() {
			g.By("Restore the original virtual-media TLS setting")
			o.Expect(restoreVirtualMediaTLS(oc, original)).NotTo(o.HaveOccurred())
			o.Expect(virtualMediaTLSMatches(getProvisioningTLS(oc).Spec.VirtualMediaServerTLS, original)).To(o.BeTrue())
			restored, waitErr := waitForHTTPDEnv(oc, func(env map[string]string) bool {
				return env[vmediaProtocolEnv] == baseline[vmediaProtocolEnv] &&
					env[vmediaTLS12Env] == baseline[vmediaTLS12Env] &&
					env[vmediaTLS13Env] == baseline[vmediaTLS13Env] &&
					env[vmediaCurvesEnv] == baseline[vmediaCurvesEnv]
			})
			if waitErr != nil {
				e2e.Logf("virtual-media env after restore: %v", restored)
			}
			o.Expect(waitErr).NotTo(o.HaveOccurred())
		}()

		override := virtualMediaServerTLS{
			MinTLSVersion: "VersionTLS12",
			Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256", "TLS_AES_128_GCM_SHA256"},
			Groups:        []string{"secp384r1"},
		}
		g.By("Apply a virtual-media TLS override")
		patch, err := json.Marshal(map[string]any{"spec": map[string]any{"virtualMediaServerTLS": override}})
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = patchProvisioning(oc, string(patch), false)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait until metal3-httpd serves the override")
		env, err := waitForHTTPDEnv(oc, func(env map[string]string) bool {
			return env[vmediaProtocolEnv] == "-ALL +TLSv1.2 +TLSv1.3" &&
				env[vmediaTLS12Env] == "ECDHE-RSA-AES128-GCM-SHA256" &&
				env[vmediaTLS13Env] == "TLS_AES_128_GCM_SHA256" &&
				env[vmediaCurvesEnv] == "secp384r1"
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(env[ironicProtocolEnv]).To(o.Equal(baseline[ironicProtocolEnv]))
		o.Expect(env[ironicTLS12Env]).To(o.Equal(baseline[ironicTLS12Env]))
		o.Expect(env[ironicTLS13Env]).To(o.Equal(baseline[ironicTLS13Env]))
		o.Expect(env[ironicCurvesEnv]).To(o.Equal(baseline[ironicCurvesEnv]))

		bmoAfter, err := bmoContainerArgs(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(bmoAfter).To(o.Equal(bmoBefore))
		o.Expect(bmoAfter).NotTo(o.ContainSubstring("VMEDIA"))

		g.By("Virtual-media port accepts TLS 1.2 with the configured cipher")
		pod := runningMetal3Pod(oc)
		vmediaOut, vmediaErr := tlsHandshake(ctx, oc, pod, vmediaPort, "-tls1_2")
		o.Expect(handshakeAccepted(vmediaOut, vmediaErr)).To(o.BeTrue(), "virtual-media TLS 1.2 handshake failed: %s", handshakeText(vmediaOut, vmediaErr))

		if baseline[ironicProtocolEnv] == "-ALL +TLSv1.3" {
			g.By("Ironic API port still accepts TLS 1.3 and rejects TLS 1.2")
			apiTLS13Out, apiTLS13Err := tlsHandshake(ctx, oc, pod, ironicAPIPort, "-tls1_3")
			o.Expect(handshakeNegotiatedTLS13(apiTLS13Out, apiTLS13Err)).To(o.BeTrue(), "Ironic API TLS 1.3 handshake failed, so a TLS 1.2 rejection is not meaningful: %s", handshakeText(apiTLS13Out, apiTLS13Err))
			apiOut, apiErr := tlsHandshake(ctx, oc, pod, ironicAPIPort, "-tls1_2")
			o.Expect(handshakeRejected(apiOut, apiErr)).To(o.BeTrue(), "Ironic API did not reject TLS 1.2: %s", handshakeText(apiOut, apiErr))
		}

		g.By("Reject disabling virtual media TLS while the override is set")
		out, err = patchProvisioning(oc, `{"spec":{"disableVirtualMediaTLS":true}}`, true)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("virtualMediaServerTLS cannot be set when disableVirtualMediaTLS is true"))
	})

	// author: jhajyahy@redhat.com
	g.It("Author:jhajyahy-Longduration-NonPreRelease-High-91127-Worker reprovision uses the virtual-media TLS listener [Serial][Disruptive]", func() {
		doc := getProvisioningTLS(oc)
		if doc.Spec.DisableVirtualMediaTLS {
			g.Skip("virtual media TLS is disabled")
		}
		bmhName, machineName, machineSet := virtualMediaMachinesetWorker(oc)
		if bmhName == "" {
			g.Skip("no virtual-media worker BareMetalHost is owned by a MachineSet")
		}

		originReplicasStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machinesets.machine.openshift.io", machineSet, "-n", machineAPINamespace, "-o=jsonpath={.spec.replicas}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		originReplicas, err := strconv.Atoi(originReplicasStr)
		o.Expect(err).NotTo(o.HaveOccurred())
		if originReplicas < 1 {
			g.Skip("worker MachineSet has no replicas to reprovision")
		}

		baseline := runningHTTPDEnv(oc)
		original := append(json.RawMessage(nil), doc.Spec.VirtualMediaServerTLS...)
		defer func() {
			g.By("Restore the original virtual-media TLS setting")
			o.Expect(restoreVirtualMediaTLS(oc, original)).NotTo(o.HaveOccurred())
			o.Expect(virtualMediaTLSMatches(getProvisioningTLS(oc).Spec.VirtualMediaServerTLS, original)).To(o.BeTrue())
			_, waitErr := waitForHTTPDEnv(oc, func(env map[string]string) bool {
				return env[vmediaProtocolEnv] == baseline[vmediaProtocolEnv]
			})
			o.Expect(waitErr).NotTo(o.HaveOccurred())
		}()

		g.By("Apply a TLS 1.2 minimum for the virtual-media listener")
		patch, err := json.Marshal(map[string]any{
			"spec": map[string]any{"virtualMediaServerTLS": virtualMediaServerTLS{MinTLSVersion: "VersionTLS12"}},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = patchProvisioning(oc, string(patch), false)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = waitForHTTPDEnv(oc, func(env map[string]string) bool {
			return env[vmediaProtocolEnv] == "-ALL +TLSv1.2 +TLSv1.3"
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		annotated := false
		scaledDown := false
		defer func() {
			if !scaledDown {
				if annotated {
					_, annErr := oc.AsAdmin().WithoutNamespace().Run("annotate").Args("machines.machine.openshift.io", machineName, "machine.openshift.io/cluster-api-delete-machine-", "-n", machineAPINamespace).Output()
					if annErr != nil {
						e2e.Logf("removing delete-machine annotation from %s: %v", machineName, annErr)
					}
					o.Expect(annErr).NotTo(o.HaveOccurred())
				}
				return
			}
			g.By("Restore the worker MachineSet replica count")
			currentReplicasStr, replicaErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("machinesets.machine.openshift.io", machineSet, "-n", machineAPINamespace, "-o=jsonpath={.spec.replicas}").Output()
			o.Expect(replicaErr).NotTo(o.HaveOccurred())
			if currentReplicasStr != originReplicasStr {
				_, scaleErr := oc.AsAdmin().WithoutNamespace().Run("scale").Args("machinesets.machine.openshift.io", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%s", originReplicasStr)).Output()
				o.Expect(scaleErr).NotTo(o.HaveOccurred())
			}
			waitForBMHState(oc, bmhName, "provisioned")
			nodeHealthErr := clusterNodesHealthcheck(oc, 1500)
			compat_otp.AssertWaitPollNoErr(nodeHealthErr, "worker did not return Ready after virtual-media reprovision")
			operatorErr := clusterOperatorHealthcheck(oc, 1500, "OCP-91127.log")
			compat_otp.AssertWaitPollNoErr(operatorErr, "cluster operators did not recover after virtual-media reprovision")
		}()

		g.By("Deprovision one virtual-media worker")
		_, err = oc.AsAdmin().WithoutNamespace().Run("annotate").Args("machines.machine.openshift.io", machineName, "machine.openshift.io/cluster-api-delete-machine=yes", "-n", machineAPINamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		annotated = true
		_, err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("machinesets.machine.openshift.io", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%d", originReplicas-1)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		scaledDown = true
		waitForBMHState(oc, bmhName, "available")

		g.By("Provision the worker again through the virtual-media TLS listener")
		since := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
		_, err = oc.AsAdmin().WithoutNamespace().Run("scale").Args("machinesets.machine.openshift.io", machineSet, "-n", machineAPINamespace, fmt.Sprintf("--replicas=%s", originReplicasStr)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForBMHState(oc, bmhName, "provisioned")
		g.By("Wait until the reprovisioned worker node is Ready")
		waitForReprovisionedNodeReady(oc, bmhName)

		ironicLogs, ironicErr := metal3LogsSince(oc, "metal3-ironic", since)
		httpdLogs, httpdErr := metal3LogsSince(oc, vmediaContainer, since)
		if !strings.Contains(ironicLogs+"\n"+httpdLogs, ":"+vmediaPort) {
			o.Expect(ironicErr).NotTo(o.HaveOccurred())
			o.Expect(httpdErr).NotTo(o.HaveOccurred())
			e2e.Failf("provisioning logs since %s did not mention the virtual-media HTTPS port %s", since, vmediaPort)
		}
	})
})
