package domain

import "fmt"

// RenderWgmeshCompose returns the docker-compose YAML to deploy wgmesh on a node.
// VVS manages this compose — it should not be edited manually on the node.
func RenderWgmeshCompose(clusterKey, _ string) string {
	return fmt.Sprintf(`# managed by VVS — do not edit manually
services:
  wgmesh-node:
    image: ghcr.io/atvirokodosprendimai/wgmesh:latest
    container_name: wgmesh-node
    network_mode: host
    restart: unless-stopped
    volumes:
      - ./data:/var/lib/wgmesh
    command: >
      join
      --secret %q
      --interface wgmesh0
      --listen-port 51820
    cap_add:
      - NET_ADMIN
      - SYS_MODULE
    logging:
      driver: "json-file"
      options:
        max-file: "5"
        max-size: "10m"
`, clusterKey)
}
