package externalclients

import (
	"context"
	"strings"

	"github.com/ghodss/yaml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// install-config access details
	clusterConfigName      = "cluster-config-v1"
	clusterConfigKey       = "install-config"
	clusterConfigNamespace = "kube-system"
)

type InstallConfigData struct {
	SSHKey string
}

func (e *externalResourceClient) ReadSSHKey(ctx context.Context) (string, error) {
	installConfigData := InstallConfigData{}
	clusterConfig, err := e.kubeClient.CoreV1().ConfigMaps(clusterConfigNamespace).Get(ctx, clusterConfigName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	err = yaml.Unmarshal([]byte(clusterConfig.Data[clusterConfigKey]), &installConfigData)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(installConfigData.SSHKey), nil
}
