package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/hashicorp/go-hclog"
	goPlugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform/addrs"
	"github.com/hashicorp/terraform/plugin"
	"github.com/hashicorp/terraform/plugin/discovery"
	"github.com/hashicorp/terraform/providers"
	"github.com/hashicorp/terraform/tfdiags"
	"github.com/mitchellh/cli"
	"github.com/sirupsen/logrus"
	"github.com/zclconf/go-cty/cty"
)

type Provider interface {
	Configure(providers.ConfigureRequest) providers.ConfigureResponse
	ReadResource(providers.ReadResourceRequest) providers.ReadResourceResponse
	PlanResourceChange(providers.PlanResourceChangeRequest) providers.PlanResourceChangeResponse
	ApplyResourceChange(providers.ApplyResourceChangeRequest) providers.ApplyResourceChangeResponse
	ImportResourceState(providers.ImportResourceStateRequest) providers.ImportResourceStateResponse
	ReadDataSource(providers.ReadDataSourceRequest) providers.ReadDataSourceResponse
}

type TerraformProvider struct {
	provider Provider
}

func newTerraformProvider(path string, logDebug bool) (*TerraformProvider, error) {
	m := discovery.PluginMeta{
		Path: path,
	}

	hcLoglevel := hclog.Error
	if logDebug {
		hcLoglevel = hclog.Debug
	}

	p, err := providerFactory(m, hcLoglevel)()
	if err != nil {
		return nil, err
	}
	return &TerraformProvider{p}, nil
}

// copied (and modified) from github.com/hashicorp/terraform/command/plugins.go
func providerFactory(meta discovery.PluginMeta, loglevel hclog.Level) providers.Factory {
	return func() (providers.Interface, error) {
		client := goPlugin.NewClient(clientConfig(meta, loglevel))
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

// copied (and modified) from terraform/plugin/client.go
func clientConfig(m discovery.PluginMeta, loglevel hclog.Level) *goPlugin.ClientConfig {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "plugin",
		Level:  loglevel,
		Output: os.Stderr,
	})

	return &goPlugin.ClientConfig{
		Cmd:              exec.Command(m.Path),
		HandshakeConfig:  plugin.Handshake,
		VersionedPlugins: plugin.VersionedPlugins,
		Managed:          true,
		Logger:           logger,
		AllowedProtocols: []goPlugin.Protocol{goPlugin.ProtocolGRPC},
		AutoMTLS:         true,
	}
}

func (p TerraformProvider) Configure(config cty.Value) tfdiags.Diagnostics {
	respConf := p.provider.Configure(providers.ConfigureRequest{
		Config: config,
	})

	return respConf.Diagnostics
}

func (p TerraformProvider) ImportResource(resType string, resID string) providers.ImportResourceStateResponse {
	response := p.provider.ImportResourceState(providers.ImportResourceStateRequest{
		TypeName: resType,
		ID:       resID,
	})
	return response
}

func (p TerraformProvider) ReadResource(r providers.ImportedResource) providers.ReadResourceResponse {
	response := p.provider.ReadResource(providers.ReadResourceRequest{
		TypeName:   r.TypeName,
		PriorState: r.State,
		Private:    r.Private,
	})
	return response
}

func (p TerraformProvider) DeleteResource(resType string, resID string,
	readResp providers.ReadResourceResponse, dryRun bool) bool {

	if dryRun {
		logrus.Printf("would try to delete resource (type=%s, id=%s)\n", resType, resID)
		return true
	}

	respApply := p.applyResourceChange(resType, readResp)
	if respApply.Diagnostics.HasErrors() {
		logrus.WithError(respApply.Diagnostics.Err()).Infof(
			"failed to delete resource (type=%s, id=%s); skipping resource", resType, resID)
		return false
	}
	logrus.Debugf("new resource state after apply: %s", respApply.NewState.GoString())

	logrus.Printf("deleted resource (type=%s, id=%s)\n", resType, resID)

	return true
}

