package plugin

import (
	"github.com/cloudquery/plugin-sdk/v4/plugin"
)

var (
	Version = "development"
)

func Plugin() *plugin.Plugin {
	return plugin.NewPlugin("github.com/infobloxopen/cq-destination-postgresql/cq-source-test-test", Version, Configure)
}
