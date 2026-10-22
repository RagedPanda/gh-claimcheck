package manifest

import "testing"

func TestDocker(t *testing.T) {
	expect(t, "Dockerfile", `ARG BASE=node:22
FROM golang:1.23-alpine AS build
FROM --platform=$BUILDPLATFORM docker.io/library/node:22-slim AS web
FROM build AS test
FROM gcr.io/distroless/static@sha256:0123abcd
FROM ${BASE}
FROM scratch
`, map[string][2]bool{"golang": direct, "node": direct, "gcr.io/distroless/static": direct}, "build", "scratch", "test")
	expect(t, "compose.yaml", `services:
  db:
    image: postgres:16
  cache:
    image: "redis:7-alpine"
  search:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.15.0
  app:
    build: .
`, map[string][2]bool{"postgres": direct, "redis": direct, "docker.elastic.co/elasticsearch/elasticsearch": direct})
	for _, name := range []string{"Dockerfile.dev", "api.Dockerfile", "Containerfile", "docker-compose.override.yml", "compose.prod.yaml"} {
		if !Supported(name) {
			t.Errorf("%s should be supported", name)
		}
	}
}
