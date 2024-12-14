// Package constants contains strings and const to be
// used through the whole application.
package constants

// Version of lilipod. This should be overwritten at compile time.
var Version = "development"

// FilterSeparator is the char we use to separate multiple filters in a single array.
const FilterSeparator = "\000"

// BusyboxURL is the download link to the statically compiled version of
// busybox to use if we've got missing dependencies.
const BusyboxURL = "https://busybox.net/downloads/binaries/1.35.0-x86_64-linux-musl/busybox"

// PtyAgentPath is the path inside the container where we put the pty agent.
const PtyAgentPath = "/sbin/pty"

// TrueString is useful for easy string comparisons with bools.
const TrueString = "true"

const (
	// KeepID is the string we use for user namespace settings.
	KeepID string = "keep-id"
	// Host is the string we use for shared namespaces.
	Host string = "host"
	// Private is the string we use for private namespaces.
	Private string = "private"
)
