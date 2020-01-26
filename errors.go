package main

const RetryableError = Error("resource could not be deleted yet because it might still serve as a dependency")
const NotExistingError = Error("resource found in state does not exist anymore")

type Error string

func (err Error) Error() string {
	return string(err)
}
