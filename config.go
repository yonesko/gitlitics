package main

import (
	"errors"
	"flag"
	"github.com/creasty/defaults"
	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
)

type config struct {
	Paths  []string     `yaml:"paths"`
	Author authorConfig `yaml:"author"`
}

type authorConfig struct {
	Key        string              `yaml:"key" validate:"oneof=name mail" default:"name"`
	Duplicates map[string][]string `yaml:"duplicates"`
}

var paths = flag.String("paths", "", "example: https://gitlab.int.tsum.com/preowned/simona/delta/customer-service.git, comma-separated")
var configPath = flag.String("conf", "gitlitics.yml", "path to config")

func parseConfig() (config, error) {
	flag.Parse()

	bytes, err := os.ReadFile(*configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return config{}, err
	}
	c := config{}
	err = yaml.Unmarshal(bytes, &c)
	if err != nil {
		return config{}, err
	}
	err = defaults.Set(&c)
	if err != nil {
		return config{}, err
	}

	if paths != nil && len(*paths) != 0 {
		c.Paths = strings.Split(*paths, ",")
	}

	err = validator.New().Struct(c)
	if err != nil {
		return config{}, err
	}

	return c, nil
}
