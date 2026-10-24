package command

// RegisterNamespaceCommands registers SELECT, which switches the connection
// to another numbered database (see db_commands.go). The dead multi-namespace
// surface (NAMESPACE, NAMESPACES, NAMESPACEDEL, NAMESPACEINFO) was removed.
func RegisterNamespaceCommands(router *Router) {
	router.Register(&CommandDef{Name: "SELECT", Handler: cmdSELECT})
}
