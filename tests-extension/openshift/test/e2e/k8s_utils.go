package baremetal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func checkOperatorsRunning(oc *exutil.CLI) (bool, error) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusteroperators.config.openshift.io", "-o", "json").Output()
	if err != nil {
		return false, fmt.Errorf("failed to execute 'oc get clusteroperators.config.openshift.io' command: %v", err)
	}

	var result struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Status struct {
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return false, fmt.Errorf("failed to parse clusteroperators JSON: %v", err)
	}

	for _, co := range result.Items {
		var available, degraded bool
		for _, cond := range co.Status.Conditions {
			if cond.Type == "Available" {
				available = (cond.Status == "True")
			}
			if cond.Type == "Degraded" {
				degraded = (cond.Status == "False")
			}
		}

		e2e.Logf("%s: Available=%v Degraded=%v", co.Metadata.Name, available, !degraded)
		if !available || !degraded {
			return false, nil
		}
	}

	return true, nil
}

func checkNodesRunning(oc *exutil.CLI) (bool, error) {
	jpath := `{range .items[*]}{.metadata.name}:{.status.conditions[?(@.type=='Ready')].status}{'\n'}{end}`
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-o", "jsonpath="+jpath).Output()
	if err != nil {
		return false, fmt.Errorf("failed to execute 'oc get nodes' command: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		e2e.Logf("%s", line)
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			e2e.Logf("Skipping malformed line (missing colon): %s", line)
			continue
		}
		ready := parts[1] == "True"

		if !ready {
			return false, nil
		}
	}

	return true, nil
}

func waitForDeployStatus(oc *exutil.CLI, depName string, nameSpace string, depStatus string) {
	err := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 300*time.Second, true, func(context.Context) (bool, error) {
		status, err := oc.AsAdmin().Run("get").Args("deployment", "-n", nameSpace, depName, "-o=jsonpath={.status.conditions[?(@.type=='Available')].status}").Output()
		if err != nil || string(status) != depStatus {
			e2e.Logf("Deployment status: %s. Trying again", status)
			return false, nil
		}
		e2e.Logf("Deployment status: %s", status)
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("Deployment %s did not reach status %s", depName, depStatus))
}

func waitForDeploymentReady(oc *exutil.CLI, depName string, nameSpace string, expectedReplicas int) {
	err := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 300*time.Second, true, func(context.Context) (bool, error) {
		readyReplicas, err := oc.AsAdmin().Run("get").Args("-n", nameSpace, "deployment", depName, "-o=jsonpath={.status.readyReplicas}").Output()
		if err != nil {
			return false, nil
		}
		if readyReplicas == fmt.Sprintf("%d", expectedReplicas) {
			return true, nil
		}
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("Deployment %s did not reach expected replicas", depName))
}

func getPodName(oc *exutil.CLI, ns string) string {
	podName, err := oc.AsAdmin().Run("get").Args("pod", "-n", ns, "-o=jsonpath={.items[0].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return podName
}

func getPodStatus(oc *exutil.CLI, namespace string, podName string) string {
	podStatus, err := oc.AsAdmin().Run("get").Args("pod", "-n", namespace, podName, "-o=jsonpath={.status.phase}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return podStatus
}

func checkOperator(oc *exutil.CLI, operatorName string) (bool, error) {
	// Check specifically that Available=True (not just any "True" in the output)
	available, err := oc.AsAdmin().Run("get").Args("clusteroperator", operatorName, "-o=jsonpath={.status.conditions[?(@.type=='Available')].status}").Output()
	if err != nil {
		return false, err
	}
	return available == "True", nil
}

func waitForPodNotFound(oc *exutil.CLI, podName string, nameSpace string) {
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 30*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := oc.AsAdmin().Run("get").Args("-n", nameSpace, "pod", podName).Output()
		if err != nil {
			e2e.Logf("pod %v doesn't exist", podName)
			return true, nil
		}
		e2e.Logf("pod %v exist, Trying again", podName)
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, "The test deployment job is running")
}

func getUserFromSecret(oc *exutil.CLI, namespace string, secretName string) string {
	userbase64, pwderr := oc.AsAdmin().Run("get").Args("secrets", "-n", namespace, secretName, "-o=jsonpath={.data.username}").Output()
	o.Expect(pwderr).ShouldNot(o.HaveOccurred())
	user, err := base64.StdEncoding.DecodeString(userbase64)
	o.Expect(err).ShouldNot(o.HaveOccurred())
	return string(user)
}

func getPassFromSecret(oc *exutil.CLI, namespace string, secretName string) string {
	pwdbase64, pwderr := oc.AsAdmin().Run("get").Args("secrets", "-n", namespace, secretName, "-o=jsonpath={.data.password}").Output()
	o.Expect(pwderr).ShouldNot(o.HaveOccurred())
	pwd, err := base64.StdEncoding.DecodeString(pwdbase64)
	o.Expect(err).ShouldNot(o.HaveOccurred())
	return string(pwd)
}

// clusterOperatorHealthcheck check abnormal operators
func clusterOperatorHealthcheck(oc *exutil.CLI, waitTime int, dirname string) error {
	e2e.Logf("Check the abnormal operators")
	errCo := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, time.Duration(waitTime)*time.Second, false, func(cxt context.Context) (bool, error) {
		coLogFile, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "--no-headers").OutputToFile(dirname)
		if err == nil {
			coLogs, err := exec.Command("grep", "-v", ".True.*False.*False", coLogFile).Output()
			// grep returns exit code 1 when no lines match, which is the desired outcome
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
					o.Expect(err).NotTo(o.HaveOccurred())
				}
			}
			if len(coLogs) > 0 {
				return false, nil
			}
		} else {
			return false, nil
		}
		err = oc.AsAdmin().WithoutNamespace().Run("get").Args("co").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("No abnormality found in cluster operators...")
		return true, nil
	})
	if errCo != nil {
		err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	return errCo
}

// clusterNodesHealthcheck check abnormal nodes
func clusterNodesHealthcheck(oc *exutil.CLI, waitTime int) error {
	errNode := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, time.Duration(waitTime)*time.Second, false, func(cxt context.Context) (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node").Output()
		if err == nil {
			if strings.Contains(output, "NotReady") || strings.Contains(output, "SchedulingDisabled") {
				return false, nil
			}
		} else {
			return false, nil
		}
		err = oc.AsAdmin().WithoutNamespace().Run("get").Args("node").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("No abnormality found in cluster nodes...")
		return true, nil
	})
	if errNode != nil {
		err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	return errNode
}

// checkNodeStatus
func checkNodeStatus(oc *exutil.CLI, pollIntervalSec time.Duration, pollDurationMinute time.Duration, nodeName string, nodeStatus string) error {
	e2e.Logf("Check status of node %s", nodeName)
	errNode := wait.PollUntilContextTimeout(context.Background(), pollIntervalSec, pollDurationMinute, false, func(ctx context.Context) (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", nodeName, "-o=jsonpath={.status.conditions[?(@.type==\"Ready\")].status}").Output()
		if err != nil || string(output) != nodeStatus {
			e2e.Logf("Node status: %s. Trying again", output)
			return false, nil
		}
		if string(output) == nodeStatus {
			e2e.Logf("Node status: %s", output)
			return true, nil
		}
		return false, nil
	})
	if errNode != nil {
		err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", nodeName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	return errNode
}
