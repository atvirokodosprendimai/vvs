# WireGuard VPN Setup for vvs-core ↔ vvs-portal

The NATS bridge uses a private channel between vvs-core (office) and vvs-portal (VPS).
WireGuard provides an encrypted tunnel so NATS traffic never traverses the internet unprotected.

## Topology

```
[office] vvs-core  10.0.0.1/24
              |
         WireGuard tunnel (UDP, encrypted)
              |
[VPS]  vvs-portal  10.0.0.2/24
```

## 1. Install WireGuard

```bash
# Debian/Ubuntu on both machines:
apt install wireguard
```

## 2. Generate keys

Run on each machine:
```bash
wg genkey | tee privatekey | wg pubkey > publickey
```

## 3. Core (office) — /etc/wireguard/wg0.conf

```ini
[Interface]
Address    = 10.0.0.1/24
PrivateKey = <core-private-key>
ListenPort = 51820

[Peer]
# vvs-portal VPS
PublicKey  = <portal-public-key>
AllowedIPs = 10.0.0.2/32
```

## 4. Portal (VPS) — /etc/wireguard/wg0.conf

```ini
[Interface]
Address    = 10.0.0.2/24
PrivateKey = <portal-private-key>

[Peer]
# vvs-core office (needs a static IP or DDNS)
PublicKey  = <core-public-key>
Endpoint   = <office-public-ip>:51820
AllowedIPs = 10.0.0.1/32
PersistentKeepalive = 25
```

## 5. Enable

```bash
systemctl enable --now wg-quick@wg0
```

## 6. Verify

```bash
wg show          # both sides
ping 10.0.0.1    # from VPS
ping 10.0.0.2    # from office
```

## 7. Configure vvs-core

In `/etc/vvs/core.env`:
```env
NATS_LISTEN_ADDR=10.0.0.1:4222
NATS_AUTH_TOKEN=<strong-random-token>
```

In `/etc/vvs/portal.env`:
```env
NATS_URL=nats://10.0.0.1:4222
NATS_AUTH_TOKEN=<same-token>
```

## Security notes

- NATS listens only on the WireGuard interface (10.0.0.1), not on 0.0.0.0.
- `NATS_AUTH_TOKEN` adds a second layer; rotate it periodically.
- The office firewall should block port 4222 from the public internet — only the WireGuard
  interface needs access.
- vvs-portal has no path to the admin UI (served by vvs-core on a different port/host).
  Customers can reach only `/portal/*` and `/i/{token}`.
