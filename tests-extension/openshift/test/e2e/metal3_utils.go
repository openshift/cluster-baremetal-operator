package baremetal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	machineAPINamespace = "openshift-machine-api"
)

func skipIfNotBaremetal(oc *exutil.CLI) {
	platform := compat_otp.CheckPlatform(oc)
	if platform != "baremetal" {
		g.Skip(fmt.Sprintf("Cluster is %s, not baremetal - skipping", platform))
	}
}

func getWorkerBMH(oc *exutil.CLI) (host string, machineName string) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"machines.machine.openshift.io", "-n", machineAPINamespace,
		"-l", "machine.openshift.io/cluster-api-machine-role=worker",
		"-o=jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}",
	).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	workerMachines := strings.Split(strings.TrimSpace(output), "\n")
	if len(workerMachines) == 0 || workerMachines[0] == "" {
		g.Skip("No worker machines found")
	}
	machineName = workerMachines[len(workerMachines)-1]

	bmhOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"bmh", "-n", machineAPINamespace,
		`-o=jsonpath={range .items[*]}{.metadata.name},{.spec.consumerRef.name}{"\n"}{end}`,
	).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, line := range strings.Split(strings.TrimSpace(bmhOutput), "\n") {
		parts := strings.SplitN(line, ",", 2)
		if len(parts) == 2 && parts[1] == machineName {
			host = parts[0]
			break
		}
	}
	if host == "" {
		e2e.Failf("No BMH found for worker machine %s", machineName)
	}
	return host, machineName
}

func waitForBMHState(oc *exutil.CLI, bmhName string, bmhStatus string) {
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 90*time.Minute, true, func(ctx context.Context) (bool, error) {
		out, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "bmh", bmhName, "-o=jsonpath={.status.provisioning.state}").Output()
		if err != nil {
			return false, err
		}
		if !strings.Contains(out, bmhStatus) {
			e2e.Logf("bmh %v state is %v, Trying again", bmhName, out)
			return false, nil
		}
		e2e.Logf("bmh %v state is %v", bmhName, out)
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("The BMH state of %v is not as expected", bmhName))
}

func waitForBMHDeletion(oc *exutil.CLI, bmhName string) {
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 30*time.Minute, true, func(ctx context.Context) (bool, error) {
		out, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "bmh", "-o=jsonpath={.items[*].metadata.name}").Output()
		if err != nil {
			return false, err
		}
		if !strings.Contains(out, bmhName) {
			e2e.Logf("bmh %v no longer exists", bmhName)
			return true, nil
		}
		e2e.Logf("bmh %v exists, Trying again", bmhName)
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "The BMH was not deleted as expected")
}

func getFirstDeviceName(oc *exutil.CLI, bmhName string) string {
	deviceName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("hardwaredata", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.hardware.storage[0].name}").Output()
	o.Expect(err).ShouldNot(o.HaveOccurred())
	return deviceName
}

// getHfsToggleValue returns a vendor-specific HostFirmwareSettings field name and its toggled value.
// This is used for testing firmware setting changes by returning the opposite of the current value.
// For Dell: uses "LogicalProc", for HPE: uses "NetworkBootRetry".
// Returns (settingName, toggledValue, error) where toggledValue is the opposite of current
// (Enabled → Disabled, Disabled → Enabled).
func getHfsToggleValue(oc *exutil.CLI, vendor, machineAPINamespace, host string) (string, string, error) {
	var settingName, toggledValue, currentValue string
	var err error

	switch vendor {
	case "Dell Inc.":
		settingName = "LogicalProc"
	case "HPE":
		settingName = "NetworkBootRetry"
	default:
		e2e.Failf("Unsupported vendor: %s", vendor)
	}

	currentValue, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("hfs", "-n", machineAPINamespace, host, fmt.Sprintf("-o=jsonpath={.status.settings.%s}", settingName)).Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch current status for %s: %v", settingName, err)
	}

	// Normalize and validate current value before toggling
	normalizedValue := strings.ToLower(strings.TrimSpace(currentValue))
	switch normalizedValue {
	case "enabled":
		toggledValue = "Disabled"
	case "disabled":
		toggledValue = "Enabled"
	default:
		return "", "", fmt.Errorf("unexpected firmware setting value %q for %s (expected 'Enabled' or 'Disabled')", currentValue, settingName)
	}

	return settingName, toggledValue, nil
}

func waitForBMHError(oc *exutil.CLI, bmhName string, errorMessage string) {
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 30*time.Minute, true, func(ctx context.Context) (bool, error) {
		out, err := oc.AsAdmin().Run("get").Args("-n", machineAPINamespace, "bmh", bmhName, "-o=jsonpath={.status.errorMessage}").Output()
		if err != nil {
			return false, err
		}
		if !strings.Contains(out, errorMessage) {
			e2e.Logf("bmh %v error message is %v, Trying again", bmhName, out)
			return false, nil
		}
		e2e.Logf("bmh %v error message is %v", bmhName, out)
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("The BMH error of %v is not as expected", bmhName))
}

