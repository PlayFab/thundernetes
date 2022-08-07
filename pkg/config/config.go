package config

import (
	"flag"
	"io/ioutil"
	"os"
	"strings"

	"github.com/grafana/dskit/flagext"
)

const (
	ConfigFileParam = "config-file"
	ExpandENVParam  = "config-expand-env"
)

// ParseConfigFileAndEnvParams parses the configFile and expandENV params via separate flag set as these
// parameters need to be processed in order to load the config file
// AND THEN process any remaining CLI params to override values.
func ParseConfigFileAndEnvParams(args []string) (string, bool) {
	var configFile = ""
	var expandENV = false
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.StringVar(&configFile, ConfigFileParam, "", "")
	fs.BoolVar(&expandENV, ExpandENVParam, false, "")
	fs.SetOutput(ioutil.Discard)
	for len(args) > 0 {
		// we don't care about errors here. Because flag.Parse exits on the first error
		// which will usually be UnknownFlag, we just keep iterating to process the flags
		// we are looking for.
		// Anything critical will be caught in the main
		// param flag processing.
		_ = fs.Parse(args)
		args = args[1:]
	}
	return configFile, expandENV
}

// ExpandEnvInConfigFile replaces ${var} or $var to the values of the current environment variables.
// Default values can be specified with ${var:default} format
func ExpandEnvInConfigFile(config []byte) []byte {
	return []byte(os.Expand(string(config), func(envName string) string {
		nameAndDefaultValue := strings.SplitN(envName, ":", 2)
		envName = nameAndDefaultValue[0]

		envValue, ok := os.LookupEnv(envName)
		if !ok && len(nameAndDefaultValue) == 2 {
			// env was not found, use default
			envValue = nameAndDefaultValue[1]
		}
		return envValue
	}))
}

func IgnoreConfigAndEnvParamsFromFlags(flags *flag.FlagSet) {
	flagext.IgnoredFlag(flags, ConfigFileParam, "Configuration file to load.")
	flagext.IgnoredFlag(flags, ExpandENVParam, "Expands ${var} or $var in config according to the values of the environment variables.")
}
