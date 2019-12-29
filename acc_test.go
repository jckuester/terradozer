package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gruntwork-io/terratest/modules/aws"

	"github.com/gruntwork-io/terratest/modules/terraform"
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

func TestAcc_DeleteSingleAwsResource(t *testing.T) {
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

	actualVpcId := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcId, env.AWSRegion)

	os.Args = []string{"cmd", "-state", terraformDir + "/terraform.tfstate"}
	exitCode := mainExitCode()

	assert.Equal(t, 0, exitCode)

	_, err := aws.GetVpcByIdE(t, actualVpcId, env.AWSRegion)
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

	actualVpcId := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcId, env.AWSRegion)

	os.Args = []string{"cmd", "-state", terraformDir + "/terraform.tfstate"}
	exitCode := mainExitCode()

	assert.Equal(t, 0, exitCode)

	_, err := aws.GetVpcByIdE(t, actualVpcId, env.AWSRegion)
	assert.Error(t, err, "resource hasn't been deleted")
}
