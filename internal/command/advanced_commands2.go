package command

func RegisterAdvancedCommands2(router *Router) {

	router.Register(&CommandDef{Name: "VALIDATE.CREATE", Handler: cmdVALIDATECREATE})
	router.Register(&CommandDef{Name: "VALIDATE.DELETE", Handler: cmdVALIDATEDELETE})
	router.Register(&CommandDef{Name: "VALIDATE.CHECK", Handler: cmdVALIDATECHECK})
	router.Register(&CommandDef{Name: "VALIDATE.LIST", Handler: cmdVALIDATELIST})

	router.Register(&CommandDef{Name: "STAGE.CREATE", Handler: cmdSTAGECREATE})
	router.Register(&CommandDef{Name: "STAGE.DELETE", Handler: cmdSTAGEDELETE})
	router.Register(&CommandDef{Name: "STAGE.NEXT", Handler: cmdSTAGENEXT})
	router.Register(&CommandDef{Name: "STAGE.PREV", Handler: cmdSTAGEPREV})
	router.Register(&CommandDef{Name: "STAGE.LIST", Handler: cmdSTAGELIST})

	router.Register(&CommandDef{Name: "CONTEXT.CREATE", Handler: cmdCONTEXTCREATE})
	router.Register(&CommandDef{Name: "CONTEXT.DELETE", Handler: cmdCONTEXTDELETE})
	router.Register(&CommandDef{Name: "CONTEXT.SET", Handler: cmdCONTEXTSET})
	router.Register(&CommandDef{Name: "CONTEXT.GET", Handler: cmdCONTEXTGET})
	router.Register(&CommandDef{Name: "CONTEXT.LIST", Handler: cmdCONTEXTLIST})

	router.Register(&CommandDef{Name: "PERMIT.GRANT", Handler: cmdPERMITGRANT})
	router.Register(&CommandDef{Name: "PERMIT.REVOKE", Handler: cmdPERMITREVOKE})
	router.Register(&CommandDef{Name: "PERMIT.CHECK", Handler: cmdPERMITCHECK})
	router.Register(&CommandDef{Name: "PERMIT.LIST", Handler: cmdPERMITLIST})

	router.Register(&CommandDef{Name: "GRANT.CREATE", Handler: cmdGRANTCREATE})
	router.Register(&CommandDef{Name: "GRANT.DELETE", Handler: cmdGRANTDELETE})
	router.Register(&CommandDef{Name: "GRANT.CHECK", Handler: cmdGRANTCHECK})
	router.Register(&CommandDef{Name: "GRANT.LIST", Handler: cmdGRANTLIST})

	router.Register(&CommandDef{Name: "TIMER.CREATE", Handler: cmdTIMERCREATE})
	router.Register(&CommandDef{Name: "TIMER.DELETE", Handler: cmdTIMERDELETE})
	router.Register(&CommandDef{Name: "TIMER.STATUS", Handler: cmdTIMERSTATUS})
	router.Register(&CommandDef{Name: "TIMER.LIST", Handler: cmdTIMERLIST})

	router.Register(&CommandDef{Name: "COUNTERX2.CREATE", Handler: cmdCOUNTERX2CREATE})
	router.Register(&CommandDef{Name: "COUNTERX2.INCR", Handler: cmdCOUNTERX2INCR})
	router.Register(&CommandDef{Name: "COUNTERX2.DECR", Handler: cmdCOUNTERX2DECR})
	router.Register(&CommandDef{Name: "COUNTERX2.GET", Handler: cmdCOUNTERX2GET})
	router.Register(&CommandDef{Name: "COUNTERX2.LIST", Handler: cmdCOUNTERX2LIST})

	router.Register(&CommandDef{Name: "LEVEL.CREATE", Handler: cmdLEVELCREATE})
	router.Register(&CommandDef{Name: "LEVEL.DELETE", Handler: cmdLEVELDELETE})
	router.Register(&CommandDef{Name: "LEVEL.SET", Handler: cmdLEVELSET})
	router.Register(&CommandDef{Name: "LEVEL.GET", Handler: cmdLEVELGET})
	router.Register(&CommandDef{Name: "LEVEL.LIST", Handler: cmdLEVELLIST})

	router.Register(&CommandDef{Name: "RECORD.CREATE", Handler: cmdRECORDCREATE})
	router.Register(&CommandDef{Name: "RECORD.ADD", Handler: cmdRECORDADD})
	router.Register(&CommandDef{Name: "RECORD.GET", Handler: cmdRECORDGET})
	router.Register(&CommandDef{Name: "RECORD.DELETE", Handler: cmdRECORDDELETE})

	router.Register(&CommandDef{Name: "ENTITY.CREATE", Handler: cmdENTITYCREATE})
	router.Register(&CommandDef{Name: "ENTITY.DELETE", Handler: cmdENTITYDELETE})
	router.Register(&CommandDef{Name: "ENTITY.GET", Handler: cmdENTITYGET})
	router.Register(&CommandDef{Name: "ENTITY.SET", Handler: cmdENTITYSET})
	router.Register(&CommandDef{Name: "ENTITY.LIST", Handler: cmdENTITYLIST})

	router.Register(&CommandDef{Name: "RELATION.CREATE", Handler: cmdRELATIONCREATE})
	router.Register(&CommandDef{Name: "RELATION.DELETE", Handler: cmdRELATIONDELETE})
	router.Register(&CommandDef{Name: "RELATION.GET", Handler: cmdRELATIONGET})
	router.Register(&CommandDef{Name: "RELATION.LIST", Handler: cmdRELATIONLIST})

	router.Register(&CommandDef{Name: "POOLX.CREATE", Handler: cmdPOOLXCREATE})
	router.Register(&CommandDef{Name: "POOLX.DELETE", Handler: cmdPOOLXDELETE})
	router.Register(&CommandDef{Name: "POOLX.ACQUIRE", Handler: cmdPOOLXACQUIRE})
	router.Register(&CommandDef{Name: "POOLX.RELEASE", Handler: cmdPOOLXRELEASE})
	router.Register(&CommandDef{Name: "POOLX.STATUS", Handler: cmdPOOLXSTATUS})

	router.Register(&CommandDef{Name: "BUFFERX.CREATE", Handler: cmdBUFFERXCREATE})
	router.Register(&CommandDef{Name: "BUFFERX.WRITE", Handler: cmdBUFFERXWRITE})
	router.Register(&CommandDef{Name: "BUFFERX.READ", Handler: cmdBUFFERXREAD})
	router.Register(&CommandDef{Name: "BUFFERX.DELETE", Handler: cmdBUFFERXDELETE})

	router.Register(&CommandDef{Name: "STREAMX.CREATE", Handler: cmdSTREAMXCREATE})
	router.Register(&CommandDef{Name: "STREAMX.WRITE", Handler: cmdSTREAMXWRITE})
	router.Register(&CommandDef{Name: "STREAMX.READ", Handler: cmdSTREAMXREAD})
	router.Register(&CommandDef{Name: "STREAMX.DELETE", Handler: cmdSTREAMXDELETE})

	router.Register(&CommandDef{Name: "CACHEX.CREATE", Handler: cmdCACHEXCREATE})
	router.Register(&CommandDef{Name: "CACHEX.DELETE", Handler: cmdCACHEXDELETE})
	router.Register(&CommandDef{Name: "CACHEX.GET", Handler: cmdCACHEXGET})
	router.Register(&CommandDef{Name: "CACHEX.SET", Handler: cmdCACHEXSET})
	router.Register(&CommandDef{Name: "CACHEX.LIST", Handler: cmdCACHEXLIST})

	router.Register(&CommandDef{Name: "STOREX.CREATE", Handler: cmdSTOREXCREATE})
	router.Register(&CommandDef{Name: "STOREX.DELETE", Handler: cmdSTOREXDELETE})
	router.Register(&CommandDef{Name: "STOREX.PUT", Handler: cmdSTOREXPUT})
	router.Register(&CommandDef{Name: "STOREX.GET", Handler: cmdSTOREXGET})
	router.Register(&CommandDef{Name: "STOREX.LIST", Handler: cmdSTOREXLIST})

	router.Register(&CommandDef{Name: "INDEX.CREATE", Handler: cmdINDEXCREATE})
	router.Register(&CommandDef{Name: "INDEX.DELETE", Handler: cmdINDEXDELETE})
	router.Register(&CommandDef{Name: "INDEX.ADD", Handler: cmdINDEXADD})
	router.Register(&CommandDef{Name: "INDEX.SEARCH", Handler: cmdINDEXSEARCH})
	router.Register(&CommandDef{Name: "INDEX.LIST", Handler: cmdINDEXLIST})

	router.Register(&CommandDef{Name: "VIEW.CREATE", Handler: cmdVIEWCREATE})
	router.Register(&CommandDef{Name: "VIEW.DELETE", Handler: cmdVIEWDELETE})
	router.Register(&CommandDef{Name: "VIEW.GET", Handler: cmdVIEWGET})
	router.Register(&CommandDef{Name: "VIEW.LIST", Handler: cmdVIEWLIST})

	router.Register(&CommandDef{Name: "AUDITX.LOG", Handler: cmdAUDITXLOG})
	router.Register(&CommandDef{Name: "AUDITX.GET", Handler: cmdAUDITXGET})
	router.Register(&CommandDef{Name: "AUDITX.SEARCH", Handler: cmdAUDITXSEARCH})
	router.Register(&CommandDef{Name: "AUDITX.LIST", Handler: cmdAUDITXLIST})

	router.Register(&CommandDef{Name: "TOKEN.CREATE", Handler: cmdTOKENCREATE})
	router.Register(&CommandDef{Name: "TOKEN.DELETE", Handler: cmdTOKENDELETE})
	router.Register(&CommandDef{Name: "TOKEN.VALIDATE", Handler: cmdTOKENVALIDATE})
	router.Register(&CommandDef{Name: "TOKEN.REFRESH", Handler: cmdTOKENREFRESH})
	router.Register(&CommandDef{Name: "TOKEN.LIST", Handler: cmdTOKENLIST})

	router.Register(&CommandDef{Name: "SESSIONX.CREATE", Handler: cmdSESSIONXCREATE})
	router.Register(&CommandDef{Name: "SESSIONX.DELETE", Handler: cmdSESSIONXDELETE})
	router.Register(&CommandDef{Name: "SESSIONX.GET", Handler: cmdSESSIONXGET})
	router.Register(&CommandDef{Name: "SESSIONX.SET", Handler: cmdSESSIONXSET})
	router.Register(&CommandDef{Name: "SESSIONX.LIST", Handler: cmdSESSIONXLIST})

	router.Register(&CommandDef{Name: "PROFILE.CREATE", Handler: cmdPROFILECREATE})
	router.Register(&CommandDef{Name: "PROFILE.DELETE", Handler: cmdPROFILEDELETE})
	router.Register(&CommandDef{Name: "PROFILE.GET", Handler: cmdPROFILEGET})
	router.Register(&CommandDef{Name: "PROFILE.SET", Handler: cmdPROFILESET})
	router.Register(&CommandDef{Name: "PROFILE.LIST", Handler: cmdPROFILELIST})

}
