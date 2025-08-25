// Copyright 2025 DoniLite. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadConfigFile(configPath string) ([]byte, error) {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "/"
	}
	defPath := path.Join(cwd, configPath)

	content, err := os.ReadFile(defPath)

	if err != nil {
		return nil, fmt.Errorf("error during the config file reading at: %s", defPath)
	}

	return content, nil
}

func DiscoverConfigFormat(configPath string) (string, error) {
	ext := path.Ext(configPath)
	var format string

	if ext == "" {
		return "", fmt.Errorf("invalid path provided")
	}

	if strings.Contains(ext, "json") {
		format = "json"
	} else if strings.Contains(ext, "yml") {
		format = "yaml"
	}

	return format, nil
}

func ParseConfig(content []byte, format string) (*Config, error) {
	var config Config
	var err error

	if format == "json" {
		err = json.Unmarshal(content, &config)
	} else {
		err = yaml.Unmarshal(content, &content)
	}

	if err != nil {
		return nil, err
	}

	return &config, nil
}

func buildServerURL(server *Server) (string, error) {
	url, err := url.Parse(server.URL)

	if err != nil || server.URL == "" {
		url, err = url.Parse(fmt.Sprintf("%s://%s:%d", server.Protocol, server.Host, server.Port))

		if err != nil {
			return "", nil
		}
	}

	return url.String(), nil
}

func SerializeHealthCheckStatus(status *HealthCheckStatus) (string, error) {
	b, err := json.Marshal(status)

	if err != nil {
		return "", err
	}

	return string(b), nil
}
