package domain

import "fmt"

// RenderWgmeshCompose returns the docker-compose YAML to deploy wgmesh on a node.
// VVS manages this compose — it should not be edited manually on the node.
func RenderWgmeshCompose(clusterKey, hostname string) string {
	return fmt.Sprintf(`# managed by VVS — do not edit manually
services:
  wgmesh:
    image: ghcr.io/atvirokodosprendimai/wgmesh:latest
    cap_add: [NET_ADMIN]
    network_mode: host
    environment:
      WGMESH_KEY: %q
      HOSTNAME: %q
    restart: always
`, clusterKey, hostname)
}
