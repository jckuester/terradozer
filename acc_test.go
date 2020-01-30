package main

import (
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// EnvVars contains environment variables set for tests
type EnvVars struct {
	AWSRegion  string
	AWSProfile string
}

// InitEnv sets environment variables for tests
func InitEnv(t *testing.T) EnvVars {
	t.Helper()

	profile := os.Getenv("AWS_PROFILE")
	if profile == "" {
		t.Fatal("env variable AWS_PROFILE needs to be set for tests")
	}

	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		t.Fatal("env variable AWS_DEFAULT_REGION needs to be set for tests")
	}

	return EnvVars{
		AWSProfile: profile,
		AWSRegion:  region,
	}
}

func TestAcc_DeleteResource(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	env := InitEnv(t)

	terraformDir := "./test-fixtures/single-resource"

	terraformOptions := &terraform.Options{
		TerraformDir: terraformDir,
		NoColor:      true,
		Vars: map[string]interface{}{
			"region":  env.AWSRegion,
			"profile": env.AWSProfile,
			"name":    "terradozer",
		},
	}

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcID, env.AWSRegion)

	os.Args = []string{"cmd", "-state", terraformDir + "/terraform.tfstate"}
	exitCode := mainExitCode()

	assert.Equal(t, 0, exitCode)

	_, err := aws.GetVpcByIdE(t, actualVpcID, env.AWSRegion)
	assert.Error(t, err, "resource hasn't been deleted")
}

func TestAcc_SkipUnsupportedProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	env := InitEnv(t)

	terraformDir := "./test-fixtures/unsupported-provider"

	terraformOptions := &terraform.Options{
		TerraformDir: terraformDir,
		NoColor:      true,
		Vars: map[string]interface{}{
			"region":  env.AWSRegion,
			"profile": env.AWSProfile,
			"name":    "terradozer",
		},
	}

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcID, env.AWSRegion)

	os.Args = []string{"cmd", "-state", terraformDir + "/terraform.tfstate"}
	exitCode := mainExitCode()

	assert.Equal(t, 0, exitCode)

	_, err := aws.GetVpcByIdE(t, actualVpcID, env.AWSRegion)
	assert.Error(t, err, "resource hasn't been deleted")
}

func TestAcc_DeleteNonEmptyAwsS3Bucket(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	env := InitEnv(t)

	terraformDir := "./test-fixtures/non-empty-bucket"

	terraformOptions := &terraform.Options{
		TerraformDir: terraformDir,
		NoColor:      true,
		Vars: map[string]interface{}{
			"region":  env.AWSRegion,
			"profile": env.AWSProfile,
			"name":    "terradozer",
		},
	}

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualBucketName := terraform.Output(t, terraformOptions, "bucket_name")
	aws.AssertS3BucketExists(t, env.AWSRegion, actualBucketName)

	os.Args = []string{"cmd", "-state", terraformDir + "/terraform.tfstate"}
	exitCode := mainExitCode()

	assert.Equal(t, 0, exitCode)
	time.Sleep(5 * time.Second)

	err := aws.AssertS3BucketExistsE(t, env.AWSRegion, actualBucketName)
	assert.Error(t, err, "resource hasn't been deleted")
}

func TestAcc_DeleteAwsIamRoleWithAttachedPolicy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	env := InitEnv(t)

	terraformDir := "./test-fixtures/attached-policy"

	terraformOptions := &terraform.Options{
		TerraformDir: terraformDir,
		NoColor:      true,
		Vars: map[string]interface{}{
			"region":  env.AWSRegion,
			"profile": env.AWSProfile,
			"name":    "terradozer",
		},
	}

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualIamRole := terraform.Output(t, terraformOptions, "role_name")
	AssertIamRoleExists(t, env.AWSRegion, actualIamRole)

	os.Args = []string{"cmd", "-state", terraformDir + "/terraform.tfstate"}
	exitCode := mainExitCode()

	assert.Equal(t, 0, exitCode)

	err := AssertIamRoleExistsE(t, env.AWSRegion, actualIamRole)
	assert.Error(t, err, "resource hasn't been deleted")
}

func TestAcc_DeleteDependentResources(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	env := InitEnv(t)

	terraformDir := "./test-fixtures/dependent-resources"

	terraformOptions := &terraform.Options{
		TerraformDir: terraformDir,
		NoColor:      true,
		Vars: map[string]interface{}{
			"region":  env.AWSRegion,
			"profile": env.AWSProfile,
			"name":    "terradozer",
		},
	}

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcID, env.AWSRegion)

	actualIamRole := terraform.Output(t, terraformOptions, "role_name")
	AssertIamRoleExists(t, env.AWSRegion, actualIamRole)

	actualIamPolicyARN := terraform.Output(t, terraformOptions, "policy_arn")
	AssertIamPolicyExists(t, env.AWSRegion, actualIamPolicyARN)

	os.Args = []string{"cmd", "-state", terraformDir + "/terraform.tfstate"}
	exitCode := mainExitCode()

	assert.Equal(t, 0, exitCode)

	_, err := aws.GetVpcByIdE(t, actualVpcID, env.AWSRegion)
	assert.Error(t, err, "resource hasn't been deleted")

	err = AssertIamRoleExistsE(t, env.AWSRegion, actualIamRole)
	assert.Error(t, err, "resource hasn't been deleted")

	err = AssertIamPolicyExistsE(t, env.AWSRegion, actualIamPolicyARN)
	assert.Error(t, err, "resource hasn't been deleted")
}

// AssertIamRoleExists checks if the given IAM role exists in the given region and fail the test if it does not.
func AssertIamRoleExists(t *testing.T, region string, name string) {
	err := AssertIamRoleExistsE(t, region, name)
	require.NoError(t, err)
}

// AssertIamRoleExistsE checks if the given IAM role exists in the given region and return an error if it does not.
func AssertIamRoleExistsE(t *testing.T, region string, name string) error {
	iamClient, err := aws.NewIamClientE(t, region)
	if err != nil {
		return err
	}

	params := &iam.GetRoleInput{
		RoleName: &name,
	}

	_, err = iamClient.GetRole(params)

	return err
}

// AssertIamPolicyExists checks if the given IAM policy exists in the given region and fail the test if it does not.
func AssertIamPolicyExists(t *testing.T, region string, name string) {
	err := AssertIamPolicyExistsE(t, region, name)
	require.NoError(t, err)
}

// AssertIamPolicyExistsE checks if the given IAM role exists in the given region and return an error if it does not.
func AssertIamPolicyExistsE(t *testing.T, region string, arn string) error {
	iamClient, err := aws.NewIamClientE(t, region)
	if err != nil {
		return err
	}

	params := &iam.GetPolicyInput{
		PolicyArn: &arn,
	}

	_, err = iamClient.GetPolicy(params)

	return err
}
