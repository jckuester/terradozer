package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/hashicorp/terraform/addrs"
	"github.com/hashicorp/terraform/plugin"
	"github.com/hashicorp/terraform/plugin/discovery"
	"github.com/hashicorp/terraform/providers"
	"github.com/hashicorp/terraform/states"
	"github.com/hashicorp/terraform/states/statefile"
	"github.com/hashicorp/terraform/tfdiags"
	"github.com/sirupsen/logrus"
)

type TerraformProvider struct {
	providers.Interface
}

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})

	_, err := loadAWSProvider()
	if err != nil {
		logrus.WithError(err).Fatal("failed to load AWS resource provider")
	}

	state, err := getState()
	if err != nil {
		logrus.WithError(err).Fatal("failed to read tfstate from local file")
	}

	resInstances, diagnostics := lookupAllResourceInstanceAddrs(state)
	if diagnostics.HasErrors() {
		logrus.WithError(diagnostics.Err()).Fatal("failed to lookup resource instance addresses")
	}

	for _, resAddr := range resInstances {
		if resInstance := state.ResourceInstance(resAddr); resInstance.HasCurrent() {
			resMode := resAddr.Resource.Resource.Mode
			resID := resInstance.Current.AttrsFlat["id"]

			if resMode == addrs.ManagedResourceMode {
				logrus.WithFields(map[string]interface{}{
					"id": resID,
				}).Print(resAddr.String())
			}
		}
	}
}

func loadAWSProvider() (*TerraformProvider, error) {
	awsProviderPluginData := discovery.PluginMeta{
		Name:    "terraform-provider-aws",
		Version: "v2.33.0",
		Path:    "./terraform-provider-aws_v2.33.0_x4",
	}

	awsProvider, err := providerFactory(awsProviderPluginData)()
	if err != nil {
		return nil, err
	}
	return &TerraformProvider{awsProvider}, nil
}

// copied from github.com/hashicorp/terraform/command/plugins.go
func providerFactory(meta discovery.PluginMeta) providers.Factory {
	return func() (providers.Interface, error) {
		client := plugin.Client(meta)
		// Request the RPC client so we can get the provider
		// so we can build the actual RPC-implemented provider.
		rpcClient, err := client.Client()
		if err != nil {
			return nil, err
		}

		raw, err := rpcClient.Dispense(plugin.ProviderPluginName)
		if err != nil {
			return nil, err
		}

		// store the client so that the plugin can kill the child process
		p := raw.(*plugin.GRPCProvider)
		p.PluginClient = client
		return p, nil
	}
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
