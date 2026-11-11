package command

func RegisterSchedulerCommands(router *Router) {

	router.Register(&CommandDef{Name: "CIRCUIT.CREATE", Handler: cmdCIRCUITCREATE})
	router.Register(&CommandDef{Name: "CIRCUIT.DELETE", Handler: cmdCIRCUITDELETE})
	router.Register(&CommandDef{Name: "CIRCUIT.ALLOW", Handler: cmdCIRCUITALLOW})
	router.Register(&CommandDef{Name: "CIRCUIT.SUCCESS", Handler: cmdCIRCUITSUCCESS})
	router.Register(&CommandDef{Name: "CIRCUIT.FAILURE", Handler: cmdCIRCUITFAILURE})
	router.Register(&CommandDef{Name: "CIRCUIT.STATE", Handler: cmdCIRCUITSTATE})
	router.Register(&CommandDef{Name: "CIRCUIT.RESET", Handler: cmdCIRCUITRESET})
	router.Register(&CommandDef{Name: "CIRCUIT.STATS", Handler: cmdCIRCUITSTATS})
	router.Register(&CommandDef{Name: "CIRCUIT.LIST", Handler: cmdCIRCUITLIST})

	router.Register(&CommandDef{Name: "SESSION.CREATE", Handler: cmdSESSIONCREATE})
	router.Register(&CommandDef{Name: "SESSION.GET", Handler: cmdSESSIONGET})
	router.Register(&CommandDef{Name: "SESSION.SET", Handler: cmdSESSIONSET})
	router.Register(&CommandDef{Name: "SESSION.DEL", Handler: cmdSESSIONDEL})
	router.Register(&CommandDef{Name: "SESSION.DELETE", Handler: cmdSESSIONDELETE})
	router.Register(&CommandDef{Name: "SESSION.EXISTS", Handler: cmdSESSIONEXISTS})
	router.Register(&CommandDef{Name: "SESSION.TTL", Handler: cmdSESSIONTTL})
	router.Register(&CommandDef{Name: "SESSION.REFRESH", Handler: cmdSESSIONREFRESH})
	router.Register(&CommandDef{Name: "SESSION.CLEAR", Handler: cmdSESSIONCLEAR})
	router.Register(&CommandDef{Name: "SESSION.ALL", Handler: cmdSESSIONALL})
	router.Register(&CommandDef{Name: "SESSION.LIST", Handler: cmdSESSIONLIST})
	router.Register(&CommandDef{Name: "SESSION.COUNT", Handler: cmdSESSIONCOUNT})
	router.Register(&CommandDef{Name: "SESSION.CLEANUP", Handler: cmdSESSIONCLEANUP})
}
