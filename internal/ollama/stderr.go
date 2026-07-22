package ollama

import (
	"io"
	"os"
)

// stderrWriter is the destination for all diagnostic output.
// Tests can swap this for a bytes.Buffer to capture output.
var stderrWriter io.Writer = os.Stderr
