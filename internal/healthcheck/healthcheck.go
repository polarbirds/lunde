package healthcheck

import (
	"net/http"
	"os"

	"github.com/haraldfw/cfger"
	log "github.com/sirupsen/logrus"
)

type healthConfig struct {
	DisableHealth bool `yaml:"disableHealth"`
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("OK"))
}

// StartHandlerIfEnabled starts a health-check handler if it is not disabled
// it is disabled if DISABLE_HEALTH is set to true, or
// CONFIG contains 'disableHealth: true'
func StartHandlerIfEnabled() {
	if isHealthDisabled() {
		return
	}

	http.HandleFunc("/health", health)
	log.Info("Starting health check on :8090/health")
	if err := http.ListenAndServe(":8090", nil); err != nil {
		panic(err)
	}
}

// isHealthDisabled returns true if env var DISABLE_HEALTH is "true" or if the config variable
// disableHealth is set to true
func isHealthDisabled() bool {
	if os.Getenv("DISABLE_HEALTH") != "" && os.Getenv("DISABLE_HEALTH") == "true" {
		log.Warn(
			"Healthcheck was disabled because the env var DISABLE_HEALTH was set to \"true\"")
		return true
	}

	if os.Getenv("CONFIG") == "" {
		log.Info("CONFIG not set, either, healthcheck enabled.")
		return false
	}

	var healthCfg healthConfig
	_, err := cfger.ReadStructuredCfgRecursive("env::CONFIG", &healthCfg)
	if err != nil {
		log.Fatal(err)
	}

	if healthCfg.DisableHealth {
		log.Warn("Healthcheck was disabled due to config variable disableHealth set to true")
		return true
	}

	log.Info("disableHealth was unset or otherwise not true.")
	return false
}
