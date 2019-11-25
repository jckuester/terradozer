package main

//go:generate mockgen -source=provider.go -destination=provider_mock_test.go -package=main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"

	"github.com/hashicorp/terraform/addrs"
	"github.com/hashicorp/terraform/states"
	"github.com/hashicorp/terraform/states/statefile"
	"github.com/hashicorp/terraform/tfdiags"
	"github.com/sirupsen/logrus"
)

var (
	dryRun bool
)

func init() {
	flag.BoolVar(&dryRun, "dry-run", false, "Don't delete anything, just show what would happen")
	flag.Parse()
}

func main() {
	os.Exit(mainExitCode())
}

func mainExitCode() int {
	profile := "myaccount"
	region := "us-west-2"
	providerPath := "./terraform-provider-aws_v2.33.0_x4"

	// discard TRACE logs of GRPCProvider
	log.SetOutput(ioutil.Discard)

	logrus.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})
	logrus.SetLevel(logrus.DebugLevel)

	p, err := NewTerraformProvider(providerPath)
	if err != nil {
		logrus.WithError(err).Errorf("failed to load Terraform provider: %s", providerPath)
		return 1
	}

	tfDiagnostics := p.Configure(profile, region)
	if tfDiagnostics.HasErrors() {
		logrus.WithError(tfDiagnostics.Err()).Fatal("failed to configure Terraform provider")
	}

	state, err := getState()
	if err != nil {
		logrus.WithError(err).Errorf("failed to read Terraform state from local file")
		return 1
	}

	resInstances, diagnostics := lookupAllResourceInstanceAddrs(state)
	if diagnostics.HasErrors() {
		logrus.WithError(diagnostics.Err()).Errorf("failed to lookup resource instance addresses")
		return 1
	}

	deletedResourcesCount := 0

	for _, resAddr := range resInstances {
		logrus.Debugf("absolute address for resource instance (addr=%s)", resAddr.String())

		if resInstance := state.ResourceInstance(resAddr); resInstance.HasCurrent() {
			resMode := resAddr.Resource.Resource.Mode
			resID := resInstance.Current.AttrsFlat["id"]
			resType := resAddr.Resource.Resource.Type

			logrus.Debugf("resource instance (mode=%s, type=%s, id=%s)", resMode, resType, resID)

			if resMode != addrs.ManagedResourceMode {
				logrus.Debugf("can only delete managed resources defined by a resource block; therefore, ignoring this resource (type=%s, id=%s)", resType, resID)
				continue
			}

			importResp := p.ImportResource(resType, resID)
			if importResp.Diagnostics.HasErrors() {
				logrus.WithError(importResp.Diagnostics.Err()).Infof("failed to import resource (type=%s, id=%s); skipping resource", resType, resID)
				continue
			}

			for _, resImp := range importResp.ImportedResources {
				logrus.Debugf("imported resource (type=%s, id=%s): %s", resType, resID, resImp.State.GoString())

				readResp := p.ReadResource(resImp)
				if readResp.Diagnostics.HasErrors() {
					logrus.WithError(readResp.Diagnostics.Err()).Infof("failed to read resource and refreshing its current state (type=%s, id=%s); skipping resource", resType, resID)
					continue
				}

				logrus.Debugf("read resource (type=%s, id=%s): %s", resType, resID, readResp.NewState.GoString())

				resourceNotExists := readResp.NewState.IsNull()
				if resourceNotExists {
					logrus.Debugf("resource does not exist anymore (type=%s, id=%s); skipping resource", resImp.TypeName, resID)
					continue
				}

				if p.DeleteResource(resType, resID, readResp, dryRun) {
					deletedResourcesCount++
				}
			}
		}
	}

	fmt.Printf("total resources: %d\n", deletedResourcesCount)

	return 0
}

func getState() (*states.State, error) {
	stateFile, err := getStateFromPath("terraform.tfstate")
	if err != nil {
		return nil, err
	}
	return stateFile.State, nil
}

// copied from github.com/hashicorp/terraform/command/show.go
func getStateFromPath(path string) (*statefile.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed loading statefile: %s", err)
	}
	defer f.Close()

	var stateFile *statefile.File
	stateFile, err = statefile.Read(f)
	if err != nil {
		return nil, fmt.Errorf("failed reading %s as a statefile: %s", path, err)
	}
	return stateFile, nil
}

// copied from github.com/hashicorp/terraform/command/state_meta.go
func lookupAllResourceInstanceAddrs(state *states.State) ([]addrs.AbsResourceInstance, tfdiags.Diagnostics) {
	var ret []addrs.AbsResourceInstance
	var diags tfdiags.Diagnostics
	for _, ms := range state.Modules {
		ret = append(ret, collectModuleResourceInstances(ms)...)
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Less(ret[j])
	})
	return ret, diags
}

// copied from github.com/hashicorp/terraform/command/state_meta.go
func collectModuleResourceInstances(ms *states.Module) []addrs.AbsResourceInstance {
	var ret []addrs.AbsResourceInstance
	for _, rs := range ms.Resources {
		ret = append(ret, collectResourceInstances(ms.Addr, rs)...)
	}
	return ret
}

// copied from github.com/hashicorp/terraform/command/state_meta.go
func collectResourceInstances(moduleAddr addrs.ModuleInstance, rs *states.Resource) []addrs.AbsResourceInstance {
	var ret []addrs.AbsResourceInstance
	for key := range rs.Instances {
		ret = append(ret, rs.Addr.Instance(key).Absolute(moduleAddr))
	}
	return ret
}
