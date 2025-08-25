package cloud

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/moby/moby/client"
)

func generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}

func defaultDataDirFor(t ServiceType) (string, bool) {
	switch t {
	case PostgreSQL:
		return "/var/lib/postgresql/data", true
	case MySQL, MariaDB:
		return "/var/lib/mysql", true
	case MongoDB:
		return "/data/db", true
	case Redis:
		return "/data", true
	default:
		return "", false
	}
}

func initManager(cli *client.Client) (*CloudManager, error) {
	manager := &CloudManager{
		dockerClient: cli,
		instances:    make(map[string]*ServiceInstance),
		portCounter:  5432,
		networkName:  "cloud-db-network",
	}
	if err := manager.ensureNetwork(); err != nil {
		return nil, fmt.Errorf("network creation error: %v", err)
	}
	return manager, nil
}
