// Package provider implements a client to call import, read, and destroy on any Terraform provider Plugin via GRPC.
package provider

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/terraform/helper/resource"

	"github.com/jckuester/terradozer/internal"

	"github.com/apex/log"
	"github.com/hashicorp/go-hclog"
	goPlugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform/addrs"
	"github.com/hashicorp/terraform/plugin"
	"github.com/hashicorp/terraform/plugin/discovery"
	"github.com/hashicorp/terraform/providers"
	"github.com/mitchellh/cli"
	"github.com/zclconf/go-cty/cty"
)

const requestError = "RequestError"

// provider is the interface that every Terraform Provider Plugin implements.
type provider interface {
	Configure(providers.ConfigureRequest) providers.ConfigureResponse
	ReadResource(providers.ReadResourceRequest) providers.ReadResourceResponse
	ApplyResourceChange(providers.ApplyResourceChangeRequest) providers.ApplyResourceChangeResponse
	ImportResourceState(providers.ImportResourceStateRequest) providers.ImportResourceStateResponse
}

type TerraformProvider struct {
	provider
	// timeout is the amount of time to wait for a destroy operation of the provider to finish
	timeout time.Duration
}

// Launch launches a Provider Plugin executable to provide the RPC server for this plugin.
// Timeout is the amount of time to wait for a destroy operation of the provider to finish.
func Launch(pathToPluginExecutable string, timeout time.Duration) (*TerraformProvider, error) {
	m := discovery.PluginMeta{
		Path: pathToPluginExecutable,
	}

	p, err := providerFactory(m, hclog.Error)()
	if err != nil {
		return nil, err
	}

	return &TerraformProvider{p, timeout}, nil
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
		Cmd:              exec.Command(m.Path), //nolint:gosec
		HandshakeConfig:  plugin.Handshake,
		VersionedPlugins: plugin.VersionedPlugins,
		Managed:          true,
		Logger:           logger,
		AllowedProtocols: []goPlugin.Protocol{goPlugin.ProtocolGRPC},
		AutoMTLS:         true,
	}
}

// Configure configures a provider.
func (p TerraformProvider) Configure(config cty.Value) error {
	respConf := p.provider.Configure(providers.ConfigureRequest{
		Config: config,
	})

	return respConf.Diagnostics.Err()
}

// ImportResource imports a Terraform resource by type and ID.
// Terraform Type and ID is the minimal information needed to uniquely identify a resource.
// For example, call:
//   ImportResource("aws_instance", "i-1234567890abcdef0")
// The result is a resource which has only its ID set (all other attributes are empty).
func (p TerraformProvider) ImportResource(terraformType string, resID string) ([]providers.ImportedResource, error) {
	var response providers.ImportResourceStateResponse

	err := resource.Retry(30*time.Second, func() *resource.RetryError {
		response = p.ImportResourceState(providers.ImportResourceStateRequest{
			TypeName: terraformType,
			ID:       resID,
		})

		if response.Diagnostics.HasErrors() {
			if strings.Contains(response.Diagnostics.Err().Error(), requestError) {
				log.WithError(response.Diagnostics.Err()).Debug("retrying to import resource")

				return resource.RetryableError(response.Diagnostics.Err())
			}
		}

		return nil
	})

	if response.Diagnostics.HasErrors() {
		return nil, response.Diagnostics.Err()
	}

	if err != nil {
		return nil, fmt.Errorf("import timed out (%s)", p.timeout)
	}

	return response.ImportedResources, nil
}

// ReadResource refreshes and sets all attributes of an imported resource.
// This function is used to populate all attributes of a resource after import.
func (p TerraformProvider) ReadResource(r providers.ImportedResource) (cty.Value, error) {
	var response providers.ReadResourceResponse

	err := resource.Retry(30*time.Second, func() *resource.RetryError {
		response = p.provider.ReadResource(providers.ReadResourceRequest{
			TypeName:   r.TypeName,
			PriorState: r.State,
			Private:    r.Private,
		})

		if response.Diagnostics.HasErrors() {
			if strings.Contains(response.Diagnostics.Err().Error(), requestError) {
				log.WithError(response.Diagnostics.Err()).Debug("retrying to read current state of resource")

				return resource.RetryableError(response.Diagnostics.Err())
			}
		}

		return nil
	})

	if response.Diagnostics.HasErrors() {
		return cty.NilVal, response.Diagnostics.Err()
	}

	if err != nil {
		return cty.NilVal, fmt.Errorf("read timed out (%s)", p.timeout)
	}

	return response.NewState, nil
}

