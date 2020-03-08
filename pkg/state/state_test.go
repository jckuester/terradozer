package state_test

import (
	"testing"

	"github.com/jckuester/terradozer/pkg/state"

	"github.com/jckuester/terradozer/pkg/resource"

	"github.com/jckuester/terradozer/pkg/provider"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestNewState(t *testing.T) {
	tests := []struct {
		name           string
		pathToState    string
		expectedErrMsg string
	}{
		{
			name:        "state version 3",
			pathToState: "../../test/test-fixtures/tfstates/version3.tfstate",
		},
		{
			name:        "state version 4",
			pathToState: "../../test/test-fixtures/tfstates/version4.tfstate",
		},
		{
			name:           "broken state file with malformed JSON",
			pathToState:    "../../test/test-fixtures/tfstates/malformed.tfstate",
			expectedErrMsg: "failed reading ../../test/test-fixtures/tfstates/malformed.tfstate as a statefile",
		},
		{
			name:           "wrong path to state",
			pathToState:    "not/exist/terraform.tfstate",
			expectedErrMsg: "open not/exist/terraform.tfstate: no such file or directory",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actualState, err := state.New(tc.pathToState)

			if tc.expectedErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, actualState)
			}
		})
	}
}

func TestState_ProviderNames(t *testing.T) {
	tests := []struct {
		name                  string
		pathToState           string
		expectedProviderNames []string
	}{
		{
			name:                  "state version 3",
			pathToState:           "../../test/test-fixtures/tfstates/version3.tfstate",
			expectedProviderNames: []string{"aws"},
		},
		{
			name:                  "state version 4",
			pathToState:           "../../test/test-fixtures/tfstates/version4.tfstate",
			expectedProviderNames: []string{"aws"},
		},
		{
			name:        "empty state",
			pathToState: "../../test/test-fixtures/tfstates/empty.tfstate",
		},
		{
			name:                  "multiple providers",
			pathToState:           "../../test/test-fixtures/tfstates/multiple-providers.tfstate",
			expectedProviderNames: []string{"aws", "random"},
		},
		{
			name:                  "duplicate provider",
			pathToState:           "../../test/test-fixtures/tfstates/duplicate-provider.tfstate",
			expectedProviderNames: []string{"aws"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, err := state.New(tc.pathToState)
			require.NoError(t, err)

			actualProviderNames := state.ProviderNames()

			assert.Equal(t, tc.expectedProviderNames, actualProviderNames)
		})
	}
}

func TestState_Resources(t *testing.T) {
	tests := []struct {
		name              string
		pathToState       string
		expectedResources []resource.DestroyableResource
		expectedErrMsg    string
		providers         map[string]*provider.TerraformProvider
	}{
		{
			name:        "empty provider list",
			pathToState: "../../test/test-fixtures/tfstates/version3.tfstate",
		},
		{
			name:        "single AWS resource",
			pathToState: "../../test/test-fixtures/tfstates/version3.tfstate",
			providers: map[string]*provider.TerraformProvider{
				"aws": {},
			},
			expectedResources: []resource.DestroyableResource{
				resource.New("aws_vpc",
					"vpc-003104c0d87e7a9f4",
					&provider.TerraformProvider{}),
			},
		},
		{
			name:        "resources from multiple providers",
			pathToState: "../../test/test-fixtures/tfstates/multiple-providers.tfstate",
			providers: map[string]*provider.TerraformProvider{
				"aws":    {},
				"random": {},
			},
			expectedResources: []resource.DestroyableResource{
				resource.New("aws_vpc",
					"vpc-039b3d3fb4ffcf0ea",
					&provider.TerraformProvider{}),
				resource.New(
					"random_integer",
					"12375",
					&provider.TerraformProvider{}),
			},
		},
		{
			name:        "data source",
			pathToState: "../../test/test-fixtures/tfstates/datasource.tfstate",
			providers:   map[string]*provider.TerraformProvider{"aws": nil},
		},
		{
			name:        "state with missing resource ID",
			pathToState: "../../test/test-fixtures/tfstates/missing-id.tfstate",
			providers:   map[string]*provider.TerraformProvider{"aws": nil},
			expectedResources: []resource.DestroyableResource{
				resource.New("aws_vpc", "", nil),
			},
		},
		{
			name:        "empty state",
			pathToState: "../../test/test-fixtures/tfstates/empty.tfstate",
			providers:   map[string]*provider.TerraformProvider{"aws": nil},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.pathToState != "" {
				state, err := state.New(tc.pathToState)
				require.NoError(t, err)

				actualResources, err := state.Resources(tc.providers)
				require.NoError(t, err)

				if tc.expectedErrMsg != "" {
					assert.EqualError(t, err, tc.expectedErrMsg)
				} else {
					require.NoError(t, err)
					assert.Equal(t, tc.expectedResources, actualResources)
				}
			}
		})
	}
}

func Test_getResourceID(t *testing.T) {
	tests := []struct {
		name           string
		pathToState    string
		expectedID     string
		expectedErrMsg string
	}{
		{
			name:        "state version 3",
			pathToState: "../../test/test-fixtures/tfstates/version3.tfstate",
			expectedID:  "vpc-003104c0d87e7a9f4",
		},
		{
			name:        "state version 4",
			pathToState: "../../test/test-fixtures/tfstates/version4.tfstate",
			expectedID:  "vpc-034efaa028f36357d",
		},
		{
			name:        "state with missing resource ID",
			pathToState: "../../test/test-fixtures/tfstates/missing-id.tfstate",
			expectedID:  "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, err := state.New(tc.pathToState)
			require.NoError(t, err)

			resources, err := state.Resources(map[string]*provider.TerraformProvider{"aws": nil})
			require.Len(t, resources, 1)

			if tc.expectedErrMsg != "" {
				assert.EqualError(t, err, tc.expectedErrMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedID, resources[0].ID())
			}
		})
	}
}
