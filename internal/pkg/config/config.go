package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

type Config struct {
	BotID              string `yaml:"bot_id"`
	SlackToken         string `yaml:"slack_token"`
	SlackSigningSecret string `yaml:"slack_signing_secret"`
}

func New() (c *Config) {
	c = &Config{
		BotID:      "",
		SlackToken: "",
	}
	return
}

func (c *Config) Parse(fileName string) (err error) {
	// load the yaml file
	yamlFile, err := ioutil.ReadFile(fileName)
	if err != nil {
		return
	}
	// parse the yaml file
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		return
	}
	return
}
