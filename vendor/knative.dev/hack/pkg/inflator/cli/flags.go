package cli

import (
	"os"
	"strings"
)

const (
	// ManualVerboseEnvVar is the environment variable that can be set to disable
	// automatic verbose mode on CI servers.
	ManualVerboseEnvVar = "KNATIVE_HACK_SCRIPT_MANUAL_VERBOSE"
)

type flags struct {
	verbose bool
}

func parseArgs(ex *Execution) (*flags, error) {
	f := flags{
		verbose: isCiServer(),
	}
	if len(ex.Args) == 0 {
		return nil, usageErr{}
	}
	for i := 0; i < len(ex.Args); i++ {
		if ex.Args[i] == "-v" || ex.Args[i] == "--verbose" {
			f.verbose = true
			ex.Args = append(ex.Args[:i], ex.Args[i+1:]...)
			i--
		}

		if ex.Args[i] == "-h" || ex.Args[i] == "--help" {
			return nil, usageErr{}
		}
	}
	return &f, nil
}

func isCiServer() bool {
	if strings.HasPrefix(strings.ToLower(os.Getenv(ManualVerboseEnvVar)), "t") {
		return false
	}
	return os.Getenv("CI") != "" ||
		os.Getenv("BUILD_ID") != "" ||
		os.Getenv("PROW_JOB_ID") != ""
}
