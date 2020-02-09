package main

// RetryableError is a helper to create a RetryResourceError.
func RetryableError(err error, r DeletableResource) *RetryResourceError {
	if err == nil {
		return nil
	}

	return &RetryResourceError{Err: err, Resource: r}
}

// RetryResourceError is returned when deleting of a resource has failed, most likely due to being
// a dependency for another resource. It may be worth retrying once the dependent resource is gone.
type RetryResourceError struct {
	Err error
	// Resource is the resource which deletion has failed
	Resource DeletableResource
}

func (r RetryResourceError) Error() string {
	return r.Err.Error()
}
