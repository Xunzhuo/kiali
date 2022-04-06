package kubernetes

import (
	"fmt"
	"io/ioutil"

	"github.com/kiali/kiali/config"
	kialiConfig "github.com/kiali/kiali/config"
	"k8s.io/client-go/tools/clientcmd"
)

// Be careful with how you use this token. This is the Kiali Service Account token, not the user token.
// We need the Service Account token to access third-party in-cluster services (e.g. Grafana).

const DefaultServiceAccountPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

var KialiToken string

func GetKialiToken() (string, error) {
	enableCustomSecret := config.Get().KubernetesConfig.EnableCustomSecret
	if KialiToken == "" {
		if remoteSecret, err := GetRemoteSecret(RemoteSecretData); err == nil {
			KialiToken = remoteSecret.Users[0].User.Token
		} else {
			var token []byte
			if enableCustomSecret == "true" {
				incluster, err := clientcmd.BuildConfigFromFlags("", kialiConfig.Get().KubernetesConfig.SecretPath)
				if err != nil {
					return "", fmt.Errorf("create RestConfig from custom secret failed: %v", err)
				}
				KialiToken = incluster.BearerToken
				return KialiToken, nil
			} else {
				token, err = ioutil.ReadFile(DefaultServiceAccountPath)
			}
			if err != nil {
				return "", err
			}
			KialiToken = string(token)
		}
	}
	return KialiToken, nil
}
