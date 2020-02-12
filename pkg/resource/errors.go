package resource

// NewRetryDestroyError creates a RetryDestroyError.
func NewRetryDestroyError(err error, r DestroyableResource) *RetryDestroyError {
	if err == nil {
		return nil
	}

	return &RetryDestroyError{Err: err, Resource: r}
}

// RetryDestroyError is returned when destroying of a resource has failed, most likely due to being
// a dependency for another resource. It may be worth retrying once the dependent resource is gone.
type RetryDestroyError struct {
	Err error
	// Resource is the resource for which a destroy has failed.
	Resource DestroyableResource
}

func (r RetryDestroyError) Error() string {
	return r.Err.Error()
}
