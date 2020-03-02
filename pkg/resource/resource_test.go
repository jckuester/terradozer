package resource_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/jckuester/terradozer/pkg/provider"
	"github.com/jckuester/terradozer/test"
	"github.com/stretchr/testify/require"

	"github.com/apex/log"
	"github.com/golang/mock/gomock"
	"github.com/jckuester/terradozer/pkg/resource"
	"github.com/stretchr/testify/assert"
)

func TestDestroyResources(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	tests := []struct {
		name                  string
		expectedDeletionCount int
		failedDeletions       map[string]int
	}{
		{
			name:                  "no resources to delete",
			expectedDeletionCount: 0,
		},
		{
			name: "single resource deleted in first run",
			failedDeletions: map[string]int{
				"aws_vpc": 0,
			},
			expectedDeletionCount: 1,
		},
		{
			name: "single resource failed in first run",
			failedDeletions: map[string]int{
				"aws_vpc": 1,
			},
			expectedDeletionCount: 0,
		},
		{
			name: "multiple resources deleted during two runs",
			failedDeletions: map[string]int{
				"aws_vpc":    1,
				"aws_subnet": 0,
			},
			expectedDeletionCount: 2,
		},
		{
			name: "multiple resources deleted during three runs",
			failedDeletions: map[string]int{
				"aws_vpc":      2,
				"aws_subnet":   1,
				"aws_instance": 0,
			},
			expectedDeletionCount: 3,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			var resources []resource.DestroyableResource
			for rType, numOfFailedDeletions := range tc.failedDeletions {
				m := resource.NewMockDestroyableResource(ctrl)

				resFailedDeletions := m.EXPECT().Destroy(gomock.Any()).
					Return(resource.NewRetryDestroyError(fmt.Errorf("some error"),
						m)).MaxTimes(numOfFailedDeletions)

				m.EXPECT().Destroy(gomock.Any()).
					Return(nil).After(resFailedDeletions).AnyTimes()

				m.EXPECT().ID().Return("1234").AnyTimes()
				m.EXPECT().Type().Return(rType).AnyTimes()

				resources = append(resources, m)
			}

			actualDeletionCount := resource.DestroyResources(resources, false, 3)
			assert.Equal(t, tc.expectedDeletionCount, actualDeletionCount)

			ctrl.Finish()
		})
	}
}

func TestResource_Destroy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test.")
	}

	tests := []struct {
		name                    string
		dryRun                  bool
		expectResourceIsDeleted bool
	}{
		{
			name:                    "with dry-run flag",
			dryRun:                  true,
			expectResourceIsDeleted: false,
		},
		{
			name:                    "without dry-run flag",
			dryRun:                  false,
			expectResourceIsDeleted: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := test.InitEnv(t)

			terraformDir := "../../test/test-fixtures/single-resource"

			terraformOptions := &terraform.Options{
				TerraformDir: terraformDir,
				NoColor:      true,
				Vars: map[string]interface{}{
					"region":  env.AWSRegion,
					"profile": env.AWSProfile,
					"name":    "terradozer-" + strings.ToLower(random.UniqueId()),
				},
			}

			defer terraform.Destroy(t, terraformOptions)

			terraform.InitAndApply(t, terraformOptions)

			actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
			aws.GetVpcById(t, actualVpcID, env.AWSRegion)

			awsProvider, err := provider.Init("aws", 10*time.Second)
			require.NoError(t, err)

			resource := resource.New("aws_vpc", actualVpcID, awsProvider)

			err = resource.Destroy(tc.dryRun)
			require.NoError(t, err)

			if tc.expectResourceIsDeleted {
				test.AssertVpcDeleted(t, actualVpcID, env)
			} else {
				test.AssertVpcExists(t, actualVpcID, env)
			}
		})
	}
}

func TestResource_Destroy_Timeout(t *testing.T) {
	env := test.InitEnv(t)

	terraformDir := "../../test/test-fixtures/single-resource"

	terraformOptions := &terraform.Options{
		TerraformDir: terraformDir,
		NoColor:      true,
		Vars: map[string]interface{}{
			"region":  env.AWSRegion,
			"profile": env.AWSProfile,
			"name":    "terradozer-" + strings.ToLower(random.UniqueId()),
		},
	}

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcID, env.AWSRegion)

	// apply dependency

	terraformDirDependency := "../../test/test-fixtures/single-resource/dependency"

	terraformOptionsDependency := &terraform.Options{
		TerraformDir: terraformDirDependency,
		NoColor:      true,
		Vars: map[string]interface{}{
			"region":  env.AWSRegion,
			"profile": env.AWSProfile,
			"name":    "terradozer-" + strings.ToLower(random.UniqueId()),
			"vpc_id":  actualVpcID,
		},
	}

	defer terraform.Destroy(t, terraformOptionsDependency)

	terraform.InitAndApply(t, terraformOptionsDependency)

	awsProvider, err := provider.Init("aws", 5*time.Second)
	require.NoError(t, err)

	resource := resource.New("aws_vpc", actualVpcID, awsProvider)

	err = resource.Destroy(false)
	assert.EqualError(t, err, "destroy timed out (5s)")
}
