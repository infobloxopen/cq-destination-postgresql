package plugin

import (
	"github.com/cloudquery/plugin-sdk/v4/plugin"
)

var (
	Version = "development"
)

func Plugin() *plugin.Plugin {
	return plugin.NewPlugin("github.com/infobloxopen/cq-plugin-dest-postgres/cq-source-test-test", Version, Configure)
}
