# Secure Access Configuration

## Overview

This Pocketbase instance is configured with IP-based access control that restricts access to:
1. **Fly.io Private Network (6PN)** - Other Fly.io apps in the organization
2. **Authorized Home IP** - A specific IP address configured via Fly.io secrets

All other access attempts are blocked with a 403 Forbidden response.

## Architecture

### Application-Level IP Filtering

The solution uses a custom Pocketbase build with Go middleware that filters requests based on source IP address:

- **Custom Go Application** (`main.go`) wraps the standard Pocketbase binary
- **Middleware Hook** intercepts requests before they reach Pocketbase
- **IP Validation** checks the client IP against allowlists
- **Logging** records denied access attempts for monitoring

### How It Works

```
┌─────────────────┐
│   Fly.io Proxy  │
│  (SSL/HTTPS)    │
└────────┬────────┘
         │ Adds Fly-Client-IP header
         ▼
┌─────────────────┐
│  IP Middleware  │
│                 │
│  Check IP:      │
│  1. 6PN?        │─── Yes ──┐
│  2. Home IP?    │─── Yes ──┤
│  3. Private?    │─── Yes ──┤
│                 │           │
│     No?         │           │
│     ↓           │           │
│  403 Forbidden  │           │
└─────────────────┘           │
                              ▼
                    ┌──────────────────┐
                    │   Pocketbase     │
                    │   Application    │
                    └──────────────────┘
```

## Implementation Details

### Files Created

#### `main.go`
Custom Pocketbase application with IP filtering middleware:
- Hooks into Pocketbase's `OnServe()` event
- Adds request filtering before routing
- Checks client IP via `RealIP()` (respects proxy headers)
- Allows Fly.io 6PN range: `fdaa::/48`
- Allows configured home IP from `ALLOWED_HOME_IP` environment variable
- Logs denied access attempts
- Returns 403 Forbidden for unauthorized IPs

#### `go.mod`
Go module definition:
```go
module github.com/geoffjay/admin
go 1.23
require github.com/pocketbase/pocketbase v0.35.0
```

### Files Modified

#### `Dockerfile`
Changed from prebuilt binary download to multi-stage Go build:

**Build Stage:**
- Uses `golang:1.23-alpine` image
- Installs build dependencies (git, ca-certificates)
- Copies Go module files and downloads dependencies
- Builds custom Pocketbase binary with IP filtering

**Runtime Stage:**
- Uses minimal `alpine:latest` image
- Copies only the compiled binary
- Results in small, efficient container

#### `fly.toml`
Added environment configuration:
```toml
[env]
  PB_TRUSTED_PROXY = "true"
```

This tells Pocketbase to trust the `Fly-Client-IP` header from Fly's proxy, ensuring accurate client IP detection.

#### `docker-compose.yml`
Added environment variable for local testing:
```yaml
environment:
  - ALLOWED_HOME_IP=127.0.0.1
```

Allows localhost connections when testing locally with Docker Compose.

## Deployment

### Prerequisites

1. Fly.io CLI installed and authenticated
2. Your current home IP address
3. Go 1.24+ (required - Pocketbase v0.35.0 requires Go 1.24.0+)
4. Docker Desktop (optional, for local testing)

### Step 1: Configure Home IP Secret

Before deploying, set your authorized home IP address as a Fly.io secret:

```bash
# Find your home IP
curl ifconfig.me

# Set the secret (replace with your actual IP)
flyctl secrets set ALLOWED_HOME_IP="203.0.113.42" --app com-geoffjay-admin
```

**Using CIDR Notation:**

You can specify an IP range instead of a single IP:

```bash
# Allow entire /24 subnet
flyctl secrets set ALLOWED_HOME_IP="203.0.113.0/24" --app com-geoffjay-admin

# Allow single IP with /32 notation
flyctl secrets set ALLOWED_HOME_IP="203.0.113.42/32" --app com-geoffjay-admin
```

### Step 2: Deploy Application

