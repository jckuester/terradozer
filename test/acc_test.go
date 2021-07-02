package test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jckuester/awstools-lib/test"

	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/onsi/gomega/gexec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	packagePath  = "github.com/jckuester/terradozer"
	usageMessage = `
Terraform destroy using only the state - no *.tf files needed.

USAGE:
  $ terradozer [flags] <path/to/terraform.tfstate>

FLAGS:
  -debug
    	Enable debug logging
  -dry-run
    	Show what would be destroyed
  -force
    	Destroy without asking for confirmation
  -parallel int
    	Limit the number of concurrent destroy operations (default 10)
  -timeout string
    	Amount of time to wait for a destroy of a resource to finish (default "30s")
  -version
    	Show application version
`
)

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
				"SHOWING RESOURCES THAT WOULD BE DELETED (DRY RUN)",
				"TOTAL NUMBER OF RESOURCES THAT WOULD BE DELETED: 1",
				"Are you sure you want to delete these resources (cannot be undone)? Only YES will be accepted.",
				"STARTING TO DELETE RESOURCES",
				"TOTAL NUMBER OF DELETED RESOURCES: 1",
			},
		},
		{
			name:      "confirmed with yes",
			userInput: "yes\n",
			expectedLogs: []string{
				"SHOWING RESOURCES THAT WOULD BE DELETED (DRY RUN)",
				"TOTAL NUMBER OF RESOURCES THAT WOULD BE DELETED: 1",
				"Are you sure you want to delete these resources (cannot be undone)? Only YES will be accepted.",
			},
			unexpectedLogs: []string{
				"STARTING TO DELETE RESOURCES",
				"TOTAL NUMBER OF DELETED RESOURCES:",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := test.Init(t)

			terraformDir := "./test-fixtures/single-resource/aws-vpc"

			terraformOptions := test.GetTerraformOptions(testTfStateBucket, terraformDir, env)

			defer terraform.Destroy(t, terraformOptions)

			terraform.InitAndApply(t, terraformOptions)

			actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
			aws.GetVpcById(t, actualVpcID, env.AWSRegion1)

			tfstateFile, err := WriteRemoteStateToLocalFile(t, env, terraformOptions)
			defer os.Remove(tfstateFile)

			logBuffer, err := runBinary(t, tc.userInput, tfstateFile)
			require.NoError(t, err)

			if tc.expectResourceIsDeleted {
				AssertVpcDeleted(t, actualVpcID, env)
			} else {
				AssertVpcExists(t, actualVpcID, env)
			}

			actualLogs := logBuffer.String()

			for _, expectedLogEntry := range tc.expectedLogs {
				assert.Contains(t, actualLogs, expectedLogEntry)
			}

			for _, unexpectedLogEntry := range tc.unexpectedLogs {
				assert.NotContains(t, actualLogs, unexpectedLogEntry)
			}

			fmt.Println(actualLogs)
		})
	}
}

func TestAcc_AllResourcesAlreadyDeleted(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	env := test.Init(t)

	terraformDir := "./test-fixtures/single-resource/aws-vpc"

	terraformOptions := test.GetTerraformOptions(testTfStateBucket, terraformDir, env)

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcID, env.AWSRegion1)

	tfstateFile, err := WriteRemoteStateToLocalFile(t, env, terraformOptions)
	defer os.Remove(tfstateFile)

	_, err = runBinary(t, "YES\n", tfstateFile)
	require.NoError(t, err)

	AssertVpcDeleted(t, actualVpcID, env)

	// run a second time
	logBuffer, err := runBinary(t, "", tfstateFile)
	require.NoError(t, err)

	actualLogs := logBuffer.String()

	assert.Contains(t, actualLogs, "ALL RESOURCES HAVE ALREADY BEEN DELETED")
	assert.NotContains(t, actualLogs, "TOTAL NUMBER OF DELETED RESOURCES: ")

	fmt.Println(actualLogs)
}

func TestAcc_Version(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	logBuffer, err := runBinary(t, "", "-version")
	require.NoError(t, err)

	actualLogs := logBuffer.String()

	assert.Contains(t, actualLogs, fmt.Sprintf(`
version: dev
commit: ?
built at: ?
using: %s`, runtime.Version()))

	fmt.Println(actualLogs)
}

func TestAcc_MissingStatePathArgument(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	logBuffer, err := runBinary(t, "")
	require.Error(t, err)

	actualLogs := logBuffer.String()

	assert.Contains(t, actualLogs, fmt.Sprintf(`Error: path to Terraform state file expected
%s`, usageMessage))

	fmt.Println(actualLogs)
}

