package registry

import "strings"

// dockerImages maps official and well-known images to the repositories their
// software is developed in. Images whose project does not take GitHub issues
// (postgres, mysql, mongo, mariadb, httpd, ruby, cassandra, kafka, wordpress)
// are left out: there is nothing to search.
var dockerImages = map[string]string{
	// databases, caches and search
	"redis":         "redis/redis",
	"valkey/valkey": "valkey-io/valkey",
	"memcached":     "memcached/memcached",
	"elasticsearch": "elastic/elasticsearch",
	"docker.elastic.co/elasticsearch/elasticsearch": "elastic/elasticsearch",
	"kibana":                          "elastic/kibana",
	"docker.elastic.co/kibana/kibana": "elastic/kibana",
	"opensearchproject/opensearch":    "opensearch-project/OpenSearch",
	"couchdb":                         "apache/couchdb",
	"influxdb":                        "influxdata/influxdb",
	"neo4j":                           "neo4j/neo4j",
	"clickhouse/clickhouse-server":    "ClickHouse/ClickHouse",
	"timescale/timescaledb":           "timescale/timescaledb",
	"pgvector/pgvector":               "pgvector/pgvector",
	"supabase/postgres":               "supabase/postgres",
	"surrealdb/surrealdb":             "surrealdb/surrealdb",
	"qdrant/qdrant":                   "qdrant/qdrant",
	"getmeili/meilisearch":            "meilisearch/meilisearch",
	"typesense/typesense":             "typesense/typesense",
	"docker.dragonflydb.io/dragonflydb/dragonfly": "dragonflydb/dragonfly",
	"etcd":                "etcd-io/etcd",
	"quay.io/coreos/etcd": "etcd-io/etcd",
	// queues and messaging
	"rabbitmq":              "rabbitmq/rabbitmq-server",
	"nats":                  "nats-io/nats-server",
	"redpandadata/redpanda": "redpanda-data/redpanda",
	"eclipse-mosquitto":     "eclipse-mosquitto/mosquitto",
	"temporalio/auto-setup": "temporalio/temporal",
	"temporalio/server":     "temporalio/temporal",
	// web servers and proxies
	"nginx":    "nginx/nginx",
	"haproxy":  "haproxy/haproxy",
	"traefik":  "traefik/traefik",
	"caddy":    "caddyserver/caddy",
	"registry": "distribution/distribution",
	// language runtimes and build tools
	"node":            "nodejs/node",
	"python":          "python/cpython",
	"golang":          "golang/go",
	"php":             "php/php-src",
	"rust":            "rust-lang/rust",
	"denoland/deno":   "denoland/deno",
	"oven/bun":        "oven-sh/bun",
	"dart":            "dart-lang/sdk",
	"swift":           "swiftlang/swift",
	"elixir":          "elixir-lang/elixir",
	"erlang":          "erlang/otp",
	"gradle":          "gradle/gradle",
	"jenkins/jenkins": "jenkinsci/jenkins",
	// observability, auth and other services
	"grafana/grafana":                      "grafana/grafana",
	"grafana/loki":                         "grafana/loki",
	"prom/prometheus":                      "prometheus/prometheus",
	"jaegertracing/all-in-one":             "jaegertracing/jaeger",
	"otel/opentelemetry-collector":         "open-telemetry/opentelemetry-collector",
	"otel/opentelemetry-collector-contrib": "open-telemetry/opentelemetry-collector-contrib",
	"hashicorp/consul":                     "hashicorp/consul",
	"hashicorp/vault":                      "hashicorp/vault",
	"quay.io/keycloak/keycloak":            "keycloak/keycloak",
	"axllent/mailpit":                      "axllent/mailpit",
	"mailhog/mailhog":                      "mailhog/MailHog",
	"hasura/graphql-engine":                "hasura/graphql-engine",
	"ollama/ollama":                        "ollama/ollama",
	"ghost":                                "TryGhost/Ghost",
	"nextcloud":                            "nextcloud/server",
	"mcr.microsoft.com/playwright":         "microsoft/playwright",
	"selenium/standalone-chrome":           "SeleniumHQ/docker-selenium",
}

// DockerImageRepo returns the GitHub repository behind an image name (without
// its tag), or "" when it is not known. Images on ghcr.io are named after theirs.
func DockerImageRepo(image string) string {
	if repo, ok := dockerImages[image]; ok {
		return repo
	}
	if rest, ok := strings.CutPrefix(image, "ghcr.io/"); ok {
		if parts := strings.Split(rest, "/"); len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}
	return ""
}
