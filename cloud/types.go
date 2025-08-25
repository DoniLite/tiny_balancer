package cloud

import (
	"time"

	"github.com/moby/moby/client"
)

// Represents a docker service name
type ServiceType string

const (
	PostgreSQL ServiceType = "postgresql"
	MySQL      ServiceType = "mysql"
	MongoDB    ServiceType = "mongodb"
	Redis      ServiceType = "redis"
	MariaDB    ServiceType = "mariadb"
)

// Represents a docker service instance
type ServiceInstance struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Type         ServiceType `json:"type"`
	ContainerID  string      `json:"container_id"`
	Port         int         `json:"port"`
	Username     string      `json:"username,omitempty"`
	Password     string      `json:"password,omitempty"`
	DatabaseName string      `json:"database_name,omitempty"`
	Status       string      `json:"status"`
	CreatedAt    time.Time   `json:"created_at"`
	ExternalPort int         `json:"external_port"`

	Domain      string   `json:"domain,omitempty"`
	VolumeNames []string `json:"volume_names,omitempty"`
}

// The necessary config for a docker service creation
type ServiceConfig struct {
	Type            ServiceType       `json:"type,omitempty" yaml:"type,omitempty"`
	Name            string            `json:"name" yaml:"name"`
	Username        string            `json:"username,omitempty" yaml:"username,omitempty"`
	Password        string            `json:"password,omitempty" yaml:"password,omitempty"`
	DatabaseName    string            `json:"database_name,omitempty" yaml:"database_name,omitempty"`
	Version         string            `json:"version,omitempty" yaml:"version,omitempty"`
	UsLatestVersion bool              `json:"use_latest_version,omitempty" yaml:"use_latest_version,omitempty"`
	Variables       map[string]string `json:"variables,omitempty" yaml:"variables,omitempty"`
	ExposedPorts    []string          `json:"exposed_ports,omitempty" yaml:"exposed_ports,omitempty"`

	Volumes []VolumeMount `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Domain  *DomainConfig `json:"domain,omitempty" yaml:"domain,omitempty"`
}

// Managing the docker service instances
type CloudManager struct {
	dockerClient *client.Client
	instances    map[string]*ServiceInstance
	portCounter  int
	networkName  string
}

type VolumeMount struct {
	Name          string            `json:"name" yaml:"name"`                     // docker volume name
	ContainerPath string            `json:"container_path" yaml:"container_path"` // e.g. /var/lib/postgresql/data
	ReadOnly      bool              `json:"read_only,omitempty" yaml:"read_only,omitempty"`
	Driver        string            `json:"driver,omitempty" yaml:"driver,omitempty"` // "local" etc.
	DriverOpts    map[string]string `json:"driver_opts,omitempty" yaml:"driver_opts,omitempty"`
}

type DomainConfig struct {
	Domain       string   `json:"domain" yaml:"domain"`                                   // e.g. donidb.com
	CertResolver string   `json:"cert_resolver,omitempty" yaml:"cert_resolver,omitempty"` // default "letsencrypt"
	EntryPoint   string   `json:"entrypoint,omitempty" yaml:"entrypoint,omitempty"`       // default "websecure" (443)
	AllowedCIDRs []string `json:"allowed_cidrs,omitempty" yaml:"allowed_cidrs,omitempty"` // optional IP allow-list
}