func TestAcc_UndefinedFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	logBuffer, err := runBinary(t, "", "-foo")
	require.Error(t, err)

	actualLogs := logBuffer.String()

	assert.Contains(t, actualLogs, fmt.Sprintf(`flag provided but not defined: -foo
%s`, usageMessage))

	fmt.Println(actualLogs)
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
			flag: "-dry-run",
			expectedLogs: []string{
				"SHOWING RESOURCES THAT WOULD BE DELETED (DRY RUN)",
				"TOTAL NUMBER OF RESOURCES THAT WOULD BE DELETED: 1",
			},
			unexpectedLogs: []string{
				"STARTING TO DELETE RESOURCES",
				"TOTAL NUMBER OF DELETED RESOURCES:",
			},
		},
		{
			name: "without dry-run flag",
			expectedLogs: []string{
				"SHOWING RESOURCES THAT WOULD BE DELETED (DRY RUN)",
				"TOTAL NUMBER OF RESOURCES THAT WOULD BE DELETED: 1",
				"STARTING TO DELETE RESOURCES",
				"TOTAL NUMBER OF DELETED RESOURCES: 1",
			},
			expectResourceIsDeleted: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := test.Init(t)

			terraformDir := "./test-fixtures/single-resource/aws-vpc"

			terraformOptions := test.GetTerraformOptions(testTfStateBucket, terraformDir, env)

			defer terraform.Destroy(t, terraformOptions)

			terraform.InitAndApply(t, terraformOptions)

			actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
			aws.GetVpcById(t, actualVpcID, env.AWSRegion1)

			tfstateFile, err := WriteRemoteStateToLocalFile(t, env, terraformOptions)
			defer os.Remove(tfstateFile)

			args := []string{tfstateFile}
			if tc.flag != "" {
				args = []string{tc.flag, tfstateFile}
			}

			logBuffer, err := runBinary(t, "YES\n", args...)

			require.NoError(t, err)

			if tc.expectResourceIsDeleted {
				AssertVpcDeleted(t, actualVpcID, env)
			} else {
				AssertVpcExists(t, actualVpcID, env)
			}

			actualLogs := logBuffer.String()

			for _, expectedLogEntry := range tc.expectedLogs {
				assert.Contains(t, actualLogs, expectedLogEntry)
			}

			for _, unexpectedLogEntry := range tc.unexpectedLogs {
				assert.NotContains(t, actualLogs, unexpectedLogEntry)
			}

			fmt.Println(actualLogs)
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
		expectedErrCode         int
	}{
		{
			name:  "with force flag",
			flags: []string{"-force"},
			expectedLogs: []string{
				"STARTING TO DELETE RESOURCES",
				"TOTAL NUMBER OF DELETED RESOURCES: 1",
			},
			unexpectedLogs: []string{
				"SHOWING RESOURCES THAT WOULD BE DELETED (DRY RUN)",
				"TOTAL NUMBER OF RESOURCES THAT WOULD BE DELETED: 1",
			},
			expectResourceIsDeleted: true,
		},
		{
			name: "without force flag",
			expectedLogs: []string{
				"SHOWING RESOURCES THAT WOULD BE DELETED (DRY RUN)",
				"TOTAL NUMBER OF RESOURCES THAT WOULD BE DELETED: 1",
			},
			unexpectedLogs: []string{
				"STARTING TO DELETE RESOURCES",
				"TOTAL NUMBER OF DELETED RESOURCES:",
			},
		},
		{
			name:  "with force and dry-run flag",
			flags: []string{"-force", "-dry-run"},
			expectedLogs: []string{
				"-force and -dry-run flag cannot be used together",
			},
			unexpectedLogs: []string{
				"STARTING TO DELETE RESOURCES",
				"TOTAL NUMBER OF DELETED RESOURCES:",
			},
			expectedErrCode: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := test.Init(t)

			terraformDir := "./test-fixtures/single-resource/aws-vpc"

			terraformOptions := test.GetTerraformOptions(testTfStateBucket, terraformDir, env)

			defer terraform.Destroy(t, terraformOptions)

			terraform.InitAndApply(t, terraformOptions)

			actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
			aws.GetVpcById(t, actualVpcID, env.AWSRegion1)

			tfstateFile, err := WriteRemoteStateToLocalFile(t, env, terraformOptions)
			defer os.Remove(tfstateFile)

			args := append(tc.flags, tfstateFile)
			logBuffer, err := runBinary(t, "yes\n", args...)

			if tc.expectedErrCode > 0 {
				require.EqualError(t, err, "exit status 1")
			} else {
				require.NoError(t, err)
			}

			if tc.expectResourceIsDeleted {
				AssertVpcDeleted(t, actualVpcID, env)
			} else {
				AssertVpcExists(t, actualVpcID, env)
			}

			actualLogs := logBuffer.String()

			for _, expectedLogEntry := range tc.expectedLogs {
				assert.Contains(t, actualLogs, expectedLogEntry)
			}

			for _, unexpectedLogEntry := range tc.unexpectedLogs {
				assert.NotContains(t, actualLogs, unexpectedLogEntry)
			}

			fmt.Println(actualLogs)
		})
	}
}

