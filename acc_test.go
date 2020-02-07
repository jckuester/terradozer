package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/onsi/gomega/gexec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const packagePath = "github.com/jckuester/terradozer"

func TestAcc_ConfirmDeletion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	tests := []struct {
		name                    string
		userInput               string
		expectResourceIsDeleted bool
		expectedLogs            []string
		unexpectedLogs          []string
	}{
		{
			name:                    "confirmed with YES",
			userInput:               "YES\n",
			expectResourceIsDeleted: true,
			expectedLogs: []string{
				"Are you sure you want to delete these resources (cannot be undone)? Only YES will be accepted.",
				"Starting to delete resources",
				"resource deleted",
				"TOTAL NUMBER OF RESOURCES THAT WOULD BE DELETED: 1",
				"TOTAL NUMBER OF DELETED RESOURCES: 1",
			},
		},
		{
			name:      "confirmed with yes",
			userInput: "yes\n",
			expectedLogs: []string{
				"TOTAL NUMBER OF RESOURCES THAT WOULD BE DELETED: 1",
				"Are you sure you want to delete these resources (cannot be undone)? Only YES will be accepted.",
			},
			unexpectedLogs: []string{
				"Starting to delete resources",
				"TOTAL NUMBER OF DELETED RESOURCES:",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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

			actualLogs := runBinary(t, terraformDir, tc.userInput)

			if tc.expectResourceIsDeleted {
				assertVpcDeleted(t, actualVpcID, env)
			} else {
				assertVpcExists(t, actualVpcID, env)
			}

			for _, expectedLogEntry := range tc.expectedLogs {
				assert.Contains(t, actualLogs.String(), expectedLogEntry)
			}

			for _, unexpectedLogEntry := range tc.unexpectedLogs {
				assert.NotContains(t, actualLogs.String(), unexpectedLogEntry)
			}
		})
	}
}

func TestAcc_AllResourcesAlreadyDeleted(t *testing.T) {
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

	actualLogs := runBinary(t, terraformDir, "YES\n")

	assertVpcDeleted(t, actualVpcID, env)

	// run a second time
	actualLogs = runBinary(t, terraformDir, "")

	assert.Contains(t, actualLogs.String(), "ALL RESOURCES HAVE ALREADY BEEN DELETED")
	assert.NotContains(t, actualLogs.String(), "TOTAL NUMBER OF DELETED RESOURCES: ")
}

func TestAcc_DryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	tests := []struct {
		name                    string
		flag                    string
		expectedLogs            []string
		unexpectedLogs          []string
		expectResourceIsDeleted bool
	}{
		{
			name: "with dry-run flag",
			flag: "-dry",
			expectedLogs: []string{
				"TOTAL NUMBER OF RESOURCES THAT WOULD BE DELETED: 1",
			},
			unexpectedLogs: []string{
				"Starting to delete resources",
				"TOTAL NUMBER OF DELETED RESOURCES:",
			},
		},
		{
			name: "without dry-run flag",
			expectedLogs: []string{
				"Starting to delete resources",
				"resource deleted",
				"TOTAL NUMBER OF DELETED RESOURCES: 1",
			},
			expectResourceIsDeleted: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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

			actualLogs := runBinary(t, terraformDir, "YES\n")

			if tc.expectResourceIsDeleted {
				assertVpcDeleted(t, actualVpcID, env)
			} else {
				assertVpcExists(t, actualVpcID, env)
			}

			for _, expectedLogEntry := range tc.expectedLogs {
				assert.Contains(t, actualLogs.String(), expectedLogEntry)
			}

			for _, unexpectedLogEntry := range tc.unexpectedLogs {
				assert.NotContains(t, actualLogs.String(), unexpectedLogEntry)
			}
		})
	}
}

func TestAcc_Force(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	tests := []struct {
		name                    string
		flags                   []string
		expectedLogs            []string
		unexpectedLogs          []string
		expectResourceIsDeleted bool
	}{
		{
			name:  "with force flag",
			flags: []string{"-force"},
			expectedLogs: []string{
				"Starting to delete resources",
				"resource deleted",
				"TOTAL NUMBER OF DELETED RESOURCES: 1",
			},
			expectResourceIsDeleted: true,
		},
		{
			name: "without force flag",
			unexpectedLogs: []string{
				"Starting to delete resources",
				"resource deleted",
				"TOTAL NUMBER OF DELETED RESOURCES:",
			},
		},
		{
			name:  "with force and dry-run flag",
			flags: []string{"-force", "-dry"},
			unexpectedLogs: []string{
				"Starting to delete resources",
				"resource deleted",
				"TOTAL NUMBER OF DELETED RESOURCES:",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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

			actualLogs := runBinary(t, terraformDir, "", tc.flags...)

			if tc.expectResourceIsDeleted {
				assertVpcDeleted(t, actualVpcID, env)
			} else {
				assertVpcExists(t, actualVpcID, env)
			}

			for _, expectedLogEntry := range tc.expectedLogs {
				assert.Contains(t, actualLogs.String(), expectedLogEntry)
			}

			for _, unexpectedLogEntry := range tc.unexpectedLogs {
				assert.NotContains(t, actualLogs.String(), unexpectedLogEntry)
			}
		})
	}
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

	runBinary(t, terraformDir, "YES\n")

	assertVpcDeleted(t, actualVpcID, env)
	assertIamRoleDeleted(t, actualIamRole, env)
	assertIamPolicyDeleted(t, actualIamPolicyARN, env)
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

	runBinary(t, terraformDir, "YES\n")

	assertVpcDeleted(t, actualVpcID, env)
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

	runBinary(t, terraformDir, "YES\n")
	time.Sleep(5 * time.Second)

	assertBucketDeleted(t, actualBucketName, env)
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

	runBinary(t, terraformDir, "YES\n")

	assertIamRoleDeleted(t, actualIamRole, env)
}

func runBinary(t *testing.T, terraformDir, userInput string, flags ...string) *bytes.Buffer {
	defer gexec.CleanupBuildArtifacts()

	compiledPath, err := gexec.Build(packagePath)
	require.NoError(t, err)

	args := []string{"-state", terraformDir + "/terraform.tfstate"}
	for _, f := range flags {
		args = append(args, f)
	}

	logBuffer := &bytes.Buffer{}

	p := exec.Command(compiledPath, args...)
	p.Stdin = strings.NewReader(userInput)
	p.Stdout = logBuffer
	p.Stderr = logBuffer

	err = p.Run()
	require.NoError(t, err)

	return logBuffer
}