Deploy the updated application to Fly.io:

```bash
flyctl deploy --app com-geoffjay-admin
```

The build process will:
1. Compile the custom Pocketbase application
2. Create a multi-stage Docker image
3. Deploy to your Fly.io app
4. Start the service with IP filtering enabled

### Step 3: Verify Deployment

Check deployment status:

```bash
flyctl status --app com-geoffjay-admin
```

View application logs:

```bash
flyctl logs --app com-geoffjay-admin
```

## Testing

### Test from Home IP

From your home network, access the Pocketbase API:

```bash
curl https://com-geoffjay-admin.fly.dev/api/health
```

**Expected:** Successful response (200 OK)

### Test from Unauthorized IP

From a different network (mobile data, VPN, etc.):

```bash
curl https://com-geoffjay-admin.fly.dev/api/health
```

**Expected:** Access denied (403 Forbidden)

```json
{
  "code": 403,
  "message": "Access denied from your IP address",
  "data": {}
}
```

### Test from Fly.io Private Network (6PN)

Deploy a test application in the same Fly.io organization:

```bash
# From another Fly.io app in the same org
curl http://com-geoffjay-admin.internal:8090/api/health
```

**Expected:** Successful response (200 OK)

The `.internal` DNS name routes through Fly.io's private IPv6 network (6PN), which is always allowed.

### Local Testing with Docker Compose

Test the build locally before deploying:

```bash
# Build the image
docker compose build

# Start the service
docker compose up -d

# Test access
curl http://localhost:8090/api/health
```

**Note:** The `docker-compose.yml` file sets `ALLOWED_HOME_IP=0.0.0.0/0` for local testing, which allows all IPs. This is because Docker's bridge networking means the container sees requests from the Docker gateway IP, not `127.0.0.1`. In production on Fly.io, you'll set your actual home IP via secrets.

## Operations

### Updating Home IP Address

When your home IP changes (dynamic IP, new ISP, etc.):

```bash
flyctl secrets set ALLOWED_HOME_IP="NEW_IP_ADDRESS" --app com-geoffjay-admin
```

**Important Notes:**
- Secret updates trigger a machine restart (approximately 30 seconds downtime)
- No redeployment needed - change takes effect immediately after restart
- Old IP is blocked as soon as the machine restarts with new secret

### Viewing Current Secrets

List configured secrets (values are hidden):

```bash
flyctl secrets list --app com-geoffjay-admin
```

### Monitoring Access Attempts

View real-time logs to monitor denied access attempts:

```bash
# Stream logs
flyctl logs --app com-geoffjay-admin

# Look for denied access messages
flyctl logs --app com-geoffjay-admin | grep "Access denied"
```

Denied access attempts are logged with:
- Source IP address
- Requested path
- Timestamp

Example log entry:
```
Access denied from IP: 198.51.100.42, Path: /api/collections
```

### Emergency Access Recovery

If you're locked out due to incorrect IP configuration:

**Option 1: Temporarily Allow All IPs**

```bash
# Remove the IP restriction (allows all IPs until reconfigured)
flyctl secrets unset ALLOWED_HOME_IP --app com-geoffjay-admin
```

**Option 2: Use Fly SSH Console**

Access the machine directly via Fly's SSH:

```bash
flyctl ssh console --app com-geoffjay-admin
```

From the console, you can check environment variables and troubleshoot.

**Option 3: Set a Wide CIDR Range**

```bash
# Temporarily allow entire IPv4 space (use with caution!)
flyctl secrets set ALLOWED_HOME_IP="0.0.0.0/0" --app com-geoffjay-admin
```

### Maintenance: Updating Pocketbase Version

When a new Pocketbase version is released:

1. Update `go.mod`:
   ```go
   require github.com/pocketbase/pocketbase v0.36.0  // New version
   ```

2. Test locally:
   ```bash
   docker compose build
   docker compose up
   ```

3. Deploy:
   ```bash
   flyctl deploy --app com-geoffjay-admin
   ```

