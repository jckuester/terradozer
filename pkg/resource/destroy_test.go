package resource_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/apex/log"
	"github.com/golang/mock/gomock"
	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/jckuester/awstools-lib/terraform/provider"
	testUtil "github.com/jckuester/awstools-lib/test"
	"github.com/jckuester/terradozer/pkg/resource"
	"github.com/jckuester/terradozer/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	env := testUtil.Init(t)

	err := testUtil.SetMultiEnvs(map[string]string{
		"AWS_PROFILE": env.AWSProfile1,
		"AWS_REGION":  env.AWSRegion1,
	})
	require.NoError(t, err)

	defer testUtil.UnsetAWSEnvs()

	terraformDir := "../../test/test-fixtures/single-resource/aws-vpc"

	terraformOptions := testUtil.GetTerraformOptions(test.TfStateBucket, terraformDir, env)

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcID, env.AWSRegion1)

	awsProvider, err := provider.Init("aws", ".terradozer", 10*time.Second)
	require.NoError(t, err)

	r := resource.New("aws_vpc", actualVpcID, nil, awsProvider)

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

	env := testUtil.Init(t)

	err := testUtil.SetMultiEnvs(map[string]string{
		"AWS_PROFILE": env.AWSProfile1,
		"AWS_REGION":  env.AWSRegion1,
	})
	require.NoError(t, err)

	defer testUtil.UnsetAWSEnvs()

	terraformDir := "../../test/test-fixtures/single-resource/aws-ecs-cluster"

	terraformOptions := testUtil.GetTerraformOptions(test.TfStateBucket, terraformDir, env)

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualID := terraform.Output(t, terraformOptions, "id")

	test.AssertEcsClusterExists(t, env, actualID)

	awsProvider, err := provider.Init("aws", ".terradozer", 10*time.Second)
	require.NoError(t, err)

	r := resource.New("aws_ecs_cluster", actualID, nil, awsProvider)

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

	env := testUtil.Init(t)

	err := testUtil.SetMultiEnvs(map[string]string{
		"AWS_PROFILE": env.AWSProfile1,
		"AWS_REGION":  env.AWSRegion1,
	})
	require.NoError(t, err)

	defer testUtil.UnsetAWSEnvs()

	terraformDir := "../../test/test-fixtures/single-resource/aws-lambda-function"

	terraformOptions := testUtil.GetTerraformOptions(test.TfStateBucket, terraformDir, env)

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualID := terraform.Output(t, terraformOptions, "id")
	test.AssertLambdaFunctionExists(t, env, actualID)

	awsProvider, err := provider.Init("aws", ".terradozer", 10*time.Second)
	require.NoError(t, err)

	r := resource.New("aws_lambda_function", actualID, nil, awsProvider)

	err = r.UpdateState()
	require.NoError(t, err)

	err = r.Destroy()
	require.NoError(t, err)

	test.AssertLambdaFunctionDeleted(t, env, actualID)
}

func TestResource_Destroy_Timeout(t *testing.T) {
	env := testUtil.Init(t)

	err := testUtil.SetMultiEnvs(map[string]string{
		"AWS_PROFILE": env.AWSProfile1,
		"AWS_REGION":  env.AWSRegion1,
	})
	require.NoError(t, err)

	defer testUtil.UnsetAWSEnvs()

	terraformDir := "../../test/test-fixtures/single-resource/aws-vpc"

	terraformOptions := testUtil.GetTerraformOptions(test.TfStateBucket, terraformDir, env)

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcID, env.AWSRegion1)

	// apply dependency

	terraformDirDependency := "../../test/test-fixtures/single-resource/aws-vpc/dependency"

	terraformOptionsDependency := testUtil.GetTerraformOptions(test.TfStateBucket, terraformDirDependency, env,
		map[string]interface{}{
			"profile": env.AWSProfile1,
			"region":  env.AWSRegion1,
			"name":    fmt.Sprintf("testacc-%s", strings.ToLower(random.UniqueId())),
			"vpc_id":  actualVpcID,
		})

	defer terraform.Destroy(t, terraformOptionsDependency)

	terraform.InitAndApply(t, terraformOptionsDependency)

	awsProvider, err := provider.Init("aws", ".terradozer", 5*time.Second)
	require.NoError(t, err)

	r := resource.New("aws_vpc", actualVpcID, nil, awsProvider)

	err = r.UpdateState()
	require.NoError(t, err)

	err = r.Destroy()
	assert.EqualError(t, err, "destroy timed out (5s)")
}

func TestResource_Destroy_NilState(t *testing.T) {
	r := resource.New("aws_foo", "id-1234", nil, nil)

	err := r.Destroy()
	assert.EqualError(t, err, "resource state is nil; need to call update first")
}
