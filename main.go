package main

//go:generate mockgen -source=provider.go -destination=provider_mock_test.go -package=main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/sirupsen/logrus"
)

var (
	dryRun      bool
	logDebug    bool
	pathToState string
)

func init() {
	flag.BoolVar(&dryRun, "dry", false, "Don't delete anything")
	flag.BoolVar(&logDebug, "debug", false, "Enable debug logging")
	flag.StringVar(&pathToState, "state", "terraform.tfstate", "Path to a Terraform state file")
}

func main() {
	os.Exit(mainExitCode())
}

func mainExitCode() int {
	flag.Parse()

	// discard TRACE logs of GRPCProvider
	log.SetOutput(ioutil.Discard)

	logrus.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})

	if logDebug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	state, err := NewState(pathToState)
	if err != nil {
		logrus.WithError(err).Error("failed to get Terraform state")
		return 1
	}
	logrus.Infof("using state: %s", pathToState)

	providers, err := InitProviders(state.ProviderNames())
	if err != nil {
		logrus.WithError(err).Error("failed to initialize Terraform providers needed for deletion of resources")
		return 1
	}

	resources, err := state.Resources()
	if err != nil {
		logrus.WithError(err).Error("failed to get resources from state")
		return 1
	}

	numDeletedResources := deleteResources(resources, providers)

	logrus.Infof("total number of resources deleted: %d\n", numDeletedResources)

	return 0
}

func deleteResources(resources []Resource, providers map[string]*TerraformProvider) int {
	deletionCount := 0

	for _, r := range resources {
		p, ok := providers[r.Provider]
		if !ok {
			logrus.Debugf("Terraform provider not found in provider list: %s", r.Provider)
			continue
		}

		deleted := p.DeleteResource(r, dryRun)
		if deleted {
			deletionCount++
		}
	}

	return deletionCount
}
