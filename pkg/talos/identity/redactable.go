package identity

// Redactable ensures sensitive data is not logged.
type Redactable interface {
	LogString() string
}

const RedactedMarker = "<REDACTED>"

// String implements the fmt.Stringer interface to ensure redaction.
func (i *Identity) String() string {
	return RedactedMarker
}

// LogString returns the redacted string.
func (i *Identity) LogString() string {
	return RedactedMarker
}