func getNicFwDetails(vendor, currentVersion string) (string, string) {
	bcm_226_226 := "https://docs.broadcom.com/docs-and-downloads/ethernet-network-adapters/NXE/Thor2/GCA2/bcm5751x-v22.6.226-esxi.zip"
	bcm_226_250 := "https://docs.broadcom.com/docs-and-downloads/ethernet-network-adapters/NXE/Thor2/GCA2/bcm5751x-v22.6.250-esxi.zip"
	mlx_28_40_1000 := "https://www.mellanox.com/downloads/firmware/fw-ConnectX7-rel-28_40_1000-MCX75510AAS-FEA_Ax-UEFI-14.34.11-FlexBoot-3.7.604.bin.zip"
	mlx_28_39_1014 := "https://www.mellanox.com/downloads/firmware/fw-ConnectX7-rel-28_39_1014-MCX75510AAS-FEA_Ax-UEFI-14.33.11-FlexBoot-3.7.504.bin.zip"

	switch vendor {
	case "Broadcom Inc. and subsidiaries":
		switch currentVersion {
		case "22.6.226":
			return bcm_226_250, "bcm5751x-v22.6.250-esxi.zip"
		case "22.6.250":
			return bcm_226_226, "bcm5751x-v22.6.226-esxi.zip"
		default:
			e2e.Failf("Unsupported Broadcom NIC firmware version: %s", currentVersion)
			return "", ""
		}
	case "Mellanox Technologies":
		switch currentVersion {
		case "28.40.1000":
			return mlx_28_39_1014, "fw-ConnectX7-rel-28_39_1014.bin"
		case "28.39.1014":
			return mlx_28_40_1000, "fw-ConnectX7-rel-28_40_1000.bin"
		default:
			e2e.Failf("Unsupported Mellanox NIC firmware version: %s", currentVersion)
			return "", ""
		}
	default:
		e2e.Failf("Unsupported NIC vendor: %s", vendor)
		return "", ""
	}
}

func getNicNameByVendor(vendor string) string {
	switch vendor {
	case "Broadcom Inc. and subsidiaries":
		return "BCM5720"
	case "Mellanox Technologies":
		return "ConnectX-7"
	default:
		e2e.Failf("Unsupported NIC vendor for name lookup: %s", vendor)
		return ""
	}
}