## Security Considerations

### Defense in Depth

IP filtering is **one layer** of security, not the only layer:

✓ **Still use Pocketbase authentication** - Admin passwords, API tokens, collection rules
✓ **Enable rate limiting** - Pocketbase's built-in rate limiting for API endpoints
✓ **Use HTTPS** - Fly.io handles SSL/TLS termination (already configured)
✓ **Monitor logs** - Watch for suspicious access patterns
✓ **Keep software updated** - Regularly update Pocketbase and dependencies

### IP Filtering Limitations

**What IP filtering protects against:**
- Unauthorized public internet access
- Random scanning and bots
- Accidental public exposure

**What IP filtering does NOT protect against:**
- Compromised credentials (still need strong passwords)
- Attacks from allowed IPs (6PN or home IP)
- Application vulnerabilities (XSS, SQL injection, etc.)

### Best Practices

1. **Strong Admin Passwords**
   - Use a password manager
   - Enable 2FA if available in Pocketbase
   - Rotate credentials regularly

2. **IP Management**
   - Document your home IP in a secure location
   - Use CIDR /32 for single IP (more explicit)
   - Consider dynamic DNS if home IP changes frequently

3. **Secret Management**
   - Never commit `ALLOWED_HOME_IP` to version control
   - Use Fly.io secrets for sensitive configuration
   - Limit access to Fly.io account/organization

4. **Monitoring**
   - Set up Fly.io metrics and alerts
   - Monitor for repeated 403 errors (potential attacks)
   - Review logs regularly

5. **Backup Access**
   - Keep alternative access method (Fly SSH)
   - Document emergency procedures
   - Test recovery procedures periodically

### Network Architecture Notes

**Public IPs Maintained:**
- The app retains public IPv4 and IPv6 addresses
- Required for home IP access from public internet
- Fly.io proxy handles SSL termination and DDoS protection

**6PN (Private Network):**
- Uses Fly.io's IPv6 private network (`fdaa::/48`)
- Only accessible to apps in your organization
- No public internet routing
- Use `.internal` DNS for 6PN access

**Hybrid Access Model:**
- Combines private network benefits with selective public access
- More flexible than VPN-only or fully public
- Trade-off: Application-level filtering vs. network-level isolation

## Troubleshooting

### Problem: 403 Forbidden from Home IP

**Possible causes:**
1. Home IP changed (dynamic IP from ISP)
2. `ALLOWED_HOME_IP` secret not set or incorrect
3. Accessing via VPN or proxy (different IP)

**Solutions:**
```bash
# Check your current IP
curl ifconfig.me

# Update secret with current IP
flyctl secrets set ALLOWED_HOME_IP="YOUR_CURRENT_IP" --app com-geoffjay-admin

# Verify secret is set
flyctl secrets list --app com-geoffjay-admin
```

### Problem: 6PN Access Not Working

**Possible causes:**
1. Using wrong DNS name (should use `.internal`)
2. Apps in different organizations
3. Network connectivity issues

**Solutions:**
```bash
# Verify app organization
flyctl apps list --org YOUR_ORG

# Use correct internal DNS
curl http://com-geoffjay-admin.internal:8090/api/health

# Check if app is running
flyctl status --app com-geoffjay-admin
```

### Problem: Build Fails on Deployment

**Possible causes:**
1. Go module download issues
2. Network connectivity during build
3. Invalid Go syntax

**Solutions:**
```bash
# Test build locally
docker compose build

# Check Go module validity
go mod verify

# View detailed build logs
flyctl deploy --app com-geoffjay-admin --verbose
```

### Problem: Application Not Starting

**Possible causes:**
1. Invalid `ALLOWED_HOME_IP` format
2. Pocketbase data migration issues
3. Port binding conflicts

**Solutions:**
```bash
# Check application logs
flyctl logs --app com-geoffjay-admin

# Verify secret format (should be IP or CIDR)
flyctl secrets list --app com-geoffjay-admin

# SSH into machine for debugging
flyctl ssh console --app com-geoffjay-admin
```

