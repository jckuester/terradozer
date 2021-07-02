package state_test

import (
	"testing"
	"time"

	"github.com/jckuester/awstools-lib/terraform/provider"
	"github.com/jckuester/awstools-lib/test"
	"github.com/jckuester/terradozer/pkg/resource"
	"github.com/jckuester/terradozer/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
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
	if testing.Short() {
		t.Skip("Skipping integration test.")
	}

	test.Init(t)

	awsProvider, err := provider.Init("aws", ".terradozer", 10*time.Second)
	require.NoError(t, err)

	tests := []struct {
		name              string
		pathToState       string
		expectedResources []resource.UpdatableResource
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
				"aws": awsProvider,
			},
			expectedResources: []resource.UpdatableResource{
				resource.NewWithState("aws_vpc",
					"vpc-003104c0d87e7a9f4",
					awsProvider, nil),
			},
		},
		{
			name:        "data source",
			pathToState: "../../test/test-fixtures/tfstates/datasource.tfstate",
			providers:   map[string]*provider.TerraformProvider{"aws": awsProvider},
		},
		{
			name:        "empty state",
			pathToState: "../../test/test-fixtures/tfstates/empty.tfstate",
			providers:   map[string]*provider.TerraformProvider{"aws": awsProvider},
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
					require.True(t, len(actualResources) == len(tc.expectedResources))

					for _, rExpected := range tc.expectedResources {
						for _, rActual := range actualResources {
							assert.Equal(t, rExpected.Type(), rActual.Type())
							assert.Equal(t, rExpected.ID(), rActual.ID())
							assert.Equal(t, cty.StringVal(rExpected.ID()), rActual.State().GetAttr("id"))
						}
					}
				}
			}
		})
	}
}
