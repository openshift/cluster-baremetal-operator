package baremetal

import (
	"context"
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

func waitForBMHState(oc *exutil.CLI, bmhName string, bmhStatus string) {
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 45*time.Minute, true, func(ctx context.Context) (bool, error) {
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
	deviceName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("bmh", "-n", machineAPINamespace, bmhName, "-o=jsonpath={.status.hardware.storage[0].name}").Output()
	o.Expect(err).ShouldNot(o.HaveOccurred())
	return deviceName
}

func buildFirmwareURL(vendor, currentVersion string) (string, string) {
	iDRAC_71030 := "https://dl.dell.com/FOLDER11319105M/1/iDRAC_7.10.30.00_A00.exe"
	ilo5_305 := "https://downloads.hpe.com/pub/softlib2/software1/fwpkg-ilo/p991377599/v247527/ilo5_305.fwpkg"
	ilo5_302 := "https://downloads.hpe.com/pub/softlib2/software1/fwpkg-ilo/p991377599/v243854/ilo5_302.fwpkg"
	ilo6_157 := "https://downloads.hpe.com/pub/softlib2/software1/fwpkg-ilo/p788720876/v243858/ilo6_157.fwpkg"
	ilo6_160 := "https://downloads.hpe.com/pub/softlib2/software1/fwpkg-ilo/p788720876/v247531/ilo6_160.fwpkg"

	switch vendor {
	case "Dell Inc.":
		fileName := "firmimgFIT.d9"
		switch currentVersion {
		case "7.00.00.00":
			return iDRAC_71030, fileName
		default:
			return iDRAC_71030, fileName // Default to latest
		}
	case "HPE":
		switch currentVersion {
		case "iLO 5 v3.02":
			return ilo5_305, "ilo5_305.bin"
		case "iLO 5 v3.05":
			return ilo5_302, "ilo5_302.bin"
		case "iLO 6 v1.57":
			return ilo6_160, "ilo6_160.bin"
		case "iLO 6 v1.60":
			return ilo6_157, "ilo6_157.bin"
		default:
			return ilo6_157, "ilo6_157.bin" // Default to 1.57
		}
	default:
		g.Skip("Unsupported vendor: " + vendor)
		return "", ""
	}
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
		g.Skip("Unsupported vendor")
		return "", "", nil
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
			return bcm_226_250, "bcm5751x-v22.6.250-esxi.zip" // Default to latest
		}
	case "Mellanox Technologies":
		switch currentVersion {
		case "28.40.1000":
			return mlx_28_39_1014, "fw-ConnectX7-rel-28_39_1014.bin"
		case "28.39.1014":
			return mlx_28_40_1000, "fw-ConnectX7-rel-28_40_1000.bin"
		default:
			return mlx_28_40_1000, "fw-ConnectX7-rel-28_40_1000.bin" // Default to latest
		}
	default:
		g.Skip("Unsupported NIC vendor: " + vendor)
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
		g.Skip("Unsupported NIC vendor for name lookup: " + vendor)
		return ""
	}
}

// findBMHByName finds a BareMetalHost by exact name match (e.g., "master-00", "worker-01")
// Returns the BMH name if found, empty string if not found
func findBMHByName(oc *exutil.CLI, bmhName string) string {
	allBMHs, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("baremetalhosts", "-n", machineAPINamespace, "-o=jsonpath={.items[*].metadata.name}").Output()
	if err != nil {
		e2e.Logf("Failed to get BMHs: %v", err)
		return ""
	}

	bmhList := strings.Fields(allBMHs)
	for _, bmh := range bmhList {
		if bmh == bmhName {
			e2e.Logf("Found BMH: %s", bmh)
			return bmh
		}
	}

	e2e.Logf("No BMH found with exact name %q", bmhName)
	return ""
}
