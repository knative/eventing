package cli

type usageErr struct{}

func (u usageErr) Retcode() int {
	return 0
}

func (u usageErr) Error() string {
	return `Hacks as Go self-extracting binary

Will extract Hack scripts to a temporary directory, and provide a source
file path to requested shell script.

# In Bash script
source "$(go run knative.dev/hack/cmd/script@latest library.sh)"

Usage:
	script [flags] library.sh

Flags:
	-h, --help      help
	-v, --verbose   verbose output
`
}
