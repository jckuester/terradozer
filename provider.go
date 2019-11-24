package main

import (
	"github.com/hashicorp/terraform/plugin"
	"github.com/hashicorp/terraform/plugin/discovery"
	"github.com/hashicorp/terraform/providers"
	"github.com/hashicorp/terraform/tfdiags"
	"github.com/zclconf/go-cty/cty"
)

type TerraformProvider struct {
	providers.Interface
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

func (p TerraformProvider) Configure(profile, region string) tfdiags.Diagnostics {
	respConf := p.Interface.Configure(providers.ConfigureRequest{
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

func (p TerraformProvider) ImportResource(resType string, resID string) providers.ImportResourceStateResponse {
	response := p.ImportResourceState(providers.ImportResourceStateRequest{
		TypeName: resType,
		ID:       resID,
	})
	return response
}

func (p TerraformProvider) ReadResource(r providers.ImportedResource) providers.ReadResourceResponse {
	response := p.Interface.ReadResource(providers.ReadResourceRequest{
		TypeName:   r.TypeName,
		PriorState: r.State,
		Private:    r.Private,
	})
	return response
}

func (p TerraformProvider) ApplyResourceChange(resType string,
	readResp providers.ReadResourceResponse) providers.ApplyResourceChangeResponse {

	response := p.Interface.ApplyResourceChange(providers.ApplyResourceChangeRequest{
		TypeName:       resType,
		PriorState:     readResp.NewState,
		PlannedState:   cty.NullVal(cty.DynamicPseudoType),
		Config:         cty.NullVal(cty.DynamicPseudoType),
		PlannedPrivate: readResp.Private,
	})
	return response
}