// DestroyResource destroys a resource.
// This function requires the current state of a resource as input (fetched via ReadResource).
func (p TerraformProvider) DestroyResource(terraformType string, currentState cty.Value) error {
	var response providers.ApplyResourceChangeResponse

	err := resource.Retry(p.timeout, func() *resource.RetryError {
		response = p.ApplyResourceChange(providers.ApplyResourceChangeRequest{
			TypeName:     terraformType,
			PriorState:   enableForceDestroyAttributes(currentState),
			PlannedState: cty.NullVal(cty.DynamicPseudoType),
			Config:       cty.NullVal(cty.DynamicPseudoType),
		})

		if response.Diagnostics.HasErrors() {
			if strings.Contains(response.Diagnostics.Err().Error(), requestError) {
				log.WithError(response.Diagnostics.Err()).Debug("retrying to destroy resource")

				return resource.RetryableError(response.Diagnostics.Err())
			}
		}

		return nil
	})

	if response.Diagnostics.HasErrors() {
		return response.Diagnostics.Err()
	}

	if err != nil {
		return fmt.Errorf("destroy timed out (%s)", p.timeout)
	}

	return nil
}

// enableForceDestroyAttributes sets force destroy attributes of a resource to true
// to be able to successfully delete some resources
// (eg. a non-empty S3 bucket or a AWS IAM role with attached policies).
//
// Note: this function is currently AWS specific.
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

// Install installs a Terraform Provider Plugin binary with a given version.
// For example, call:
//   Install("aws", "2.43.0", true)
func Install(providerName, versionConstraint string, cacheBinary bool) (discovery.PluginMeta, error) {
	installDir := ".terradozer"

	providerInstaller := &discovery.ProviderInstaller{
		Dir: installDir,
		Cache: func() discovery.PluginCache {
			if cacheBinary {
				return discovery.NewLocalPluginCache(filepath.FromSlash(installDir + "/cache"))
			}
			return nil
		}(),
		PluginProtocolVersion: discovery.PluginInstallProtocolVersion,
		SkipVerify:            false,
		Ui: &cli.BasicUi{
			Reader:      os.Stdin,
			Writer:      &bytes.Buffer{},
			ErrorWriter: os.Stderr,
		},
	}

	providerConstraint := discovery.AllVersions

	if versionConstraint != "" {
		constraints, err := version.NewConstraint(versionConstraint)
		if err != nil {
			return discovery.PluginMeta{}, fmt.Errorf("failed to parse provider version constraint: %s", err)
		}

		providerConstraint = discovery.NewConstraints(constraints)
	}

	pty := addrs.NewLegacyProvider(providerName)

	meta, tfDiagnostics, err := providerInstaller.Get(pty, providerConstraint)
	if err != nil {
		tfDiagnostics = tfDiagnostics.Append(err)
		return discovery.PluginMeta{}, tfDiagnostics.Err()
	}

	return meta, nil
}

// Init installs, launches (i.e., starts the plugin binary process), and configures
// a given Terraform Provider by name with a default configuration.
//
// Note: Init() combines calls to the functions Install(), Launch(), and Configure().
// Timeout is the amount of time to wait for a destroy operation of the provider to finish.
func Init(providerName string, timeout time.Duration) (*TerraformProvider, error) {
	pConfig, pVersion, err := config(providerName)
	if err != nil {
		log.WithField("name", providerName).Info(internal.Pad("ignoring resources of (yet) unsupported provider"))
		return nil, nil
	}

	metaPlugin, err := Install(providerName, pVersion, true)
	if err != nil {
		return nil, fmt.Errorf("failed to install provider (%s): %s", providerName, err)
	}

	log.WithFields(log.Fields{
		"name":    metaPlugin.Name,
		"version": metaPlugin.Version,
	}).Info(internal.Pad("downloaded and installed provider"))

	p, err := Launch(metaPlugin.Path, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to launch provider (%s): %s", metaPlugin.Path, err)
	}

	err = p.Configure(pConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to configure provider (name=%s, version=%s): %s",
			metaPlugin.Name, metaPlugin.Version, err)
	}

	log.WithFields(log.Fields{
		"name":    metaPlugin.Name,
		"version": metaPlugin.Version,
	}).Info(internal.Pad("configured provider"))

	return p, nil
}

// InitProviders installs, launches (i.e., starts the plugin binary process), and configures
// a given list of Terraform Providers by name with a default configuration.
func InitProviders(providerNames []string, timeout time.Duration) (map[string]*TerraformProvider, error) {
	providers := map[string]*TerraformProvider{}

	for _, pName := range providerNames {
		p, err := Init(pName, timeout)
		if err != nil {
			return nil, err
		}

		if p != nil {
			providers[pName] = p
		}
	}

	return providers, nil
}
