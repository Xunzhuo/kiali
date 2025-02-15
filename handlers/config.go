package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v2"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/log"
	"github.com/kiali/kiali/prometheus"
)

const (
	defaultPrometheusGlobalScrapeInterval       = 15    // seconds
	defaultPrometheusGlobalStorageTSDBRetention = 21600 // seconds
)

type IstioAnnotations struct {
	AmbientAnnotation        string `json:"ambientAnnotation,omitempty"`
	AmbientAnnotationEnabled string `json:"ambientAnnotationEnabled,omitempty"`
	IstioInjectionAnnotation string `json:"istioInjectionAnnotation,omitempty"`
}

type IstioCanaryRevision struct {
	Current string `json:"current,omitempty"`
	Upgrade string `json:"upgrade,omitempty"`
}

// PrometheusConfig holds actual Prometheus configuration that is useful to Kiali.
// All durations are in seconds.
type PrometheusConfig struct {
	GlobalScrapeInterval int64 `json:"globalScrapeInterval,omitempty"`
	StorageTsdbRetention int64 `json:"storageTsdbRetention,omitempty"`
}

type DeploymentConfig struct {
	ViewOnlyMode bool `json:"viewOnlyMode,omitempty"`
}

// PublicConfig is a subset of Kiali configuration that can be exposed to clients to
// help them interact with the system.
type PublicConfig struct {
	AccessibleNamespaces []string                      `json:"accessibleNamespaces,omitempty"`
	AuthStrategy         string                        `json:"authStrategy,omitempty"`
	AmbientEnabled       bool                          `json:"ambientEnabled,omitempty"`
	Clusters             map[string]kubernetes.Cluster `json:"clusters,omitempty"`
	Deployment           DeploymentConfig              `json:"deployment,omitempty"`
	GatewayAPIClasses    []config.GatewayAPIClass      `json:"gatewayAPIClasses,omitempty"`
	GatewayAPIEnabled    bool                          `json:"gatewayAPIEnabled,omitempty"`
	HealthConfig         config.HealthConfig           `json:"healthConfig,omitempty"`
	InstallationTag      string                        `json:"installationTag,omitempty"`
	IstioAnnotations     IstioAnnotations              `json:"istioAnnotations,omitempty"`
	IstioCanaryRevision  IstioCanaryRevision           `json:"istioCanaryRevision,omitempty"`
	IstioConfigMap       string                        `json:"istioConfigMap"`
	IstioIdentityDomain  string                        `json:"istioIdentityDomain,omitempty"`
	IstioLabels          config.IstioLabels            `json:"istioLabels,omitempty"`
	IstioNamespace       string                        `json:"istioNamespace,omitempty"`
	IstioStatusEnabled   bool                          `json:"istioStatusEnabled,omitempty"`
	KialiFeatureFlags    config.KialiFeatureFlags      `json:"kialiFeatureFlags,omitempty"`
	LogLevel             string                        `json:"logLevel,omitempty"`
	Prometheus           PrometheusConfig              `json:"prometheus,omitempty"`
}

