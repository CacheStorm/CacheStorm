package command

func RegisterWorkflowCommands(router *Router) {
	router.Register(&CommandDef{Name: "STATEM.CREATE", Handler: cmdSTATEMCREATE})
	router.Register(&CommandDef{Name: "STATEM.DELETE", Handler: cmdSTATEMDELETE})
	router.Register(&CommandDef{Name: "STATEM.ADDSTATE", Handler: cmdSTATEMADDSTATE})
	router.Register(&CommandDef{Name: "STATEM.ADDTRANS", Handler: cmdSTATEMADDTRANS})
	router.Register(&CommandDef{Name: "STATEM.TRIGGER", Handler: cmdSTATEMTRIGGER})
	router.Register(&CommandDef{Name: "STATEM.CURRENT", Handler: cmdSTATEMCURRENT})
	router.Register(&CommandDef{Name: "STATEM.CANTRIGGER", Handler: cmdSTATEMCANTRIGGER})
	router.Register(&CommandDef{Name: "STATEM.EVENTS", Handler: cmdSTATEMEVENTS})
	router.Register(&CommandDef{Name: "STATEM.RESET", Handler: cmdSTATEMRESET})
	router.Register(&CommandDef{Name: "STATEM.ISFINAL", Handler: cmdSTATEMISFINAL})
	router.Register(&CommandDef{Name: "STATEM.INFO", Handler: cmdSTATEMINFO})
	router.Register(&CommandDef{Name: "STATEM.LIST", Handler: cmdSTATEMLIST})

	router.Register(&CommandDef{Name: "CHAINED.SET", Handler: cmdCHAINEDSET})
	router.Register(&CommandDef{Name: "CHAINED.GET", Handler: cmdCHAINEDGET})
	router.Register(&CommandDef{Name: "CHAINED.DEL", Handler: cmdCHAINEDDEL})
}
