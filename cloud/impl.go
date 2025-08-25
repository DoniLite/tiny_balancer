package cloud

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/volume"
)

// create the docker service network if this don't exist yet
func (m *CloudManager) ensureNetwork() error {
	ctx := context.Background()

	networks, err := m.dockerClient.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return err
	}

	// verifying if this network exist
	for _, net := range networks {
		if net.Name == m.networkName {
			return nil
		}
	}

	// creating the network
	_, err = m.dockerClient.NetworkCreate(ctx, m.networkName, network.CreateOptions{
		Driver: "bridge",
	})
	return err
}

// return the next port available
func (m *CloudManager) getNextPort() int {
	m.portCounter++
	return m.portCounter
}

// Returning the Docker configuration for a service/db type
func (m *CloudManager) getDockerConfig(config ServiceConfig) (string, map[string]string, []string, error) {
	var image string
	var env map[string]string
	var exposedPorts []string

	// generating a random password if not provided in the config
	password := config.Password
	if password == "" {
		password = generateRandomString(16)
	}

	switch config.Type {
	case PostgreSQL:
		version := config.Version
		if version == "" {
			version = "16"
		}
		image = fmt.Sprintf("postgres:%s", version)
		env = map[string]string{
			"POSTGRES_DB":       config.DatabaseName,
			"POSTGRES_USER":     config.Username,
			"POSTGRES_PASSWORD": password,
		}
		exposedPorts = []string{"5432/tcp"}

	case MySQL:
		version := config.Version
		if version == "" {
			version = "8.0"
		}
		image = fmt.Sprintf("mysql:%s", version)
		env = map[string]string{
			"MYSQL_DATABASE":      config.DatabaseName,
			"MYSQL_USER":          config.Username,
			"MYSQL_PASSWORD":      password,
			"MYSQL_ROOT_PASSWORD": generateRandomString(16),
		}
		exposedPorts = []string{"3306/tcp"}

	case MariaDB:
		version := config.Version
		if version == "" {
			version = "10.9"
		}
		image = fmt.Sprintf("mariadb:%s", version)
		env = map[string]string{
			"MARIADB_DATABASE":      config.DatabaseName,
			"MARIADB_USER":          config.Username,
			"MARIADB_PASSWORD":      password,
			"MARIADB_ROOT_PASSWORD": generateRandomString(16),
		}
		exposedPorts = []string{"3306/tcp"}

	case MongoDB:
		version := config.Version
		if version == "" {
			version = "6.0"
		}
		image = fmt.Sprintf("mongo:%s", version)
		env = map[string]string{
			"MONGO_INITDB_DATABASE":      config.DatabaseName,
			"MONGO_INITDB_ROOT_USERNAME": config.Username,
			"MONGO_INITDB_ROOT_PASSWORD": password,
		}
		exposedPorts = []string{"27017/tcp"}

	case Redis:
		version := config.Version
		if version == "" {
			version = "7"
		}
		image = fmt.Sprintf("redis:%s", version)
		env = map[string]string{}
		exposedPorts = []string{"6379/tcp"}

		// Redis avec authentification
		if password != "" {
			env["REDIS_PASSWORD"] = password
		}

	default:
		version := config.Version
		image = config.Name
		env = config.Variables
		exposedPorts = config.ExposedPorts
		if version == "" && config.UsLatestVersion {
			version = "latest"
			image = fmt.Sprintf("%s:%s", config.Name, version)
		}

		if image == "" {
			return "", nil, nil, fmt.Errorf("please provide a valid name for your service")
		}
	}

	return image, env, exposedPorts, nil
}

