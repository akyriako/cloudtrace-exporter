package provider

import (
	"os"

	"gopkg.in/yaml.v2"
)

type CloudAuth struct {
	ProjectName string `yaml:"project_name"`
	ProjectID   string `yaml:"project_id"`
	DomainName  string `yaml:"domain_name"`
	AccessKey   string `yaml:"access_key"`
	Region      string `yaml:"region"`
	SecretKey   string `yaml:"secret_key"`
	AuthURL     string `yaml:"auth_url"`
	UserName    string `yaml:"user_name"`
	Password    string `yaml:"password"`
}

type Global struct {
	Port            string `yaml:"port"`
	Prefix          string `yaml:"prefix"`
	MetricsPath     string `yaml:"metrics_path"`
	MaxRoutines     int    `yaml:"max_routines"`
	ScrapeBatchSize int    `yaml:"scrape_batch_size"`
}

type CloudConfig struct {
	Auth   CloudAuth `yaml:"auth"`
	Global Global    `yaml:"global"`
}

func GetConfigFromFile(configPath string) (*CloudConfig, error) {
	var config CloudConfig

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, err
}
