# unifi-external-dns

![Contributors](https://img.shields.io/github/contributors/gianmarco-mameli/unifi-external-dns?style=plastic) ![Forks](https://img.shields.io/github/forks/gianmarco-mameli/unifi-external-dns?style=plastic) ![Stargazers](https://img.shields.io/github/stars/gianmarco-mameli/unifi-external-dns?style=plastic) ![Issues](https://img.shields.io/github/issues/gianmarco-mameli/unifi-external-dns?style=plastic) ![License](https://img.shields.io/github/license/gianmarco-mameli/unifi-external-dnsstyle=plastic)

In my home K3S clusters, I'm already using this great projects to sync custom DNS Policies to a Unifi Dream Machine PRO Se router:

- <https://github.com/kashalls/external-dns-unifi-webhook>
- <https://github.com/kubernetes-sigs/external-dns>

but, other than my clusters, I also have some single docker nodes for IOT on Raspberry Pi, so, with a little help from Copilot, I decided to build from scratch a little service image to update the DNS Policies on my UDM PRO SE.
This allows me to completely remove any local dnsmasq on the nodes and manage all my internal dns entry for the services on a single point (the router in that case)

This image specifically sync Unifi DNS policies from Docker container labels and an optional YAML file to a Unifi Controller, via Network API Token
Also it's implemented a 'owner txt' like that one that uses the original external-dns to keep track of the entries

Docker images are available here <https://hub.docker.com/r/gmmserv/unifi-external-dns>

## Features

- Authenticates using `UNIFI_URL` and `UNIFI_API_TOKEN`.
- Reads DNS entries from Docker container labels (`unifi.dns.domain`, `unifi.dns.type`, `unifi.dns.value`).
- Reads extra entries from a YAML file via `UNIFI_DNS_YAML_PATH`.
- Optional TXT ownership registry records (ExternalDNS-style) using a configurable prefix.
- Creates or updates policies; optional pruning of policies not in the desired set.

## Labels

Each container can publish one or more DNS records. Use indexed labels for multiple records:

### Single record (legacy format)

- `unifi.dns.domain` (example: `app.example.com`)
- `unifi.dns.type` (`A`, `A_RECORD`, `CNAME`, `CNAME_RECORD`, `SRV`, `SRV_RECORD`)
- `unifi.dns.value` (IPv4 for `A`, target name for `CNAME`)
- `unifi.dns.ttl` (optional, seconds)

### Multiple records (indexed format)

Use an index (1, 2, 3, ...) to publish multiple records:

- `unifi.dns.1.domain`, `unifi.dns.1.type`, `unifi.dns.1.value`, `unifi.dns.1.ttl`
- `unifi.dns.2.domain`, `unifi.dns.2.type`, `unifi.dns.2.value`, `unifi.dns.2.ttl`

### SRV records

For `SRV_RECORD` type, use:

- `unifi.dns.<index>.domain` (base domain)
- `unifi.dns.<index>.type` = `SRV`
- `unifi.dns.<index>.service` (e.g., `_node_exporter`)
- `unifi.dns.<index>.protocol` (e.g., `_tcp`)
- `unifi.dns.<index>.server` (target hostname)
- `unifi.dns.<index>.port` (port number)
- `unifi.dns.<index>.priority` (optional, default 10)
- `unifi.dns.<index>.weight` (optional, default 5)
- `unifi.dns.<index>.ttl` (optional)

The full SRV domain is constructed as: `<service>.<protocol>.<domain>` (e.g., `_node_exporter._tcp.example.com`)

## YAML format

Set `UNIFI_DNS_YAML_PATH` to a file like:

```yaml
records:
  - type: A
    domain: api.example.com
    ip: 192.168.1.10
    ttl: 14400
  - type: CNAME
    domain: www.example.com
    cname: api.example.com
```

If both YAML and labels define the same `type+domain`, the Docker label wins.

## Examples

### Docker Compose with indexed labels

```yaml
services:
  myapp:
    image: myapp:latest
    labels:
      # A record
      unifi.dns.1.domain: "api.example.com"
      unifi.dns.1.type: "A"
      unifi.dns.1.value: "192.168.1.100"
      unifi.dns.1.ttl: "3600"
      # SRV record for monitoring
      unifi.dns.2.domain: "example.com"
      unifi.dns.2.type: "SRV"
      unifi.dns.2.service: "_metrics"
      unifi.dns.2.protocol: "_tcp"
      unifi.dns.2.server: "myapp.local"
      unifi.dns.2.port: "9090"
      unifi.dns.2.priority: "10"
      unifi.dns.2.weight: "100"
```

## Environment variables

- `UNIFI_URL` (required) Example: `https://192.168.123.1`
- `UNIFI_API_TOKEN` (required)
- `UNIFI_SITE_NAME` (required, matches site name or siteName)
- `UNIFI_INSECURE_SKIP_VERIFY` (optional, default false)
- `UNIFI_SYNC_INTERVAL` (optional, duration or seconds, default `60s`)
- `UNIFI_DNS_TTL_SECONDS` (optional, default `14400`)
- `UNIFI_DNS_YAML_PATH` (optional)
- `UNIFI_DNS_PRUNE` (optional, default false; deletes policies not in desired set)
- `UNIFI_TXT_PREFIX` (optional; enables TXT registry records when set)

## TXT ownership tracking

When `UNIFI_TXT_PREFIX` is set, each managed A and CNAME record gets a companion TXT record (SRV records are excluded due to underscore characters in their domains):

- TXT entry: `<prefix>.<domain>`
- TXT value: `"heritage=unifi-external-dns,unifi-external-dns/owner=<prefix>,unifi-external-dns/resource=<resource>"`

For SRV records, underscores are removed from the TXT companion domain to ensure DNS compatibility (e.g., `_service._tcp.domain.com` → `<prefix>.servicetcp.domain.com`).

Examples:

- Record: `cname-example.domain` (from container `web`)
- TXT: `<prefix>.cname-example.domain`
- Value: `"heritage=unifi-external-dns,unifi-external-dns/owner=<prefix>,unifi-external-dns/resource=docker/web"`

## Build

```bash
docker build -t unifi-external-dns .
```

## Run

```bash
docker run --rm \
  -e UNIFI_URL=https://192.168.123.1 \
  -e UNIFI_API_TOKEN=YOUR_API_KEY \
  -e UNIFI_SITE_NAME=default \
  -e UNIFI_DNS_YAML_PATH=/config/dns.yaml \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v $PWD/dns.yaml:/config/dns.yaml:ro \
  unifi-external-dns
```

To use the Docker TCP socket instead of the Unix socket, set `DOCKER_HOST=tcp://HOST:2375`.

## Contribution

Feel free to try it, enhance it and open issues if you find bugs or errors

## Support

<a href="https://www.buymeacoffee.com/app/gianmarcomameli">
<img src="https://cdn.simpleicons.org/buymeacoffee" alt="buymeacoffe" height="32" />
</a>
<a href="https://ko-fi.com/gianmarcomameli">
<img src="https://cdn.simpleicons.org/kofi" alt="kofi" height="32"/>
</a>