// Config is a REST http.HandlerFunc serving up the Kiali configuration made public to clients.
func Config(w http.ResponseWriter, r *http.Request) {
	defer handlePanic(w)

	// Note that determine the Prometheus config at request time because it is not
	// guaranteed to remain the same during the Kiali lifespan.
	promConfig := getPrometheusConfig()
	conf := config.Get()
	publicConfig := PublicConfig{
		AccessibleNamespaces: conf.Deployment.AccessibleNamespaces,
		AuthStrategy:         conf.Auth.Strategy,
		Clusters:             make(map[string]kubernetes.Cluster),
		Deployment: DeploymentConfig{
			ViewOnlyMode: conf.Deployment.ViewOnlyMode,
		},
		InstallationTag: conf.InstallationTag,
		IstioAnnotations: IstioAnnotations{
			AmbientAnnotation:        config.AmbientAnnotation,
			AmbientAnnotationEnabled: config.AmbientAnnotationEnabled,
			IstioInjectionAnnotation: conf.ExternalServices.Istio.IstioInjectionAnnotation,
		},
		HealthConfig:        conf.HealthConfig,
		IstioStatusEnabled:  conf.ExternalServices.Istio.ComponentStatuses.Enabled,
		IstioIdentityDomain: conf.ExternalServices.Istio.IstioIdentityDomain,
		IstioNamespace:      conf.IstioNamespace,
		IstioLabels:         conf.IstioLabels,
		IstioConfigMap:      conf.ExternalServices.Istio.ConfigMapName,
		IstioCanaryRevision: IstioCanaryRevision{
			Current: conf.ExternalServices.Istio.IstioCanaryRevision.Current,
			Upgrade: conf.ExternalServices.Istio.IstioCanaryRevision.Upgrade,
		},
		KialiFeatureFlags: conf.KialiFeatureFlags,
		LogLevel:          log.GetLogLevel(),
		Prometheus: PrometheusConfig{
			GlobalScrapeInterval: promConfig.GlobalScrapeInterval,
			StorageTsdbRetention: promConfig.StorageTsdbRetention,
		},
	}

	// The following code fetches the cluster info. Cluster info is not critical.
	// It's even possible that it cannot be resolved (because of Istio not being with MC turned on).
	// Because of these two reasons, let's simply ignore errors in the following code.
	err := func() error {
		layer, err := getBusiness(r)
		if err != nil {
			return err
		}

		// @TODO hardcoded home cluster
		publicConfig.GatewayAPIEnabled = layer.IstioConfig.IsGatewayAPI(conf.KubernetesConfig.ClusterName)
		publicConfig.AmbientEnabled = layer.IstioConfig.IsAmbientEnabled()
		publicConfig.GatewayAPIClasses = layer.IstioConfig.GatewayAPIClasses()

		// Fetch the list of all clusters in the mesh
		// One usage of this data is to cross-link Kiali instances, when possible.
		clusters, err := layer.Mesh.GetClusters(r)
		if err != nil {
			return fmt.Errorf("failure while listing clusters in the mesh: %w", err)
		}

		for _, cluster := range clusters {
			publicConfig.Clusters[cluster.Name] = cluster
		}

		return nil
	}()
	if err != nil {
		log.Warningf("Unable to resolve cluster info: %s", err)
	}

	RespondWithJSONIndent(w, http.StatusOK, publicConfig)
}

type PrometheusPartialConfig struct {
	Global struct {
		Scrape_interval string
	}
}

func getPrometheusConfig() PrometheusConfig {
	promConfig := PrometheusConfig{
		GlobalScrapeInterval: defaultPrometheusGlobalScrapeInterval,
		StorageTsdbRetention: defaultPrometheusGlobalStorageTSDBRetention,
	}
	// Check if thanosProxy
	thanosConf := config.Get().ExternalServices.Prometheus.ThanosProxy
	if thanosConf.Enabled {
		scrapeInterval, err := model.ParseDuration(thanosConf.ScrapeInterval)
		if checkErr(err, fmt.Sprintf("Invalid scrape interval in ThanosProxy configuration [%s]", scrapeInterval)) {
			promConfig.GlobalScrapeInterval = int64(time.Duration(scrapeInterval).Seconds())
		}
		retention, err := model.ParseDuration(thanosConf.RetentionPeriod)
		if checkErr(err, fmt.Sprintf("Invalid retention period in ThanosProxy configuration [%s]", retention)) {
			promConfig.StorageTsdbRetention = int64(time.Duration(retention).Seconds())
		}
	} else {
		client, err := prometheus.NewClient()
		if !checkErr(err, "") {
			log.Error(err)
			return promConfig
		}
		configResult, err := client.GetConfiguration()
		if checkErr(err, "Failed to fetch Prometheus configuration") {
			var config PrometheusPartialConfig
			if checkErr(yaml.Unmarshal([]byte(configResult.YAML), &config), "Failed to unmarshal Prometheus configuration") {
				scrapeIntervalString := config.Global.Scrape_interval
				scrapeInterval, err := model.ParseDuration(scrapeIntervalString)
				if checkErr(err, fmt.Sprintf("Invalid global scrape interval [%s]", scrapeIntervalString)) {
					promConfig.GlobalScrapeInterval = int64(time.Duration(scrapeInterval).Seconds())
				}
			}
		}

		flags, err := client.GetFlags()
		if checkErr(err, "Failed to fetch Prometheus flags") {
			// Prometheus deprecated the storage.tsdb.retention setting in lieu of storage.tsdb.retention.time.
			// But the old one still takes effect if the new one is not set.
			// See: https://prometheus.io/docs/prometheus/latest/storage/#operational-aspects
			retentionString := ""
			if flag, ok := flags["storage.tsdb.retention.time"]; ok && flag != "0s" {
				retentionString = flag
			} else if flag, ok := flags["storage.tsdb.retention"]; ok {
				retentionString = flag
				log.Debugf("Prometheus is using deprecated retention setting: %v", flag)
			}
			if retentionString != "" {
				retention, err := model.ParseDuration(retentionString)
				if checkErr(err, fmt.Sprintf("Invalid storage.tsdb.retention.time [%s]", retentionString)) {
					if retention == 0 {
						log.Warning("Prometheus storage.tsdb.retention.time configured to 0, ignoring...")
					} else {
						promConfig.StorageTsdbRetention = int64(time.Duration(retention).Seconds())
					}
				}
			} else {
				log.Warning("Cannot determine Prometheus retention time; ignoring...")
			}
		}
	}

	return promConfig
}

