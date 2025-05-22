package main

import (
	"github.com/luckcrafter/hostredirect"
	"go.minekube.com/gate/cmd/gate"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

func main() {
	proxy.Plugins = append(proxy.Plugins, hostredirect.Plugin)

	gate.Execute()
}
