package main

import (
	lazygate "github.com/kasefuchs/lazygate/pkg/plugin"
	hostredirect "github.com/luckcrafter/hostredirect"
	"go.minekube.com/gate/cmd/gate"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

func main() {
	proxy.Plugins = append(proxy.Plugins, lazygate.NewProxyPlugin(), hostredirect.Plugin)

	gate.Execute()
}