type KialiCrippledFeatures struct {
	RequestSize             bool `json:"requestSize"`
	RequestSizeAverage      bool `json:"requestSizeAverage"`
	RequestSizePercentiles  bool `json:"requestSizePercentiles"`
	ResponseSize            bool `json:"responseSize"`
	ResponseSizeAverage     bool `json:"responseSizeAverage"`
	ResponseSizePercentiles bool `json:"responseSizePercentiles"`
	ResponseTime            bool `json:"responseTime"`
	ResponseTimeAverage     bool `json:"responseTimeAverage"`
	ResponseTimePercentiles bool `json:"responseTimePercentiles"`
}

func CrippledFeatures(w http.ResponseWriter, r *http.Request) {
	defer handlePanic(w)

	requiredMetrics := []string{
		"istio_request_bytes_bucket",
		"istio_request_bytes_count",
		"istio_request_bytes_sum",
		"istio_request_duration_milliseconds_bucket",
		"istio_request_duration_milliseconds_count",
		"istio_request_duration_milliseconds_sum",
		"istio_requests_total",
		"istio_response_bytes_bucket",
		"istio_response_bytes_count",
		"istio_response_bytes_sum",
	}

	// assume nothing crippled on error
	crippledFeatures := KialiCrippledFeatures{}

	client, err := prometheus.NewClient()
	if !checkErr(err, "") {
		log.Error(err)
		RespondWithJSONIndent(w, http.StatusOK, crippledFeatures)
	}

	existingMetrics, err := client.GetExistingMetricNames(requiredMetrics)
	if !checkErr(err, "") {
		log.Error(err)
		RespondWithJSONIndent(w, http.StatusOK, crippledFeatures)
	}

	// if we have all of the metrics then nothing is crippled, just return
	// if we have no metrics then we have no requests (note that we check for istio_request_totals), nothing is known to be crippled
	if len(existingMetrics) == len(requiredMetrics) || len(existingMetrics) == 0 {
		RespondWithJSONIndent(w, http.StatusOK, crippledFeatures)
	}

	exists := make(map[string]bool, len(existingMetrics))
	for _, metric := range existingMetrics {
		exists[metric] = true
	}

	crippledFeatures.RequestSize = !exists["istio_request_bytes_sum"]
	crippledFeatures.RequestSizeAverage = crippledFeatures.RequestSize || !exists["istio_request_bytes_count"]
	crippledFeatures.RequestSizePercentiles = crippledFeatures.RequestSizeAverage || !exists["istio_request_bytes_bucket"]

	crippledFeatures.ResponseSize = !exists["istio_response_bytes_sum"]
	crippledFeatures.ResponseSizeAverage = crippledFeatures.ResponseSize || !exists["istio_response_bytes_count"]
	crippledFeatures.ResponseSizePercentiles = crippledFeatures.ResponseSizeAverage || !exists["istio_response_bytes_bucket"]

	crippledFeatures.ResponseTime = !exists["istio_request_duration_milliseconds_sum"]
	crippledFeatures.ResponseTimeAverage = crippledFeatures.ResponseTime || !exists["istio_request_duration_milliseconds_count"]
	crippledFeatures.ResponseTimePercentiles = crippledFeatures.ResponseTimeAverage || !exists["istio_request_duration_milliseconds_bucket"]

	RespondWithJSONIndent(w, http.StatusOK, crippledFeatures)
}

func checkErr(err error, message string) bool {
	if err != nil {
		log.Errorf("%s: %v", message, err)
		return false
	}
	return true
}
