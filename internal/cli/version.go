package cli

import (
	"fmt"
	"io"
)

// Version is the current release version of ContractLens, populated at build time if available.
var Version = "v0.1.0"

// RunVersion prints the application version.
func RunVersion(w io.Writer) int {
	fmt.Fprintf(w, "ContractLens %s\n", Version)
	return 0
}
