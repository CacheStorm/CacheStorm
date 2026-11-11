package command

func RegisterExtendedCommands(router *Router) {
	router.Register(&CommandDef{Name: "MSGQUEUE.CREATE", Handler: cmdMSGQUEUECREATE})
	router.Register(&CommandDef{Name: "MSGQUEUE.PUBLISH", Handler: cmdMSGQUEUEPUBLISH})
	router.Register(&CommandDef{Name: "MSGQUEUE.CONSUME", Handler: cmdMSGQUEUECONSUME})
	router.Register(&CommandDef{Name: "MSGQUEUE.ACK", Handler: cmdMSGQUEUEACK})
	router.Register(&CommandDef{Name: "MSGQUEUE.NACK", Handler: cmdMSGQUEUENACK})
	router.Register(&CommandDef{Name: "MSGQUEUE.DEADLETTER", Handler: cmdMSGQUEUEDEADLETTER})
	router.Register(&CommandDef{Name: "MSGQUEUE.REQUEUE", Handler: cmdMSGQUEUEREQUEUE})
	router.Register(&CommandDef{Name: "MSGQUEUE.PURGE", Handler: cmdMSGQUEUEPURGE})
	router.Register(&CommandDef{Name: "MSGQUEUE.STATS", Handler: cmdMSGQUEUESTATS})
	router.Register(&CommandDef{Name: "MSGQUEUE.DELETE", Handler: cmdMSGQUEUEDELETE})

	router.Register(&CommandDef{Name: "SERVICE.REGISTER", Handler: cmdSERVICEREGISTER})
	router.Register(&CommandDef{Name: "SERVICE.DEREGISTER", Handler: cmdSERVICEDEREGISTER})
	router.Register(&CommandDef{Name: "SERVICE.DISCOVER", Handler: cmdSERVICEDISCOVER})
	router.Register(&CommandDef{Name: "SERVICE.HEARTBEAT", Handler: cmdSERVICEHEARTBEAT})
	router.Register(&CommandDef{Name: "SERVICE.LIST", Handler: cmdSERVICELIST})
	router.Register(&CommandDef{Name: "SERVICE.HEALTHY", Handler: cmdSERVICEHEALTHY})
	router.Register(&CommandDef{Name: "SERVICE.WEIGHT", Handler: cmdSERVICEWEIGHT})
	router.Register(&CommandDef{Name: "SERVICE.TAGS", Handler: cmdSERVICETAGS})

	router.Register(&CommandDef{Name: "VECTOR.CREATE", Handler: cmdVECTORCREATE})
	router.Register(&CommandDef{Name: "VECTOR.ADD", Handler: cmdVECTORADD})
	router.Register(&CommandDef{Name: "VECTOR.GET", Handler: cmdVECTORGET})
	router.Register(&CommandDef{Name: "VECTOR.DELETE", Handler: cmdVECTORDELETE})
	router.Register(&CommandDef{Name: "VECTOR.SEARCH", Handler: cmdVECTORSEARCH})
	router.Register(&CommandDef{Name: "VECTOR.SIMILARITY", Handler: cmdVECTORSIMILARITY})
	router.Register(&CommandDef{Name: "VECTOR.NORMALIZE", Handler: cmdVECTORNORMALIZE})
	router.Register(&CommandDef{Name: "VECTOR.DIMENSIONS", Handler: cmdVECTORDIMENSIONS})
	router.Register(&CommandDef{Name: "VECTOR.MERGE", Handler: cmdVECTORMERGE})
	router.Register(&CommandDef{Name: "VECTOR.STATS", Handler: cmdVECTORSTATS})

	router.Register(&CommandDef{Name: "DOC.INSERT", Handler: cmdDOCINSERT})
	router.Register(&CommandDef{Name: "DOC.FIND", Handler: cmdDOCFIND})
	router.Register(&CommandDef{Name: "DOC.FINDONE", Handler: cmdDOCFINDONE})
	router.Register(&CommandDef{Name: "DOC.UPDATE", Handler: cmdDOCUPDATE})
	router.Register(&CommandDef{Name: "DOC.DELETE", Handler: cmdDOCDELETE})
	router.Register(&CommandDef{Name: "DOC.COUNT", Handler: cmdDOCCOUNT})
	router.Register(&CommandDef{Name: "DOC.DISTINCT", Handler: docDOCDISTINCT})
	router.Register(&CommandDef{Name: "DOC.AGGREGATE", Handler: cmdDOCAGGREGATE})
	router.Register(&CommandDef{Name: "DOC.INDEX", Handler: cmdDOCINDEX})
	router.Register(&CommandDef{Name: "DOC.DROPINDEX", Handler: cmdDOCDROPINDEX})

	router.Register(&CommandDef{Name: "LEADER.ELECT", Handler: cmdLEADERELECT})
	router.Register(&CommandDef{Name: "LEADER.RENEW", Handler: cmdLEADERRENEW})
	router.Register(&CommandDef{Name: "LEADER.RESIGN", Handler: cmdLEADERRESIGN})
	router.Register(&CommandDef{Name: "LEADER.CURRENT", Handler: cmdLEADERCURRENT})
	router.Register(&CommandDef{Name: "LEADER.HISTORY", Handler: cmdLEADERHISTORY})

	router.Register(&CommandDef{Name: "AGG.SUM", Handler: cmdAGGSUM})
	router.Register(&CommandDef{Name: "AGG.AVG", Handler: cmdAGGAVG})
	router.Register(&CommandDef{Name: "AGG.MIN", Handler: cmdAGGMIN})
	router.Register(&CommandDef{Name: "AGG.MAX", Handler: cmdAGGMAX})
	router.Register(&CommandDef{Name: "AGG.COUNT", Handler: cmdAGGCOUNT})
	router.Register(&CommandDef{Name: "AGG.PUSH", Handler: cmdAGGPUSH})
	router.Register(&CommandDef{Name: "AGG.CLEAR", Handler: cmdAGGCLEAR})
}

type HealthCheckResultX struct {
	Timestamp int64
	Status    string
	Latency   int64
	Message   string
}

type CronJobRun struct {
	Timestamp int64
	Status    string
	Output    string
}

type SentinelAlertX struct {
	Timestamp int64
	Value     int64
	Message   string
}

type Replay struct {
	ID       string
	Status   string
	Speed    float64
	Position int64
	Total    int64
}