// getBypathDeviceName retrieves the first storage device's by-path name from HardwareData
func getBypathDeviceName(oc *exutil.CLI, bmhName string) string {
	deviceName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("hardwaredata", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.spec.hardware.storage[0].name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get device name from HardwareData: %s", bmhName)
	o.Expect(deviceName).NotTo(o.BeEmpty(), "Device name should not be empty in HardwareData: %s", bmhName)
	return deviceName
}

// getHfsByVendor returns vendor-specific HostFirmwareSettings field and its toggled value
// This is a convenience wrapper around getHfsToggleValue for tests that use this naming
func getHfsByVendor(oc *exutil.CLI, vendor, machineAPINamespace, host string) (string, string, error) {
	return getHfsToggleValue(oc, vendor, machineAPINamespace, host)
}

const bastionFirmwareBaseURL = "http://192.168.70.1/firmware"

func bastionBmcFirmwareURL(vendor, currentVersion string) string {
	switch vendor {
	case "Dell Inc.":
		switch currentVersion {
		case "7.00.00.183":
			return bastionFirmwareBaseURL + "/iDRAC-with-Lifecycle-Controller_Firmware_FWMWV_WN64_7.00.00.184_A00.EXE"
		case "7.00.00.184":
			return bastionFirmwareBaseURL + "/iDRAC-with-Lifecycle-Controller_Firmware_VP556_WN64_7.00.00.183_A00.EXE"
		case "7.30.10.50":
			return bastionFirmwareBaseURL + "/iDRAC-with-Lifecycle-Controller_Firmware_CPCHX_WN64_7.30.30.51_A00.EXE"
		case "7.30.30.51":
			return bastionFirmwareBaseURL + "/iDRAC-with-Lifecycle-Controller_Firmware_924YT_WN64_7.30.10.50_A00.EXE"
		default:
			e2e.Failf("Unsupported Dell iDRAC version for bastion firmware: %s", currentVersion)
		}
	case "HPE":
		if strings.Contains(currentVersion, "iLO 5") {
			switch currentVersion {
			case "iLO 5 v3.02":
				return bastionFirmwareBaseURL + "/ilo5_305.fwpkg"
			case "iLO 5 v3.05":
				return bastionFirmwareBaseURL + "/ilo5_321.fwpkg"
			case "iLO 5 v3.21":
				return bastionFirmwareBaseURL + "/ilo5_305.fwpkg"
			default:
				e2e.Failf("Unsupported iLO 5 version for bastion firmware: %s", currentVersion)
			}
		} else if strings.Contains(currentVersion, "iLO 6") {
			e2e.Failf("No iLO 6 firmware staged on bastion for version: %s", currentVersion)
		} else {
			e2e.Failf("Unsupported HPE BMC version for bastion firmware: %s", currentVersion)
		}
	default:
		e2e.Failf("Unsupported vendor for bastion firmware: %s", vendor)
	}
	return ""
}

func bastionBiosFirmwareURL(vendor, currentVersion string) string {
	switch vendor {
	case "Dell Inc.":
		switch currentVersion {
		case "2.26.1":
			return bastionFirmwareBaseURL + "/BIOS_N5C9K_WN64_2.27.0_01.EXE"
		case "2.27.0":
			return bastionFirmwareBaseURL + "/BIOS_W3XYW_WN64_2.26.1.EXE"
		case "1.20.2":
			return bastionFirmwareBaseURL + "/BIOS_DCFN9_WN64_1.21.1.EXE"
		case "1.21.1":
			return bastionFirmwareBaseURL + "/BIOS_GWT21_WN64_1.20.2.EXE"
		default:
			e2e.Failf("Unsupported Dell BIOS version for bastion firmware: %s", currentVersion)
		}
	case "HPE":
		for _, ver := range []string{"U46 v2.24", "U46 v2.42"} {
			if currentVersion == ver || strings.HasPrefix(currentVersion, ver+" ") {
				return bastionFirmwareBaseURL + "/U46_2.64_04_01_2026.fwpkg"
			}
		}
		if currentVersion == "U46 v2.64" || strings.HasPrefix(currentVersion, "U46 v2.64 ") {
			return bastionFirmwareBaseURL + "/U46_2.24_10_04_2024.fwpkg"
		}
		e2e.Failf("Unsupported HPE BIOS version for bastion firmware: %s", currentVersion)
	default:
		e2e.Failf("Unsupported vendor for bastion BIOS firmware: %s", vendor)
	}
	return ""
}

var nicFirmwareArtifacts = map[string]map[string]string{
	"Dell Inc.": {
		"16.35.80.02": "/Network_Firmware_XY16R_WN64_16.35.30.06_01.EXE",
		"16.35.30.06": "/Network_Firmware_P5F14_WN64_16.35.80.02_A00.EXE",
	},
	// HPE ConnectX-6 Lx OCP3 (MCX631432AS-ADAI). 26.32.2004 is unpublished; 26.32.1010 is the HPE package.
	"HPE": {
		"26.32.2004": "/26_35_1012-MCX631432AS-ADA_Ax.pldm.fwpkg",
		"26.32.1010": "/26_35_1012-MCX631432AS-ADA_Ax.pldm.fwpkg",
		"26.35.1012": "/26_32_1010-MCX631432AS-ADA_Ax.pldm.fwpkg",
	},
}

func bastionNicFirmwareURL(vendor, currentVersion string) string {
	if artifacts, ok := nicFirmwareArtifacts[vendor]; ok {
		if artifact, ok := artifacts[currentVersion]; ok {
			return bastionFirmwareBaseURL + artifact
		}
		e2e.Failf("Unsupported %s NIC firmware version for bastion firmware: %s", vendor, currentVersion)
		return ""
	}
	e2e.Failf("Unsupported vendor for bastion NIC firmware: %s", vendor)
	return ""
}

func pollForFirmwareVersionChange(oc *exutil.CLI, host, component, initialVersion string) string {
	jsonPath := fmt.Sprintf(`{.status.components[?(@.component=="%s")].currentVersion}`, component)
	var currentVersion string
	pollErr := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 30*time.Minute, true, func(ctx context.Context) (bool, error) {
		ver, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "-o=jsonpath="+jsonPath).Output()
		if err != nil {
			e2e.Logf("Transient error getting %s version: %v", component, err)
			return false, nil
		}
		if ver != "" && ver != initialVersion {
			currentVersion = ver
			return true, nil
		}
		e2e.Logf("%s version: %s, waiting for change from %s...", component, ver, initialVersion)
		return false, nil
	})
	o.Expect(pollErr).NotTo(o.HaveOccurred(), "%s firmware version did not change from %s", component, initialVersion)
	o.Expect(currentVersion).NotTo(o.BeEmpty(), "%s firmware version must not be empty after update", component)
	o.Expect(currentVersion).ShouldNot(o.Equal(initialVersion), "%s firmware version should have changed from %s", component, initialVersion)
	return currentVersion
}

func getBastionNicComponent(oc *exutil.CLI, host, vendor string) (string, string) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("HostFirmwareComponents", "-n", machineAPINamespace, host, "-o=jsonpath={.status.components}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(output).NotTo(o.BeEmpty(), "HFC status components must not be empty")

	var components []struct {
		Component      string `json:"component"`
		CurrentVersion string `json:"currentVersion"`
	}
	err = json.Unmarshal([]byte(output), &components)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to parse HFC status components")

	artifacts := nicFirmwareArtifacts[vendor]
	for _, c := range components {
		if strings.HasPrefix(c.Component, "nic:") && artifacts[c.CurrentVersion] != "" {
			e2e.Logf("Found supported NIC component: %s at version %s", c.Component, c.CurrentVersion)
			return c.Component, c.CurrentVersion
		}
	}
	e2e.Failf("No NIC component with supported firmware version found on host %s vendor %s", host, vendor)
	return "", ""
}
