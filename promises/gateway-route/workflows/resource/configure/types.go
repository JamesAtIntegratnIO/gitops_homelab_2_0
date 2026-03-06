package main

// GatewayRouteConfig holds the resolved configuration from the CR.
type GatewayRouteConfig struct {
	Name             string
	Namespace        string
	Hostname         string
	Path             string
	BackendName      string
	BackendPort      int
	GatewayName      string
	GatewayNS        string
	HTTPRedirect      bool
	OwnerPromise      string
	PromiseName       string
	HTTPSSectionName  string
	HTTPSectionName   string
}
