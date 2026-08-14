package config

import (
	"fmt"
	"os"
	"strconv"
)

const developmentAuthEnvironmentVariable = "KAERU_DEV_AUTH"

type Runtime struct {
	DevelopmentAuth bool
}

func LoadRuntime() (Runtime, error) {
	rawValue, exists := os.LookupEnv(developmentAuthEnvironmentVariable)
	if !exists || rawValue == "" {
		return Runtime{}, nil
	}

	developmentAuth, err := strconv.ParseBool(rawValue)
	if err != nil {
		return Runtime{}, fmt.Errorf("parse %s: %w", developmentAuthEnvironmentVariable, err)
	}

	return Runtime{DevelopmentAuth: developmentAuth}, nil
}
