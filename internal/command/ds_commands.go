package command

func RegisterDataStructuresCommands(router *Router) {
	router.Register(&CommandDef{Name: "PQ.CREATE", Handler: cmdPQCREATE})
	router.Register(&CommandDef{Name: "PQ.PUSH", Handler: cmdPQPUSH})
	router.Register(&CommandDef{Name: "PQ.POP", Handler: cmdPQPOP})
	router.Register(&CommandDef{Name: "PQ.PEEK", Handler: cmdPQPEEK})
	router.Register(&CommandDef{Name: "PQ.LEN", Handler: cmdPQLEN})
	router.Register(&CommandDef{Name: "PQ.CLEAR", Handler: cmdPQCLEAR})
	router.Register(&CommandDef{Name: "PQ.GETALL", Handler: cmdPQGETALL})

	router.Register(&CommandDef{Name: "LRU.CREATE", Handler: cmdLRUCREATE})
	router.Register(&CommandDef{Name: "LRU.GET", Handler: cmdLRUGET})
	router.Register(&CommandDef{Name: "LRU.SET", Handler: cmdLRUSET})
	router.Register(&CommandDef{Name: "LRU.DEL", Handler: cmdLRUDEL})
	router.Register(&CommandDef{Name: "LRU.CLEAR", Handler: cmdLRUCLEAR})
	router.Register(&CommandDef{Name: "LRU.KEYS", Handler: cmdLRUKEYS})
	router.Register(&CommandDef{Name: "LRU.STATS", Handler: cmdLRUSTATS})

	router.Register(&CommandDef{Name: "TOKENBUCKET.CREATE", Handler: cmdTOKENBUCKETCREATE})
	router.Register(&CommandDef{Name: "TOKENBUCKET.CONSUME", Handler: cmdTOKENBUCKETCONSUME})
	router.Register(&CommandDef{Name: "TOKENBUCKET.AVAILABLE", Handler: cmdTOKENBUCKETAVAILABLE})
	router.Register(&CommandDef{Name: "TOKENBUCKET.RESET", Handler: cmdTOKENBUCKETRESET})
	router.Register(&CommandDef{Name: "TOKENBUCKET.DELETE", Handler: cmdTOKENBUCKETDELETE})

	router.Register(&CommandDef{Name: "LEAKYBUCKET.CREATE", Handler: cmdLEAKYBUCKETCREATE})
	router.Register(&CommandDef{Name: "LEAKYBUCKET.ADD", Handler: cmdLEAKYBUCKETADD})
	router.Register(&CommandDef{Name: "LEAKYBUCKET.AVAILABLE", Handler: cmdLEAKYBUCKETAVAILABLE})
	router.Register(&CommandDef{Name: "LEAKYBUCKET.DELETE", Handler: cmdLEAKYBUCKETDELETE})

	router.Register(&CommandDef{Name: "SLIDINGWINDOW.CREATE", Handler: cmdSLIDINGWINDOWCREATE})
	router.Register(&CommandDef{Name: "SLIDINGWINDOW.INCR", Handler: cmdSLIDINGWINDOWINCR})
	router.Register(&CommandDef{Name: "SLIDINGWINDOW.COUNT", Handler: cmdSLIDINGWINDOWCOUNT})
	router.Register(&CommandDef{Name: "SLIDINGWINDOW.RESET", Handler: cmdSLIDINGWINDOWRESET})
	router.Register(&CommandDef{Name: "SLIDINGWINDOW.DELETE", Handler: cmdSLIDINGWINDOWDELETE})

	router.Register(&CommandDef{Name: "DEBOUNCE.SET", Handler: cmdDEBOUNCESET})
	router.Register(&CommandDef{Name: "DEBOUNCE.GET", Handler: cmdDEBOUNCEGET})
	router.Register(&CommandDef{Name: "DEBOUNCE.CALL", Handler: cmdDEBOUNCECALL})
	router.Register(&CommandDef{Name: "DEBOUNCE.DELETE", Handler: cmdDEBOUNCEDELETE})

	router.Register(&CommandDef{Name: "THROTTLE.SET", Handler: cmdTHROTTLESET})
	router.Register(&CommandDef{Name: "THROTTLE.CALL", Handler: cmdTHROTTLECALL})
	router.Register(&CommandDef{Name: "THROTTLE.RESET", Handler: cmdTHROTTLERESET})
	router.Register(&CommandDef{Name: "THROTTLE.DELETE", Handler: cmdTHROTTLEDELETE})
}

func parseFloatExt(data []byte) float64 {
	var result float64
	var sign float64 = 1
	var decimal float64 = 1
	inDecimal := false

	i := 0
	if i < len(data) && data[i] == '-' {
		sign = -1
		i++
	} else if i < len(data) && data[i] == '+' {
		i++
	}

	for ; i < len(data); i++ {
		if data[i] >= '0' && data[i] <= '9' {
			digit := float64(data[i] - '0')
			if inDecimal {
				decimal *= 10
				result += digit / decimal
			} else {
				result = result*10 + digit
			}
		} else if data[i] == '.' && !inDecimal {
			inDecimal = true
		} else {
			break
		}
	}

	return sign * result
}
