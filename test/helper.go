// Package test contains acceptance tests.
package test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/jckuester/awstools-lib/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTfStateBucket = "terradozer-testacc-tfstate-492043"

func WriteRemoteStateToLocalFile(t *testing.T, env test.Vars, terraformOptions *terraform.Options) (string, error) {
	tfstate := aws.GetS3ObjectContents(t, env.AWSRegion1,
		terraformOptions.BackendConfig["bucket"].(string),
		terraformOptions.BackendConfig["key"].(string))

	localStatePath := fmt.Sprintf("%s/%s", os.TempDir(), terraformOptions.BackendConfig["key"].(string))

	err := ioutil.WriteFile(localStatePath, []byte(tfstate), 0644)
	if err != nil {
		t.Fatal(err)
	}

	return localStatePath, err
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
func AssertIamRoleDeleted(t *testing.T, actualIamRole string, env test.Vars) {
	err := AssertIamRoleExistsE(t, env.AWSRegion1, actualIamRole)
	assert.Error(t, err, "resource hasn't been deleted")
}

// AssertIamPolicyDeleted checks if an IAM policy has been deleted.
func AssertIamPolicyDeleted(t *testing.T, actualIamPolicyARN string, env test.Vars) {
	err := AssertIamPolicyExistsE(t, env.AWSRegion1, actualIamPolicyARN)
	assert.Error(t, err, "resource hasn't been deleted")
}

func AssertVpcExists(t *testing.T, actualVpcID string, env test.Vars) {
	_, err := aws.GetVpcByIdE(t, actualVpcID, env.AWSRegion1)
	assert.NoError(t, err, "resource has been unexpectedly deleted")
}

// AssertVpcDeleted checks if an VPC has been deleted.
func AssertVpcDeleted(t *testing.T, actualVpcID string, env test.Vars) {
	_, err := aws.GetVpcByIdE(t, actualVpcID, env.AWSRegion1)
	assert.Error(t, err, "resource hasn't been deleted")
}

// AssertBucketDeleted checks if an AWS S3 bucket has been deleted.
func AssertBucketDeleted(t *testing.T, actualBucketName string, env test.Vars) {
	err := aws.AssertS3BucketExistsE(t, env.AWSRegion1, actualBucketName)
	assert.Error(t, err, "resource hasn't been deleted")
}

func AssertEcsClusterExists(t *testing.T, env test.Vars, id string) {
	assert.True(t, ecsClusterExists(t, env, id))
}

func AssertEcsClusterDeleted(t *testing.T, env test.Vars, id string) {
	assert.False(t, ecsClusterExists(t, env, id))
}

func ecsClusterExists(t *testing.T, env test.Vars, id string) bool {
	opts := &ecs.DescribeClustersInput{
		Clusters: []*string{&id},
	}

	resp, err := aws.NewEcsClient(t, env.AWSRegion1).DescribeClusters(opts)
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.Clusters) == 0 {
		return false
	}

	if *resp.Clusters[0].Status == "INACTIVE" {
		return false
	}

	return true
}

func AssertLambdaFunctionExists(t *testing.T, env test.Vars, id string) {
	assert.True(t, lambdaFunctionExists(t, env, id))
}

func AssertLambdaFunctionDeleted(t *testing.T, env test.Vars, id string) {
	assert.False(t, lambdaFunctionExists(t, env, id))
}

func lambdaFunctionExists(t *testing.T, env test.Vars, id string) bool {
	opts := &lambda.GetFunctionInput{
		FunctionName: &id,
	}

	_, err := NewLambdaClient(t, env.AWSRegion1).GetFunction(opts)
	if err != nil {
		awsErr, ok := err.(awserr.Error)
		if !ok {
			t.Fatal(err)
		}

		if awsErr.Code() == "ResourceNotFoundException" {
			return false
		}

		t.Fatal(err)
	}

	return true
}

// NewLambdaClient creates a Lambda client.
func NewLambdaClient(t *testing.T, region string) *lambda.Lambda {
	client, err := NewLambdaClientE(region)
	require.NoError(t, err)

	return client
}

// NewLambdaClientE creates a Lambda client.
func NewLambdaClientE(region string) (*lambda.Lambda, error) {
	sess, err := aws.NewAuthenticatedSession(region)
	if err != nil {
		return nil, err
	}

	return lambda.New(sess), nil
}
