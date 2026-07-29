package build

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Application struct {
	Name       string `yaml:"name"`
	Identifier string `yaml:"identifier"`
	Version    string `yaml:"version"`
	Build      int    `yaml:"build"`
}

type Commands struct {
	Web     string `yaml:"web"`
	Server  string `yaml:"server"`
	Desktop string `yaml:"desktop"`
	Mobile  string `yaml:"mobile"`
	TUI     string `yaml:"tui"`
}

type IOS struct {
	MinimumVersion     string `yaml:"minimum_version"`
	SigningIdentity    string `yaml:"signing_identity"`
	ProvisioningProfile string `yaml:"provisioning_profile"`
}

type Android struct {
	MinimumSDK int `yaml:"minimum_sdk"`
	TargetSDK  int `yaml:"target_sdk"`
}

type Toolchain struct {
	CC  string `yaml:"cc"`
	CXX string `yaml:"cxx"`
}

type Desktop struct {
	Toolchains map[string]Toolchain `yaml:"toolchains"`
}

type Web struct {
	Output string `yaml:"output"`
	Loader string `yaml:"loader"`
}

type Targets struct {
	IOS     IOS     `yaml:"ios"`
	Android Android `yaml:"android"`
	Web     Web     `yaml:"web"`
	Desktop Desktop `yaml:"desktop"`
}

type Config struct {
	Version     int         `yaml:"version"`
	Application Application `yaml:"application"`
	Commands    Commands    `yaml:"commands"`
	Targets     Targets     `yaml:"targets"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, err
	}
	if config.Version != 1 {
		return Config{}, fmt.Errorf("unsupported quicken config version %d", config.Version)
	}
	if config.Application.Name == "" || config.Application.Identifier == "" {
		return Config{}, fmt.Errorf("application.name and application.identifier are required")
	}
	if config.Application.Version == "" {
		config.Application.Version = "0.1.0"
	}
	if config.Application.Build == 0 {
		config.Application.Build = 1
	}
	if config.Targets.IOS.MinimumVersion == "" {
		config.Targets.IOS.MinimumVersion = "13.0"
	}
	if config.Targets.Android.MinimumSDK == 0 {
		config.Targets.Android.MinimumSDK = 26
	}
	if config.Targets.Android.TargetSDK == 0 {
		config.Targets.Android.TargetSDK = 36
	}
	if config.Targets.Web.Output == "" {
		config.Targets.Web.Output = "web"
	}
	return config, nil
}
