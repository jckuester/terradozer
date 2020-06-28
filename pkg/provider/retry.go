package provider

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws/request"
)

//nolint:gochecknoglobals
var (
	// copied from github.com/aws-sdk-go/aws/request/retryer.go
	retryableCodes = map[string]struct{}{
		request.ErrCodeRequestError:    {},
		"RequestTimeout":               {},
		request.ErrCodeResponseTimeout: {},
		"RequestTimeoutException":      {}, // Glacier's flavor of RequestTimeout
	}

	// copied from github.com/aws-sdk-go/aws/request/retryer.go
	throttleCodes = map[string]struct{}{
		"ProvisionedThroughputExceededException": {},
		"ThrottledException":                     {}, // SNS, XRay, ResourceGroupsTagging API
		"Throttling":                             {},
		"ThrottlingException":                    {},
		"RequestLimitExceeded":                   {},
		"RequestThrottled":                       {},
		"RequestThrottledException":              {},
		"TooManyRequestsException":               {}, // Lambda functions
		"PriorRequestNotComplete":                {}, // Route53
		"TransactionInProgressException":         {},
		"EC2ThrottledException":                  {}, // EC2
	}

	// copied from github.com/aws-sdk-go/aws/request/retryer.go
	credsExpiredCodes = map[string]struct{}{
		"ExpiredToken":          {},
		"ExpiredTokenException": {},
		"RequestExpired":        {}, // EC2 Only
	}
)

// shouldRetry returns true if the request should be retried.
// Note: the given error is checked against retryable error codes of the AWS SDK API v1,
// since Terraform AWS Provider also uses v1.
func shouldRetry(err error) bool {
	return isCodeRetryable(err) || isCodeThrottle(err)
}

func isCodeThrottle(err error) bool {
	for throttleCode := range throttleCodes {
		if strings.Contains(err.Error(), throttleCode) {
			return true
		}
	}

	return false
}

func isCodeRetryable(err error) bool {
	for retryableCode := range retryableCodes {
		if strings.Contains(err.Error(), retryableCode) {
			return true
		}
	}

	return isCodeExpiredCreds(err)
}

func isCodeExpiredCreds(err error) bool {
	for credsExpiredCode := range credsExpiredCodes {
		if strings.Contains(err.Error(), credsExpiredCode) {
			return true
		}
	}

	return false
}
