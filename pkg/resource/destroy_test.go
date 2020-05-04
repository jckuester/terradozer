package resource_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/aws"
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
		parallel              int
	}{
		{
			name:                  "no resources to delete",
			expectedDeletionCount: 0,
			parallel:              1,
		},
		{
			name: "single resource deleted in first run",
			failedDeletions: map[string]int{
				"aws_vpc": 0,
			},
			expectedDeletionCount: 1,
			parallel:              1,
		},
		{
			name: "single resource failed in first run",
			failedDeletions: map[string]int{
				"aws_vpc": 1,
			},
			expectedDeletionCount: 0,
			parallel:              1,
		},
		{
			name: "multiple resources deleted with one retry run",
			failedDeletions: map[string]int{
				"aws_vpc":    1,
				"aws_subnet": 0,
			},
			expectedDeletionCount: 2,
			parallel:              1,
		},
		{
			name: "multiple resources deleted with two retry runs; only one worker",
			failedDeletions: map[string]int{
				"aws_vpc":      2,
				"aws_subnet":   1,
				"aws_instance": 0,
			},
			expectedDeletionCount: 3,
			parallel:              1,
		},
		{
			name: "multiple resources deleted with two retry runs; multiple workers",
			failedDeletions: map[string]int{
				"aws_vpc":      2,
				"aws_subnet":   1,
				"aws_instance": 0,
			},
			expectedDeletionCount: 3,
			parallel:              10,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			var resources []resource.DestroyableResource
			for rType, numOfFailedDeletions := range tc.failedDeletions {
				m := NewMockDestroyableResource(ctrl)

				resFailedDeletions := m.EXPECT().Destroy().
					Return(resource.NewRetryDestroyError(fmt.Errorf("some error"), m)).
					MaxTimes(numOfFailedDeletions)

				m.EXPECT().Destroy().Return(nil).After(resFailedDeletions).AnyTimes()

				m.EXPECT().ID().Return("1234").AnyTimes()
				m.EXPECT().Type().Return(rType).AnyTimes()

				resources = append(resources, m)
			}

			actualDeletionCount := resource.DestroyResources(resources, tc.parallel)
			assert.Equal(t, tc.expectedDeletionCount, actualDeletionCount)

			ctrl.Finish()
		})
	}
}

func TestDestroyResources_DestroyError(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	ctrl := gomock.NewController(t)

	m := NewMockDestroyableResource(ctrl)

	m.EXPECT().Destroy().
		Return(fmt.Errorf("some error")).MaxTimes(1)

	m.EXPECT().ID().Return("1234").AnyTimes()
	m.EXPECT().Type().Return("aws_vpc").AnyTimes()

	actualDeletionCount := resource.DestroyResources([]resource.DestroyableResource{m}, 3)
	assert.Equal(t, actualDeletionCount, 0)
}

func TestResource_Destroy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test.")
	}

	env := test.InitEnv(t)

	terraformDir := "../../test/test-fixtures/single-resource/aws-vpc"

	terraformOptions := test.GetTerraformOptions(terraformDir, env)

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcID, env.AWSRegion)

	awsProvider, err := provider.Init("aws", 10*time.Second)
	require.NoError(t, err)

	r := resource.New("aws_vpc", actualVpcID, awsProvider)

	err = r.UpdateState()
	require.NoError(t, err)

	err = r.Destroy()
	require.NoError(t, err)

	test.AssertVpcDeleted(t, actualVpcID, env)
}

// For this resource, Terraform import function uses the name as an identifier,
// but the id attribute set in the state is the ARN. Therefore, this resource
// cannot be imported by ID und must try to call read directly.
func TestResource_Destroy_AwsEcsCluster(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test.")
	}

	log.SetLevel(log.DebugLevel)

	env := test.InitEnv(t)

	terraformDir := "../../test/test-fixtures/single-resource/aws-ecs-cluster"

	terraformOptions := test.GetTerraformOptions(terraformDir, env)

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualID := terraform.Output(t, terraformOptions, "id")

	test.AssertEcsClusterExists(t, env, actualID)

	awsProvider, err := provider.Init("aws", 10*time.Second)
	require.NoError(t, err)

	r := resource.New("aws_ecs_cluster", actualID, awsProvider)

	err = r.UpdateState()
	require.NoError(t, err)

	err = r.Destroy()
	require.NoError(t, err)

	test.AssertEcsClusterDeleted(t, env, actualID)
}

// For this resource under test, the read function cannot be used without
// an import first to populate all resource attributes.
//
// The reason is that the read function uses the function_name attribute
// and not the ID attribute (although both are equal values).
func TestResource_Destroy_AwsLambdaFunction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test.")
	}

	log.SetLevel(log.DebugLevel)

	env := test.InitEnv(t)

	terraformDir := "../../test/test-fixtures/single-resource/aws-lambda-function"

	terraformOptions := test.GetTerraformOptions(terraformDir, env)

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualID := terraform.Output(t, terraformOptions, "id")
	test.AssertLambdaFunctionExists(t, env, actualID)

	awsProvider, err := provider.Init("aws", 10*time.Second)
	require.NoError(t, err)

	r := resource.New("aws_lambda_function", actualID, awsProvider)

	err = r.UpdateState()
	require.NoError(t, err)

	err = r.Destroy()
	require.NoError(t, err)

	test.AssertLambdaFunctionDeleted(t, env, actualID)
}

func TestResource_Destroy_Timeout(t *testing.T) {
	env := test.InitEnv(t)

	terraformDir := "../../test/test-fixtures/single-resource/aws-vpc"

	terraformOptions := test.GetTerraformOptions(terraformDir, env)

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcID, env.AWSRegion)

	// apply dependency

	terraformDirDependency := "../../test/test-fixtures/single-resource/aws-vpc/dependency"

	terraformOptionsDependency := test.GetTerraformOptions(terraformDirDependency, env,
		map[string]interface{}{"vpc_id": actualVpcID})

	defer terraform.Destroy(t, terraformOptionsDependency)

	terraform.InitAndApply(t, terraformOptionsDependency)

	awsProvider, err := provider.Init("aws", 5*time.Second)
	require.NoError(t, err)

	r := resource.New("aws_vpc", actualVpcID, awsProvider)

	err = r.UpdateState()
	require.NoError(t, err)

	err = r.Destroy()
	assert.EqualError(t, err, "destroy timed out (5s)")
}

func TestResource_Destroy_NilState(t *testing.T) {
	r := resource.New("aws_foo", "id-1234", nil)

	err := r.Destroy()
	assert.EqualError(t, err, "resource state is nil; need to call update first")
}