func (p TerraformProvider) applyResourceChange(resType string,
	readResp providers.ReadResourceResponse) providers.ApplyResourceChangeResponse {

	response := p.provider.ApplyResourceChange(providers.ApplyResourceChangeRequest{
		TypeName:       resType,
		PriorState:     enableForceDestroyAttributes(readResp.NewState),
		PlannedState:   cty.NullVal(cty.DynamicPseudoType),
		Config:         cty.NullVal(cty.DynamicPseudoType),
		PlannedPrivate: readResp.Private,
	})
	return response
}

// enableForceDestroyAttributes sets force destroy attributes to true to be able to successfully delete some resources
func enableForceDestroyAttributes(state cty.Value) cty.Value {
	stateWithDestroyAttrs := map[string]cty.Value{}

	if state.CanIterateElements() {
		for k, v := range state.AsValueMap() {
			if k == "force_detach_policies" || k == "force_destroy" {
				if v.Type().Equals(cty.Bool) {
					stateWithDestroyAttrs[k] = cty.True
				}
			} else {
				stateWithDestroyAttrs[k] = v
			}
		}
	}

	return cty.ObjectVal(stateWithDestroyAttrs)
}

func InstallProvider(providerName, constraint string, useCache bool) (discovery.PluginMeta, error) {
	installDir := ".terradozer"

	providerInstaller := &discovery.ProviderInstaller{
		Dir: installDir,
		Cache: func() discovery.PluginCache {
			if useCache {
				return discovery.NewLocalPluginCache(installDir + "/cache")
			}
			return nil
		}(),
		PluginProtocolVersion: discovery.PluginInstallProtocolVersion,
		SkipVerify:            false,
		Ui: &cli.BasicUi{
			Reader:      os.Stdin,
			Writer:      os.Stdout,
			ErrorWriter: os.Stderr,
		},
	}

	providerConstraint := discovery.AllVersions

	if constraint != "" {
		constraints, err := version.NewConstraint(constraint)
		if err != nil {
			return discovery.PluginMeta{}, fmt.Errorf("failed to parse provider version constraint: %s", err)
		}
		providerConstraint = discovery.NewConstraints(constraints)
	}

	pty := addrs.ProviderType{Name: providerName}
	meta, tfdiagnostics, err := providerInstaller.Get(pty, providerConstraint)
	if err != nil {
		tfdiagnostics = tfdiagnostics.Append(err)
		return discovery.PluginMeta{}, tfdiagnostics.Err()
	}

	return meta, nil
}

// InitProviders installs and initializes, and configures each provider in the given provider address list
func InitProviders(providerAddrs []addrs.AbsProviderConfig) (map[string]*TerraformProvider, error) {
	providers := map[string]*TerraformProvider{}

	for _, pAddr := range providerAddrs {
		pName := pAddr.ProviderConfig.StringCompact()
		logrus.Debugf("provider name: %s", pName)

		pConfig, pVersion, err := ProviderConfig(pName)
		if err != nil {
			logrus.Infof("ignoring resources of provider (name=%s) as it is not (yet) supported", pName)
			continue
		}

		metaPlugin, err := InstallProvider(pName, pVersion, true)
		if err != nil {
			return nil, fmt.Errorf("failed to install provider (%s): %s", pName, err)
		}
		logrus.Infof("installed provider (name=%s, version=%s)", metaPlugin.Name, metaPlugin.Version)

		p, err := newTerraformProvider(metaPlugin.Path, logDebug)
		if err != nil {
			return nil, fmt.Errorf("failed to load Terraform provider (%s): %s", metaPlugin.Path, err)
		}

		tfDiagnostics := p.Configure(pConfig)
		if tfDiagnostics.HasErrors() {
			return nil, fmt.Errorf("failed to configure provider (name=%s, version=%s): %s", metaPlugin.Name, metaPlugin.Version, tfDiagnostics.Err())
		}
		logrus.Infof("configured provider (name=%s, version=%s)", metaPlugin.Name, metaPlugin.Version)

		providers[pName] = p
	}

	return providers, nil
}
