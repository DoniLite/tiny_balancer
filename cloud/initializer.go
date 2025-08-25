package cloud

import (
	"fmt"

	"github.com/moby/moby/client"
)

func NewCloudDBManager() (*CloudManager, error) {
	// 1) Try env/context as-is
	if cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation()); err == nil {
		if err := pingOK(cli); err == nil {
			return initManager(cli)
		}
		_ = cli.Close()
	}

	// 2) Ask `docker context` (same info your terminal shows) → use Endpoints.docker.Host
	if host, err := resolveDockerHostViaCLI(); err == nil {
		if cli, err := client.NewClientWithOpts(client.WithHost(host), client.WithAPIVersionNegotiation()); err == nil {
			if err := pingOK(cli); err == nil {
				return initManager(cli)
			}
			_ = cli.Close()
		}
	}

	// 3) Fallback guesses (rootless, classic, tcp)
	for _, h := range dockerHostCandidates() {
		if cli, err := client.NewClientWithOpts(client.WithHost(h), client.WithAPIVersionNegotiation()); err == nil {
			if err := pingOK(cli); err == nil {
				return initManager(cli)
			}
			_ = cli.Close()
		}
	}

	return nil, fmt.Errorf("docker client creation error: no reachable Docker daemon (start Docker Desktop / set a context)")
}
