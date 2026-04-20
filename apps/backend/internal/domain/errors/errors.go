package errors

// ErrorString returns err.Error() or "" if err is nil.
func ErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