// Creating a new service instance based on the provided configuration
// The service is created with default values if not provided
// After creating the instance, it will be started and exposed to a public port on your machine
func (m *CloudManager) CreateInstance(config ServiceConfig) (*ServiceInstance, error) {
	ctx := context.Background()

	if config.Username == "" {
		config.Username = "admin"
	}
	if config.DatabaseName == "" {
		config.DatabaseName = config.Name
	}

	image, envVars, exposedPorts, err := m.getDockerConfig(config)
	if err != nil {
		return nil, err
	}

	instanceID := generateRandomString(12)
	instance := &ServiceInstance{
		ID:           instanceID,
		Name:         config.Name,
		Type:         config.Type,
		Username:     config.Username,
		Password:     config.Password,
		DatabaseName: config.DatabaseName,
		Status:       "creating",
		CreatedAt:    time.Now(),
		ExternalPort: m.getNextPort(),
	}

	labels := map[string]string{}
	publishToHost := true

	// find first internal port used by the service (e.g., 5432/tcp)
	internalPort := ""
	if len(exposedPorts) > 0 {
		internalPort = strings.Split(exposedPorts[0], "/")[0]
	}

	if config.Domain != nil && config.Domain.Domain != "" && internalPort != "" {
		routerName := fmt.Sprintf("db-%s-%s", config.Type, instanceID)
		entryPoint := config.Domain.EntryPoint
		if entryPoint == "" {
			entryPoint = "websecure"
		}
		certResolver := config.Domain.CertResolver
		if certResolver == "" {
			certResolver = "letsencrypt"
		}

		labels = map[string]string{
			"traefik.enable": "true",
			// TCP router
			fmt.Sprintf("traefik.tcp.routers.%s.rule", routerName):             fmt.Sprintf("HostSNI(`%s`)", config.Domain.Domain),
			fmt.Sprintf("traefik.tcp.routers.%s.entrypoints", routerName):      entryPoint,
			fmt.Sprintf("traefik.tcp.routers.%s.tls", routerName):              "true",
			fmt.Sprintf("traefik.tcp.routers.%s.tls.certresolver", routerName): certResolver,
			fmt.Sprintf("traefik.tcp.routers.%s.service", routerName):          routerName,
			// TCP service to container port
			fmt.Sprintf("traefik.tcp.services.%s.loadbalancer.server.port", routerName): internalPort,
		}

		// Optional IP allow‑list middleware for TCP
		if len(config.Domain.AllowedCIDRs) > 0 {
			mw := routerName + "-allow"
			labels[fmt.Sprintf("traefik.tcp.middlewares.%s.ipallowlist.sourcerange", mw)] =
				strings.Join(config.Domain.AllowedCIDRs, ",")
			labels[fmt.Sprintf("traefik.tcp.routers.%s.middlewares", routerName)] = mw
		}

		instance.Domain = config.Domain.Domain
		publishToHost = false // rely on Traefik by default when domain set
	}

	containerConfig := &container.Config{
		Image:  image,
		Env:    make([]string, 0, len(envVars)),
		Labels: labels,
	}

	for key, value := range envVars {
		containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Ports config
	portBindings := nat.PortMap{}
	exposedPortsMap := nat.PortSet{}

	for _, port := range exposedPorts {
		natPort, _ := nat.NewPort("tcp", strings.Split(port, "/")[0])
		exposedPortsMap[natPort] = struct{}{}
		if publishToHost {
			portBindings[natPort] = []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: strconv.Itoa(instance.ExternalPort)},
			}
		}
	}

	containerConfig.ExposedPorts = exposedPortsMap

	// ---- Volumes: create named volumes and mount them
	mounts := []mount.Mount{}
	volNames := []string{}
	if len(config.Volumes) == 0 {
		if dataDir, ok := defaultDataDirFor(config.Type); ok {
			config.Volumes = []VolumeMount{
				{Name: fmt.Sprintf("%s-data-%s", config.Type, instanceID), ContainerPath: dataDir, Driver: "local"},
			}
		}
	}
	for _, v := range config.Volumes {
		name := v.Name
		if name == "" {
			name = fmt.Sprintf("%s-vol-%s", config.Type, generateRandomString(6))
		}
		if _, err := m.dockerClient.VolumeCreate(ctx, volume.CreateOptions{
			Name:       name,
			Driver:     v.Driver,
			DriverOpts: v.DriverOpts,
			Labels: map[string]string{
				"managed-by": "cloud-db-manager",
				"service":    string(config.Type),
				"instance":   instanceID,
			},
		}); err != nil {
			return nil, fmt.Errorf("volume creation error (%s): %v", name, err)
		}
		volNames = append(volNames, name)
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeVolume,
			Source:   name,
			Target:   v.ContainerPath,
			ReadOnly: v.ReadOnly,
		})
	}
	instance.VolumeNames = volNames

	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
		Mounts: mounts,
	}

	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			m.networkName: {},
		},
	}

	containerName := fmt.Sprintf("cloud-db-%s-%s", config.Type, instanceID)
	resp, err := m.dockerClient.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		networkConfig,
		nil,
		containerName,
	)
	if err != nil {
		return nil, fmt.Errorf("container creation error: %v", err)
	}

	instance.ContainerID = resp.ID

	// Démarrer le conteneur
	if err := m.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("container starting error: %v", err)
	}

	instance.Status = "running"
	m.instances[instanceID] = instance

	return instance, nil
}

