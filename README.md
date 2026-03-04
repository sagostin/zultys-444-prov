# Zultys Provisioning Proxy

This application acts as an intermediate proxy to resolve certificate and provisioning path issues for Zultys IP phones. It listens on a custom port (default 444), rewrites the legacy provisioning path, and forwards the request to the PBX's standard HTTPS port (443).

## Features

- **Port Redirection**: Accepts traffic on port 444 (or custom) and forwards to port 443.
- **Dynamic Host Targeting**: Automatically detects the requested Host header (e.g., `customer.zultys.example.com:444`) and targets the *same host* on port 443 (`customer.zultys.example.com:443`). This supports multi-tenant deployments.
- **Path Rewriting**: Automatically rewrites requests from `/httpsphone2/` to `/httpsphone/`.
- **Header Preservation**: Forwards critical headers like `User-Agent` to the PBX.
- **TLS Termination**: Handles HTTPS connections from phones using a provided certificate.

## Prerequisites

- **Go**: 1.18 or later (to build).
- **SSL Certificate**: A valid (or self-signed, depending on phone trust store) certificate pair (`server.crt`, `server.key`) for the server hostname.

## Certificate Setup

The proxy just needs *any* TLS certificate to complete the HTTPS handshake with the phones — the CN/hostname doesn't need to match since this is a multi-tenant proxy serving requests for multiple PBX servers. The phones don't strictly validate the certificate.

Generate a generic self-signed cert valid for 10 years:

```bash
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout server.key -out server.crt \
  -days 3650 \
  -subj "/CN=zultys-provisioning-proxy"
```

This produces `server.crt` and `server.key` in the current directory — the default paths the proxy expects.

Verify it was created:

```bash
openssl x509 -in server.crt -noout -text | head -20
```

## Docker

### 1. Generate Certs

```bash
mkdir -p certs
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout certs/server.key -out certs/server.crt \
  -days 3650 \
  -subj "/CN=zultys-provisioning-proxy"
```

### 2. Build & Run

```bash
docker compose up -d --build
```

The container will listen on port 444 and auto-restart on failure. Certs are mounted read-only from `./certs/`.

### 3. Logs

```bash
docker compose logs -f
```

## Build (Native)

```bash
go build -o provisioning-proxy
```

## Usage

Run the binary on your server. You may need `sudo` to bind to privileged ports (though 444 is usually non-privileged).

```bash
./provisioning-proxy [flags]
```

### Flags

- `-listen`: Address/Port to listen on (default `":444"`).
- `-cert`: Path to SSL certificate file (default `"server.crt"`).
- `-key`: Path to SSL key file (default `"server.key"`).
- `-insecure`: Skip upstream TLS verification (default `false`). Useful if the PBX itself has an expired or self-signed cert effectively ignored by the proxy.

### Example

```bash
./provisioning-proxy -listen :8443 -cert mycert.pem -key mykey.pem
```

## Deployment Architecture

1.  **Server**: Deploy this binary on a server accessible to your phones.
2.  **DNS/Routing**: Ensure that traffic destined for the PBX on port 444 is routed to this server's IP.
    *   *Option A (DNS)*: Change the A record for the PBX hostname to point to this proxy server (if ports 443/80 are also handled or passed through).
    *   *Option B (NAT/Firewall)*: Use a firewall rule to Destination NAT (DNAT) traffic for `original-pbx-ip:444` to `proxy-server-ip:444`.
3.  **Firewall**: Open the listening port (e.g., 444) on the proxy server's firewall.

## Logic Flow

1.  **Phone Request**: `GET https://customer.host.ca:444/httpsphone2/<remainder>`
2.  **Proxy Intercept**:
    *   Reads Host: `customer.host.ca:444`
    *   Rewrites Host Target: `customer.host.ca:443`
    *   Rewrites Path: `/httpsphone2/` → `/httpsphone/` (the remainder is passed through as-is)
3.  **Upstream Forward**: `GET https://customer.host.ca:443/httpsphone/<remainder>`
4.  **Response**: Streams configuration back to phone.
