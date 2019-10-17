package main

import (
	"log"

	"github.com/alexandre-normand/slackscot"
	"github.com/alexandre-normand/slackscot/config"
	slackscotPlugins "github.com/alexandre-normand/slackscot/plugins"
	"github.com/spf13/pflag"

	"github.com/cflee/prime-sentinel/plugins"
)

const (
	name    = "prime-sentinel"
	version = "0.0.1"
)

func main() {
	// flags come first as they set config file path
	configurationPath := pflag.String("config", "config.yml", "path to config file")
	pflag.Parse()

	// config
	v := config.NewViperWithDefaults()
	// enable env vars to override any part of config
	v.AutomaticEnv()
	// map some common env var names to config keys
	v.BindEnv(config.TokenKey, "SLACK_TOKEN")

	v.SetConfigFile(*configurationPath)
	err := v.ReadInConfig()
	if err != nil {
		log.Fatalf("Error loading configuration file [%s]: %v", *configurationPath, err)
	}

	options := make([]slackscot.Option, 0)

	bot, err := slackscot.NewBot(name, v, options...).
		WithPlugin(slackscotPlugins.NewVersionner(name, version)).
		WithConfigurablePluginErr(plugins.QuoterPluginName, func(c *config.PluginConfig) (*slackscot.Plugin, error) {
			return plugins.NewQuoter(c)
		}).
		Build()
	defer bot.Close()
	if err != nil {
		log.Fatal(err)
	}

	err = bot.Run()
	if err != nil {
		log.Fatal(err)
	}
}