func TestAcc_DeleteDependentResources(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	env := test.Init(t)

	terraformDir := "./test-fixtures/dependent-resources"

	terraformOptions := test.GetTerraformOptions(testTfStateBucket, terraformDir, env)

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcID, env.AWSRegion1)

	actualIamRole := terraform.Output(t, terraformOptions, "role_name")
	AssertIamRoleExists(t, env.AWSRegion1, actualIamRole)

	actualIamPolicyARN := terraform.Output(t, terraformOptions, "policy_arn")
	AssertIamPolicyExists(t, env.AWSRegion1, actualIamPolicyARN)

	tfstateFile, err := WriteRemoteStateToLocalFile(t, env, terraformOptions)
	defer os.Remove(tfstateFile)

	_, err = runBinary(t, "YES\n", tfstateFile)
	require.NoError(t, err)

	AssertVpcDeleted(t, actualVpcID, env)
	AssertIamRoleDeleted(t, actualIamRole, env)
	AssertIamPolicyDeleted(t, actualIamPolicyARN, env)
}

func TestAcc_SkipUnsupportedProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	env := test.Init(t)

	terraformDir := "./test-fixtures/unsupported-provider"

	terraformOptions := test.GetTerraformOptions(testTfStateBucket, terraformDir, env)

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcID, env.AWSRegion1)

	tfstateFile, err := WriteRemoteStateToLocalFile(t, env, terraformOptions)
	defer os.Remove(tfstateFile)

	_, err = runBinary(t, "YES\n", tfstateFile)
	require.NoError(t, err)

	AssertVpcDeleted(t, actualVpcID, env)
}

func TestAcc_DeleteTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	env := test.Init(t)

	terraformDir := "./test-fixtures/single-resource/aws-vpc"

	terraformOptions := test.GetTerraformOptions(testTfStateBucket, terraformDir, env)

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualVpcID := terraform.Output(t, terraformOptions, "vpc_id")
	aws.GetVpcById(t, actualVpcID, env.AWSRegion1)

	// apply dependency

	terraformDirDependency := "./test-fixtures/single-resource/aws-vpc/dependency"

	terraformOptionsDependency := test.GetTerraformOptions(testTfStateBucket, terraformDirDependency, env,
		map[string]interface{}{"vpc_id": actualVpcID})

	defer terraform.Destroy(t, terraformOptionsDependency)

	terraform.InitAndApply(t, terraformOptionsDependency)

	tfstateFile, err := WriteRemoteStateToLocalFile(t, env, terraformOptions)
	defer os.Remove(tfstateFile)

	logBuffer, err := runBinary(t, "YES\n", "-timeout", "2s", tfstateFile)
	require.NoError(t, err)

	actualLogs := logBuffer.String()

	assert.Contains(t, actualLogs, "FAILED TO DELETE THE FOLLOWING RESOURCES (RETRIES EXCEEDED): 1")
	assert.Contains(t, actualLogs, "destroy timed out (2s)")

	fmt.Println(actualLogs)
}

func TestAcc_DeleteNonEmptyAwsS3Bucket(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	env := test.Init(t)

	terraformDir := "./test-fixtures/non-empty-bucket"

	terraformOptions := test.GetTerraformOptions(testTfStateBucket, terraformDir, env)

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualBucketName := terraform.Output(t, terraformOptions, "bucket_name")
	aws.AssertS3BucketExists(t, env.AWSRegion1, actualBucketName)

	tfstateFile, err := WriteRemoteStateToLocalFile(t, env, terraformOptions)
	defer os.Remove(tfstateFile)

	_, err = runBinary(t, "YES\n", tfstateFile)
	require.NoError(t, err)

	time.Sleep(5 * time.Second)

	AssertBucketDeleted(t, actualBucketName, env)
}

func TestAcc_DeleteAwsIamRoleWithAttachedPolicy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test.")
	}

	env := test.Init(t)

	terraformDir := "./test-fixtures/attached-policy"

	terraformOptions := test.GetTerraformOptions(testTfStateBucket, terraformDir, env)

	defer terraform.Destroy(t, terraformOptions)

	terraform.InitAndApply(t, terraformOptions)

	actualIamRole := terraform.Output(t, terraformOptions, "role_name")
	AssertIamRoleExists(t, env.AWSRegion1, actualIamRole)

	tfstateFile, err := WriteRemoteStateToLocalFile(t, env, terraformOptions)
	defer os.Remove(tfstateFile)

	_, err = runBinary(t, "YES\n", tfstateFile)
	require.NoError(t, err)

	AssertIamRoleDeleted(t, actualIamRole, env)
}

func runBinary(t *testing.T, userInput string, args ...string) (*bytes.Buffer, error) {
	defer gexec.CleanupBuildArtifacts()

	compiledPath, err := gexec.Build(packagePath)
	require.NoError(t, err)

	logBuffer := &bytes.Buffer{}

	p := exec.Command(compiledPath, args...)
	p.Stdin = strings.NewReader(userInput)
	p.Stdout = logBuffer
	p.Stderr = logBuffer

	err = p.Run()

	return logBuffer, err
}
