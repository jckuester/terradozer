// Package test contains acceptance tests.
package test

import (
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// EnvVars contains environment variables for that must be set for tests.
type EnvVars struct {
	AWSRegion  string
	AWSProfile string
}

// InitEnv sets environment variables for acceptance tests.
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

// AssertIamRoleDeleted checks if an IAM role has been deleted.
func AssertIamRoleDeleted(t *testing.T, actualIamRole string, env EnvVars) {
	err := AssertIamRoleExistsE(t, env.AWSRegion, actualIamRole)
	assert.Error(t, err, "resource hasn't been deleted")
}

// AssertIamPolicyDeleted checks if an IAM policy has been deleted.
func AssertIamPolicyDeleted(t *testing.T, actualIamPolicyARN string, env EnvVars) {
	err := AssertIamPolicyExistsE(t, env.AWSRegion, actualIamPolicyARN)
	assert.Error(t, err, "resource hasn't been deleted")
}

func AssertVpcExists(t *testing.T, actualVpcID string, env EnvVars) {
	_, err := aws.GetVpcByIdE(t, actualVpcID, env.AWSRegion)
	assert.NoError(t, err, "resource has been unexpectedly deleted")
}

// AssertVpcDeleted checks if an VPC has been deleted.
func AssertVpcDeleted(t *testing.T, actualVpcID string, env EnvVars) {
	_, err := aws.GetVpcByIdE(t, actualVpcID, env.AWSRegion)
	assert.Error(t, err, "resource hasn't been deleted")
}

// AssertBucketDeleted checks if an AWS S3 bucket has been deleted.
func AssertBucketDeleted(t *testing.T, actualBucketName string, env EnvVars) {
	err := aws.AssertS3BucketExistsE(t, env.AWSRegion, actualBucketName)
	assert.Error(t, err, "resource hasn't been deleted")
}
