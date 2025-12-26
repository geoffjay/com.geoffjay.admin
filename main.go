package main

import (
	"log"
	"net"
	"os"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

func main() {
	app := pocketbase.New()

	// IP allowlist middleware
	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Func: func(e *core.ServeEvent) error {
			// Add middleware to router
			e.Router.BindFunc(func(re *core.RequestEvent) error {
				if !isAllowedIP(re) {
					log.Printf("Access denied from IP: %s, Path: %s", re.RealIP(), re.Request.URL.Path)
					return re.ForbiddenError("Access denied from your IP address", nil)
				}
				return re.Next()
			})
			return e.Next()
		},
		Priority: 1, // Execute early in the chain
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func isAllowedIP(e *core.RequestEvent) bool {
	// Get client IP - RealIP() respects proxy headers
	clientIP := e.RealIP()

	// Always allow private network (6PN) traffic
	// Fly.io 6PN uses fdaa::/48 IPv6 range
	if isPrivateNetwork(clientIP) {
		return true
	}

	// Check against configured home IP
	allowedHomeIP := os.Getenv("ALLOWED_HOME_IP")
	if allowedHomeIP == "" {
		// If not set, only allow private network
		return false
	}

	// Support CIDR notation for home IP
	if strings.Contains(allowedHomeIP, "/") {
		_, allowedNet, err := net.ParseCIDR(allowedHomeIP)
		if err != nil {
			log.Printf("Invalid ALLOWED_HOME_IP CIDR: %v", err)
			return false
		}
		ip := net.ParseIP(clientIP)
		return allowedNet.Contains(ip)
	}

	// Direct IP match
	return clientIP == allowedHomeIP
}

func isPrivateNetwork(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Check if IPv6 Fly.io 6PN range (fdaa::/48)
	_, flyNet, _ := net.ParseCIDR("fdaa::/48")
	if flyNet.Contains(parsedIP) {
		return true
	}

	// Check standard private ranges
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"fc00::/7", // IPv6 ULA
	}

	for _, cidr := range privateRanges {
		_, privNet, _ := net.ParseCIDR(cidr)
		if privNet.Contains(parsedIP) {
			return true
		}
	}

	return false
}
