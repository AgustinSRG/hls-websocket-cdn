// Utils to get the external websocket connection URL
// to store it in the publish registry

package main

import (
	"fmt"
	"net"

	"github.com/AgustinSRG/genv"
)

// Figures out the external websocket URL of the server
// It must be used when publishing to register the server in the publish registry database
func FigureOutExternalServerWebsocketUrl() string {
	customExternalWebsocketUrl := genv.GetEnvString("EXTERNAL_WEBSOCKET_URL", "")

	if customExternalWebsocketUrl != "" {
		return customExternalWebsocketUrl
	}

	isSecure := genv.GetEnvBool("TLS_ENABLED", false)

	var proto string

	if isSecure {
		proto = "wss"
	} else {
		proto = "ws"
	}

	port := genv.GetEnvInt("HTTP_PORT", 80)

	if isSecure {
		port = genv.GetEnvInt("TLS_PORT", 443)
	}

	prefix := genv.GetEnvString("WEBSOCKET_PREFIX", "/")

	networkInterfaces, err := net.Interfaces()

	if err != nil {
		LogError(err, "Error loading network interfaces")
		return ""
	}

	// Check network interfaces

	_, badRange, err := net.ParseCIDR("169.254.0.0/16")

	if err != nil {
		LogError(err, "Error loading network interfaces")
		return ""
	}

	for _, i := range networkInterfaces {
		addrs, err := i.Addrs()

		if err != nil {
			LogError(err, "Error loading addresses from network interface")
			continue
		}

		// handle err
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			default:
				continue
			}

			if ip.IsLoopback() || ip.IsMulticast() || badRange.Contains(ip) {
				continue
			}

			if ip.To4() != nil {
				return proto + "://" + ip.To4().String() + ":" + fmt.Sprint(port) + prefix
			}
		}
	}

	// Default

	return ""
}
