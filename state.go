package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/sirupsen/logrus"

	"github.com/hashicorp/terraform/addrs"
	"github.com/hashicorp/terraform/states"
	"github.com/hashicorp/terraform/states/statefile"
)

type State struct {
	state *states.State
}

func NewState(path string) (*State, error) {
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

// ProviderNames returns a list of all provider names found in the state (e.g., "aws", "google")
func (s *State) ProviderNames() []string {
	var providers []string

	logrus.Debugf("provider addresses found in state: %s", s.state.ProviderAddrs())

	for _, pAddr := range s.state.ProviderAddrs() {
		providers = append(providers, pAddr.ProviderConfig.StringCompact())
	}

	return providers
}

// Resources returns all the resources (not data sources) in a state for the given providers
func (s *State) Resources(providers map[string]*TerraformProvider) ([]DeletableResource, error) {
	var resources []DeletableResource

	for _, resAddr := range lookupAllResourceInstanceAddrs(s.state) {
		logrus.Debugf("absolute address for resource instance (addr=%s)", resAddr.String())

		resInstance := s.state.ResourceInstance(resAddr)
		resID, err := getResourceID(resInstance)
		if err != nil {
			return nil, fmt.Errorf("failed to get id for resource (addr=%s): %s", resAddr.String(), err)
		}

		if resAddr.ContainingResource().Resource.Mode != addrs.ManagedResourceMode {
			logrus.Infof("ignoring data source (type=%s, id=%s)",
				resAddr.Resource.Resource.Type, resID)
			continue
		}

		providerName := resAddr.Resource.Resource.DefaultProviderConfig().StringCompact()

		p, ok := providers[providerName]
		if !ok {
			logrus.Debugf("Terraform provider not found in providers list: %s", providerName)
			continue
		}

		r := Resource{
			TerraformType: resAddr.Resource.Resource.Type,
			Provider:      p,
			id:            resID,
		}

		resources = append(resources, r)
	}

	return resources, nil
}

type ResourceID struct {
	ID string `json:"id"`
}

func getResourceID(resInstance *states.ResourceInstance) (string, error) {
	var result ResourceID

	if !resInstance.HasCurrent() {
		return "", fmt.Errorf("resource instance has no current object")
	}

	if resInstance.Current.AttrsJSON != nil {
		logrus.Tracef("JSON-encoded attributes of resource instance: %s", resInstance.Current.AttrsJSON)

		err := json.Unmarshal(resInstance.Current.AttrsJSON, &result)
		if err != nil {
			return "", fmt.Errorf("failed to unmarshal JSON-encoded resource instance attributes: %s", err)
		}
		return result.ID, nil
	}
	logrus.Tracef("legacy attributes of resource instance: %s", resInstance.Current.AttrsFlat)

	if resInstance.Current.AttrsFlat == nil {
		return "", fmt.Errorf("flat attribute map of resource instance is nil")
	}

	return resInstance.Current.AttrsFlat["id"], nil
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
