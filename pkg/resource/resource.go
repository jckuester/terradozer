// Package resource manages the deletion of Terraform resources.
package resource

import (
	"github.com/jckuester/terradozer/pkg/provider"
	"github.com/zclconf/go-cty/cty"
)

// DestroyableResource implementations can destroy a Terraform resource.
type DestroyableResource interface {
	Destroy() error
	Type() string
	ID() string
	State() *cty.Value
	UpdateState() error
}

// Resource represents a Terraform resource that can be destroyed.
type Resource struct {
	// terraformType is the Terraform type of a resource
	terraformType string
	// id is used by the provider to import and delete the resource
	id string
	// provider is able to delete a resource
	provider *provider.TerraformProvider
	// internal Terraform state of the resource
	state *cty.Value
}

// New creates a destroyable Terraform resource.
//
// To destroy a resource, its Terraform Type and ID
// (which both together uniquely identify a resource), plus a provider that
// will handle the destroy is needed.
func New(terraformType, id string, provider *provider.TerraformProvider) *Resource {
	return &Resource{
		terraformType: terraformType,
		id:            id,
		provider:      provider,
	}
}

// NewWithState creates a destroyable Terraform resource which
// contains the internal state representation of the resource.
func NewWithState(terraformType, id string, provider *provider.TerraformProvider, state *cty.Value) *Resource {
	return &Resource{
		terraformType: terraformType,
		id:            id,
		provider:      provider,
		state:         state,
	}
}

// Type returns the Terraform type of a resource.
func (r Resource) Type() string {
	return r.terraformType
}

// ID returns the Terraform ID of a resource.
func (r Resource) ID() string {
	return r.id
}

// State returns the internal Terraform state representation of a resource.
func (r Resource) State() *cty.Value {
	return r.state
}