// Get an service instance from its ID
func (m *CloudManager) GetInstance(id string) (*ServiceInstance, bool) {
	instance, exists := m.instances[id]
	return instance, exists
}

// This returns all service instances
func (m *CloudManager) ListInstances() []*ServiceInstance {
	instances := make([]*ServiceInstance, 0, len(m.instances))
	for _, instance := range m.instances {
		instances = append(instances, instance)
	}
	return instances
}

// Deleting a service instance be careful when using this in production
func (m *CloudManager) DeleteInstance(id string) error {
	instance, exists := m.instances[id]
	if !exists {
		return fmt.Errorf("instance not found: %s", id)
	}

	ctx := context.Background()

	// Stop and remove the container
	if err := m.dockerClient.ContainerStop(ctx, instance.ContainerID, container.StopOptions{}); err != nil {
		log.Printf("Error stopping container: %v", err)
	}

	if err := m.dockerClient.ContainerRemove(ctx, instance.ContainerID, container.RemoveOptions{}); err != nil {
		return fmt.Errorf("error removing container: %v", err)
	}

	delete(m.instances, id)
	return nil
}

// RecreateWithDomain — convenience to switch/add domain (labels are immutable)
func (m *CloudManager) RecreateWithDomain(id string, domain *DomainConfig) (*ServiceInstance, error) {
	inst, ok := m.instances[id]
	if !ok {
		return nil, fmt.Errorf("instance not found: %s", id)
	}
	// read back its “shape” as best as we can (you’d keep original ServiceConfig in a real system)
	cfg := ServiceConfig{
		Type:         inst.Type,
		Name:         inst.Name,
		Username:     inst.Username,
		Password:     inst.Password,
		DatabaseName: inst.DatabaseName,
		// Let CreateInstance derive image/env/ports again
		Domain: domain,
		// restore previous volumes (mount at known paths by type)
	}
	// best effort: re-attach existing volume names to default data dir
	if len(inst.VolumeNames) > 0 {
		if path, ok := defaultDataDirFor(inst.Type); ok {
			cfg.Volumes = []VolumeMount{{Name: inst.VolumeNames[0], ContainerPath: path}}
		}
	}
	// delete current container and recreate
	if err := m.DeleteInstance(id); err != nil {
		return nil, err
	}
	return m.CreateInstance(cfg)
}

// CreateTraefikBundle — programmatically run Traefik with ACME on your network
// Equivalent to the docker-compose in the docs; keeps ops inside Go if desired.
func (m *CloudManager) CreateTraefikBundle(acmeEmail string) (string, error) {
	ctx := context.Background()

	// Pull image if needed
	if _, err := m.dockerClient.ImagePull(ctx, "traefik:v3.0", image.PullOptions{}); err != nil {
		// ignore streaming body; just attempt pull
		log.Printf("warning: image pull may have failed (non-fatal): %v", err)
	}

	cfg := &container.Config{
		Image:  "traefik:v3.0",
		Labels: map[string]string{
			// none: we control via CLI flags
		},
	}
	hostCfg := &container.HostConfig{
		PortBindings: nat.PortMap{
			"443/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "443"}},
		},
		Binds: []string{
			"/var/run/docker.sock:/var/run/docker.sock:ro",
		},
	}
	// Static Traefik flags for Docker provider & ACME TLS‑ALPN on :443
	cfg.Cmd = []string{
		"--providers.docker=true",
		"--providers.docker.exposedbydefault=false",
		"--entrypoints.websecure.address=:443/tcp",
		"--certificatesresolvers.letsencrypt.acme.tlschallenge=true",
		fmt.Sprintf("--certificatesresolvers.letsencrypt.acme.email=%s", acmeEmail),
		"--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json",
	}
	// persist ACME store in an anonymous volume
	hostCfg.Mounts = append(hostCfg.Mounts, mount.Mount{
		Type:   mount.TypeVolume,
		Source: "traefik-acme",
		Target: "/letsencrypt",
	})
	netCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			m.networkName: {},
		},
	}

	resp, err := m.dockerClient.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, "traefik-proxy")
	if err != nil {
		return "", fmt.Errorf("traefik container creation error: %v", err)
	}
	if err := m.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("traefik start error: %v", err)
	}
	return resp.ID, nil
}
