package hostredirect

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/robinbraemer/event"

	"go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/edition/java/lite"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

// Plugin is a demo plugin that redirects players based on the host they connect with.
var Plugin = proxy.Plugin{
	Name: "HostRedirect",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		// Get the logger for this plugin.
		log := logr.FromContextOrDiscard(ctx)
		basedomain := os.Getenv("HOSTDOMAIN")

		log.Info("HostRedirect Initlize...")
		event.Subscribe(p.Event(), 0, onPlayerChooseInitialServer(p, log, basedomain))

		log.Info("HostRedirect Initlized")
		return nil
	},
}

// onPlayerChooseInitialServer handles the PlayerChooseInitialServerEvent to redirect players.
func onPlayerChooseInitialServer(p *proxy.Proxy, log logr.Logger, basedomain string) func(*proxy.PlayerChooseInitialServerEvent) {
	return func(e *proxy.PlayerChooseInitialServerEvent) {
		// Get the player's connecting host.
		conn := e.Player().VirtualHost()
		host := conn.String()
		host = lite.ClearVirtualHost(host)

		serverDomain, _, _ := strings.Cut(host, "."+basedomain)

		server := p.Server(serverDomain)
		if server == nil {
			msg := fmt.Sprintf("Server not found: %s", server)
			log.Info(msg)
			e.Player().Disconnect(&component.Text{Content: msg})
			return
		}

		// Redirect the player to the appropriate server.
		e.SetInitialServer(server)
		log.Info("Redirecting player", "username", e.Player().Username(), "server", server)
	}
}
