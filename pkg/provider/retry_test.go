package provider

import (
	"fmt"
	"testing"
)

func Test_shouldRetry(t *testing.T) {
	tests := []struct {
		name string
		arg  error
		want bool
	}{
		{
			name: "a 'Throttling' error that is retryable",
			arg:  fmt.Errorf("ThrottlingException: Rate exceeded"),
			want: true,
		},

		{
			name: "a 'RequestExpired' error that is retryable",
			arg:  fmt.Errorf("RequestExpired: request has expired"),
			want: true,
		},
		{
			name: "a 'RequestError' error that is retryable",
			arg:  fmt.Errorf("RequestError: send request failed"),
			want: true,
		},
		{
			name: "some error that is not retryable",
			arg:  fmt.Errorf("SomeError: foo bar"),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRetry(tt.arg); got != tt.want {
				t.Errorf("shouldRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}
