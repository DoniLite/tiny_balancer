
package cloud

import (
	"testing"
)

func TestGetDockerConfig_PostgresDefaults(t *testing.T) {
	m := &CloudManager{}
	img, env, ports, err := m.getDockerConfig(ServiceConfig{
		Type:         PostgreSQL,
		Name:         "pg1",
		Username:     "admin",
		DatabaseName: "db1",
		Password:     "", // force autogen to verify env wiring
	})
	if err != nil {
		t.Fatalf("getDockerConfig error: %v", err)
	}
	if img != "postgres:16" {
		t.Fatalf("expected image postgres:16, got %s", img)
	}
	if env["POSTGRES_DB"] != "db1" || env["POSTGRES_USER"] != "admin" {
		t.Fatalf("bad env for postgres: %#v", env)
	}
	if env["POSTGRES_PASSWORD"] == "" {
		t.Fatalf("expected auto-generated POSTGRES_PASSWORD")
	}
	if len(ports) != 1 || ports[0] != "5432/tcp" {
		t.Fatalf("bad ports: %#v", ports)
	}
}

func TestGetDockerConfig_MySQLDefaults(t *testing.T) {
	m := &CloudManager{}
	img, env, ports, err := m.getDockerConfig(ServiceConfig{
		Type:         MySQL,
		Name:         "mysql1",
		Username:     "userx",
		DatabaseName: "dbx",
		Password:     "", // autogen
	})
	if err != nil {
		t.Fatalf("getDockerConfig error: %v", err)
	}
	if img != "mysql:8.0" {
		t.Fatalf("expected image mysql:8.0, got %s", img)
	}
	if env["MYSQL_DATABASE"] != "dbx" || env["MYSQL_USER"] != "userx" {
		t.Fatalf("bad env for mysql: %#v", env)
	}
	if env["MYSQL_PASSWORD"] == "" || env["MYSQL_ROOT_PASSWORD"] == "" {
		t.Fatalf("expected auto-generated MYSQL passwords")
	}
	if len(ports) != 1 || ports[0] != "3306/tcp" {
		t.Fatalf("bad ports: %#v", ports)
	}
}

func TestGetDockerConfig_MariaDBDefaults(t *testing.T) {
	m := &CloudManager{}
	img, env, ports, err := m.getDockerConfig(ServiceConfig{
		Type:         MariaDB,
		Name:         "mdb",
		Username:     "u",
		DatabaseName: "d",
	})
	if err != nil {
		t.Fatalf("getDockerConfig error: %v", err)
	}
	if img != "mariadb:10.9" {
		t.Fatalf("expected image mariadb:10.9, got %s", img)
	}
	if env["MARIADB_DATABASE"] != "d" || env["MARIADB_USER"] != "u" {
		t.Fatalf("bad env for mariadb: %#v", env)
	}
	if env["MARIADB_PASSWORD"] == "" || env["MARIADB_ROOT_PASSWORD"] == "" {
		t.Fatalf("expected auto-generated MARIADB passwords")
	}
	if len(ports) != 1 || ports[0] != "3306/tcp" {
		t.Fatalf("bad ports: %#v", ports)
	}
}

func TestGetDockerConfig_MongoDefaults(t *testing.T) {
	m := &CloudManager{}
	img, env, ports, err := m.getDockerConfig(ServiceConfig{
		Type:         MongoDB,
		Name:         "mongo1",
		Username:     "root",
		DatabaseName: "d1",
	})
	if err != nil {
		t.Fatalf("getDockerConfig error: %v", err)
	}
	if img != "mongo:6.0" {
		t.Fatalf("expected image mongo:6.0, got %s", img)
	}
	if env["MONGO_INITDB_DATABASE"] != "d1" || env["MONGO_INITDB_ROOT_USERNAME"] != "root" {
		t.Fatalf("bad env for mongo: %#v", env)
	}
	if env["MONGO_INITDB_ROOT_PASSWORD"] == "" {
		t.Fatalf("expected auto-generated mongo root password")
	}
	if len(ports) != 1 || ports[0] != "27017/tcp" {
		t.Fatalf("bad ports: %#v", ports)
	}
}

func TestGetDockerConfig_RedisDefaults(t *testing.T) {
	m := &CloudManager{}
	img, env, ports, err := m.getDockerConfig(ServiceConfig{
		Type:     Redis,
		Name:     "cache",
		Username: "",           // ignored
		Password: "secretpass", // verify REDIS_PASSWORD propagation
	})
	if err != nil {
		t.Fatalf("getDockerConfig error: %v", err)
	}
	if img != "redis:7" {
		t.Fatalf("expected image redis:7, got %s", img)
	}
	if env["REDIS_PASSWORD"] != "secretpass" {
		t.Fatalf("expected REDIS_PASSWORD to be set, got env=%#v", env)
	}
	if len(ports) != 1 || ports[0] != "6379/tcp" {
		t.Fatalf("bad ports: %#v", ports)
	}
}

func TestGetDockerConfig_CustomImageTagLatest(t *testing.T) {
	m := &CloudManager{}
	img, env, ports, err := m.getDockerConfig(ServiceConfig{
		Type:            "custom",
		Name:            "my/image",
		UsLatestVersion: true,
		ExposedPorts:    []string{"8080/tcp"},
		Variables:       map[string]string{"FOO": "BAR"},
	})
	if err != nil {
		t.Fatalf("getDockerConfig error: %v", err)
	}
	if img != "my/image:latest" {
		t.Fatalf("expected image my/image:latest, got %s", img)
	}
	if env["FOO"] != "BAR" {
		t.Fatalf("expected env FOO=BAR, got %#v", env)
	}
	if len(ports) != 1 || ports[0] != "8080/tcp" {
		t.Fatalf("bad ports: %#v", ports)
	}
}
