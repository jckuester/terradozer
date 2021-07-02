// Package state provides primitives to list all resources and providers in a Terraform state file.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/apex/log"
	"github.com/hashicorp/terraform/addrs"
	"github.com/hashicorp/terraform/states"
	"github.com/hashicorp/terraform/states/statefile"
	"github.com/jckuester/awstools-lib/terraform/provider"
	"github.com/jckuester/terradozer/internal"
	"github.com/jckuester/terradozer/pkg/resource"
	"github.com/zclconf/go-cty/cty"
)

// State represents a Terraform state.
type State struct {
	state *states.State
}

// New creates a state from a given path to a Terraform state file.
func New(path string) (*State, error) {
	stateFile, err := getStateFromPath(path)
	if err != nil {
		return nil, err
	}

	return &State{stateFile.State}, nil
}

// copied from github.com/hashicorp/terraform/command/show.go
func getStateFromPath(path string) (*statefile.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var stateFile *statefile.File

	stateFile, err = statefile.Read(f)
	if err != nil {
		return nil, fmt.Errorf("failed reading %s as a statefile: %s", path, err)
	}

	return stateFile, nil
}

// ProviderNames returns a list of all provider names (e.g., "aws", "google") in the state.
// The result of provider names is deduplicated.
func (s *State) ProviderNames() []string {
	var providers []string

	log.WithField("addresses", s.state.ProviderAddrs()).Debug(internal.Pad("providers found in state"))

	for _, pAddr := range s.state.ProviderAddrs() {
		providers = append(providers, pAddr.ProviderConfig.StringCompact())
	}

	return removeDuplicates(providers)
}

func removeDuplicates(elements []string) []string {
	encountered := map[string]bool{}

	var result []string

	for i := range elements {
		if encountered[elements[i]] {
			// do not add duplicate
		} else {
			// record this element as an encountered element
			encountered[elements[i]] = true
			result = append(result, elements[i])
		}
	}

	return result
}

// Resources returns a list of resources in the state that are managed by one of the given providers.
//
// Data sources are not returned as these are managed outside the scope of the state and
// therefore shouldn't be destroyed.
func (s *State) Resources(providers map[string]*provider.TerraformProvider) ([]resource.UpdatableResource, error) {
	var resources []resource.UpdatableResource

	for _, resAddr := range lookupAllResourceInstanceAddrs(s.state) {
		log.WithField("absolute_address", resAddr.String()).
			Debug(internal.Pad("looked up resource instance address"))

		resInstance := s.state.ResourceInstance(resAddr)

		resID, err := getResourceID(resInstance)
		if err != nil {
			return nil, fmt.Errorf("failed to get id for resource (addr=%s): %s", resAddr.String(), err)
		}

		if resAddr.ContainingResource().Resource.Mode != addrs.ManagedResourceMode {
			log.WithFields(log.Fields{
				"mode": resAddr.Resource.Resource.Mode,
				"type": resAddr.Resource.Resource.Type,
				"id":   resID}).Debug(internal.Pad("ignoring non-managed resource"))

			continue
		}

		providerName := resAddr.Resource.Resource.DefaultProviderConfig().StringCompact()

		p, ok := providers[providerName]
		if !ok {
			log.WithField("name", providerName).Debug(internal.Pad("Terraform provider not found in providers list"))

			continue
		}

		resObject, err := getResourceState(resInstance, resAddr.Resource.Resource.Type, p)
		if err != nil {
			return nil, fmt.Errorf("failed to decode resource into object (addr=%s): %s", resAddr.String(), err)
		}

		r := resource.NewWithState(resAddr.Resource.Resource.Type, resID, p, &resObject)
		resources = append(resources, r)
	}

	return resources, nil
}

// resourceID represents the ID attribute of a Terraform resource.
type resourceID struct {
	ID string `json:"id"`
}

// getResourceID looks up the resource ID amongst all resource attributes.
func getResourceID(resInstance *states.ResourceInstance) (string, error) {
	var result resourceID

	if !resInstance.HasCurrent() {
		return "", fmt.Errorf("resource instance has no current object")
	}

	if resInstance.Current.AttrsJSON != nil {
		err := json.Unmarshal(resInstance.Current.AttrsJSON, &result)
		if err != nil {
			log.WithField("attributes", resInstance.Current.AttrsJSON).
				Debug(internal.Pad("JSON-encoded attributes of resource instance"))

			return "", fmt.Errorf("failed to unmarshal JSON-encoded resource instance attributes: %s", err)
		}

		return result.ID, nil
	}

	if resInstance.Current.AttrsFlat == nil {
		log.WithField("attributes", resInstance.Current.AttrsFlat).
			Debug(internal.Pad("legacy attributes of resource instance"))

		return "", fmt.Errorf("flat attribute map of resource instance is nil")
	}

	return resInstance.Current.AttrsFlat["id"], nil
}

// getResourceState unmarshals the JSON representation of a resource found in the state file into
// an internal Terraform state object representation.
func getResourceState(resInstance *states.ResourceInstance, rType string,
	provider *provider.TerraformProvider) (cty.Value, error) {
	if !resInstance.HasCurrent() {
		return cty.NilVal, fmt.Errorf("resource instance has no current object")
	}

	resourceSchema, err := provider.GetSchemaForResource(rType)
	if err != nil {
		return cty.NilVal, err
	}

	resInstanceObj, err := resInstance.Current.Decode(resourceSchema.Block.ImpliedType())
	if err != nil {
		return cty.NilVal, err
	}

	return resInstanceObj.Value, nil
}

// copied (and modified) from github.com/hashicorp/terraform/command/state_meta.go
func lookupAllResourceInstanceAddrs(state *states.State) []addrs.AbsResourceInstance {
	var ret []addrs.AbsResourceInstance

	for _, ms := range state.Modules {
		ret = append(ret, collectModuleResourceInstances(ms)...)
	}

	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Less(ret[j])
	})

	return ret
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