## Advanced Configuration

### Multiple Authorized IPs

To allow multiple home IPs (e.g., home and office), modify `main.go`:

```go
func isAllowedIP(e *core.RequestEvent) bool {
    clientIP := e.RealIP()

    if isPrivateNetwork(clientIP) {
        return true
    }

    // Support comma-separated list
    allowedIPs := strings.Split(os.Getenv("ALLOWED_HOME_IP"), ",")
    for _, allowedIP := range allowedIPs {
        allowedIP = strings.TrimSpace(allowedIP)
        if strings.Contains(allowedIP, "/") {
            // CIDR notation
            _, allowedNet, err := net.ParseCIDR(allowedIP)
            if err == nil {
                ip := net.ParseIP(clientIP)
                if allowedNet.Contains(ip) {
                    return true
                }
            }
        } else {
            // Direct match
            if clientIP == allowedIP {
                return true
            }
        }
    }

    return false
}
```

Then set multiple IPs:
```bash
flyctl secrets set ALLOWED_HOME_IP="203.0.113.42,198.51.100.10" --app com-geoffjay-admin
```

### Time-Based Access

Add time-based restrictions to the middleware:

```go
import "time"

func isAllowedIP(e *core.RequestEvent) bool {
    clientIP := e.RealIP()

    // Always allow 6PN
    if isPrivateNetwork(clientIP) {
        return true
    }

    // Check business hours (9 AM - 5 PM UTC)
    now := time.Now().UTC()
    if now.Hour() < 9 || now.Hour() >= 17 {
        log.Printf("Access denied outside business hours from IP: %s", clientIP)
        return false
    }

    // Continue with IP checks...
    return checkHomeIP(clientIP)
}
```

### Geographic Restrictions

Integrate with GeoIP database for country-level blocking:

```go
import "github.com/oschwald/geoip2-golang"

func isAllowedIP(e *core.RequestEvent) bool {
    clientIP := e.RealIP()

    if isPrivateNetwork(clientIP) {
        return true
    }

    // Check geographic location
    db, err := geoip2.Open("/path/to/GeoLite2-Country.mmdb")
    if err == nil {
        defer db.Close()

        ip := net.ParseIP(clientIP)
        record, err := db.Country(ip)
        if err == nil {
            // Only allow US and Canada
            allowedCountries := []string{"US", "CA"}
            for _, country := range allowedCountries {
                if record.Country.IsoCode == country {
                    return checkHomeIP(clientIP)
                }
            }
            return false
        }
    }

    return checkHomeIP(clientIP)
}
```

## Migration to Fully Private

If you later decide to make the app fully private (6PN only):

1. Remove public IP addresses:
   ```bash
   flyctl ips list --app com-geoffjay-admin
   flyctl ips release <IP_ADDRESS> --app com-geoffjay-admin
   ```

2. Use Flycast for private networking:
   ```bash
   flyctl ips allocate-v6 --private --app com-geoffjay-admin
   ```

3. Update `fly.toml` to disable public service:
   ```toml
   [[services]]
     internal_port = 8090
     protocol = "tcp"

   [[services.ports]]
     port = 8090
   ```

4. Access via WireGuard VPN for personal access:
   ```bash
   flyctl wireguard create
   ```

## References

- [Fly.io Private Networking (6PN)](https://fly.io/docs/networking/private-networking/)
- [Pocketbase Documentation](https://pocketbase.io/docs/)
- [Fly.io Secrets Management](https://fly.io/docs/reference/secrets/)
- [Go net Package](https://pkg.go.dev/net)

## Support

For issues or questions:
1. Check application logs: `flyctl logs --app com-geoffjay-admin`
2. Review this documentation
3. Consult [Fly.io Community Forum](https://community.fly.io)
4. Check [Pocketbase GitHub Discussions](https://github.com/pocketbase/pocketbase/discussions)
