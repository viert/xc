package cli

import "fmt"

var (
	appVersion = ""
	appBuild   = ""

	kernelName    = ""
	kernelVersion = ""
	kernelArch    = ""
)

func version() string {
	if appVersion == "" {
		return "dev"
	}
	return fmt.Sprintf("%s-%s (%s %s %s)", appVersion, appBuild, kernelName, kernelVersion, kernelArch)
}
