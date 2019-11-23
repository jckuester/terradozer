package main

import (
	"fmt"
	"io/ioutil"
	"log"
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
	"github.com/zclconf/go-cty/cty"
)

type TerraformProvider struct {
	providers.Interface
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

	tfDiagnostics := p.configure(profile, region)
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

			importedResources, tfDiagnostics := p.importResource(resType, resID)
			if tfDiagnostics.HasErrors() {
				logrus.WithError(tfDiagnostics.Err()).Infof("failed to import resource (type=%s, id=%s); skipping resource", resType, resID)
				continue
			}

			for _, resImp := range importedResources {
				logrus.Debugf("imported resource (type=%s, id=%s): %s", resType, resID, resImp.State.GoString())

				readResp := p.ReadResource(providers.ReadResourceRequest{
					TypeName:   resImp.TypeName,
					PriorState: resImp.State,
					Private:    resImp.Private,
				})
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

				respApply := p.ApplyResourceChange(providers.ApplyResourceChangeRequest{
					TypeName:       resType,
					PriorState:     readResp.NewState,
					PlannedState:   cty.NullVal(cty.DynamicPseudoType),
					Config:         cty.NullVal(cty.DynamicPseudoType),
					PlannedPrivate: readResp.Private,
				})

				if respApply.Diagnostics.HasErrors() {
					logrus.WithError(respApply.Diagnostics.Err()).Infof("failed to delete resource (type=%s, id=%s); skipping resource", resType, resID)
					continue
				}
				logrus.Debugf("new resource state after apply: %s", respApply.NewState.GoString())

				fmt.Printf("finished deleting resource (type=%s, id=%s)\n", resImp.TypeName, resID)
				deletedResourcesCount++
			}
		}
	}

	fmt.Printf("deleted %d resource(s)\n", deletedResourcesCount)

	return 0
}

func NewTerraformProvider(path string) (*TerraformProvider, error) {
	m := discovery.PluginMeta{
		Path: path,
	}

	p, err := providerFactory(m)()
	if err != nil {
		return nil, err
	}
	return &TerraformProvider{p}, nil
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

func (p TerraformProvider) configure(profile, region string) tfdiags.Diagnostics {
	respConf := p.Configure(providers.ConfigureRequest{
		TerraformVersion: "0.12.11",
		Config: cty.ObjectVal(map[string]cty.Value{
			"profile":                     cty.StringVal(profile),
			"region":                      cty.StringVal(region),
			"access_key":                  cty.UnknownVal(cty.DynamicPseudoType),
			"allowed_account_ids":         cty.UnknownVal(cty.DynamicPseudoType),
			"assume_role":                 cty.UnknownVal(cty.DynamicPseudoType),
			"endpoints":                   cty.UnknownVal(cty.DynamicPseudoType),
			"forbidden_account_ids":       cty.UnknownVal(cty.DynamicPseudoType),
			"insecure":                    cty.UnknownVal(cty.DynamicPseudoType),
			"max_retries":                 cty.UnknownVal(cty.DynamicPseudoType),
			"s3_force_path_style":         cty.UnknownVal(cty.DynamicPseudoType),
			"secret_key":                  cty.UnknownVal(cty.DynamicPseudoType),
			"shared_credentials_file":     cty.UnknownVal(cty.DynamicPseudoType),
			"skip_credentials_validation": cty.UnknownVal(cty.DynamicPseudoType),
			"skip_get_ec2_platforms":      cty.UnknownVal(cty.DynamicPseudoType),
			"skip_metadata_api_check":     cty.UnknownVal(cty.DynamicPseudoType),
			"skip_region_validation":      cty.UnknownVal(cty.DynamicPseudoType),
			"skip_requesting_account_id":  cty.UnknownVal(cty.DynamicPseudoType),
			"token":                       cty.UnknownVal(cty.DynamicPseudoType),
		})})

	return respConf.Diagnostics
}

func (p TerraformProvider) importResource(resType string, resID string) ([]providers.ImportedResource, tfdiags.Diagnostics) {
	respImport := p.ImportResourceState(providers.ImportResourceStateRequest{
		TypeName: resType,
		ID:       resID,
	})

	return respImport.ImportedResources, respImport.Diagnostics
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
