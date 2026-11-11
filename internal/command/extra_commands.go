package command

import (
	"strings"
)

func RegisterExtraCommands(router *Router) {

	router.Register(&CommandDef{Name: "VECTOR_CLOCK.CREATE", Handler: cmdVECTORCLOCKCREATE})
	router.Register(&CommandDef{Name: "VECTOR_CLOCK.INCREMENT", Handler: cmdVECTORCLOCKINCREMENT})
	router.Register(&CommandDef{Name: "VECTOR_CLOCK.COMPARE", Handler: cmdVECTORCLOCKCOMPARE})
	router.Register(&CommandDef{Name: "VECTOR_CLOCK.MERGE", Handler: cmdVECTORCLOCKMERGE})
	router.Register(&CommandDef{Name: "VECTOR_CLOCK.GET", Handler: cmdVECTORCLOCKGET})

	router.Register(&CommandDef{Name: "CRDT.LWW.SET", Handler: cmdCRDTLWWSET})
	router.Register(&CommandDef{Name: "CRDT.LWW.GET", Handler: cmdCRDTLWWGET})
	router.Register(&CommandDef{Name: "CRDT.LWW.DELETE", Handler: cmdCRDTLWWDELETE})
	router.Register(&CommandDef{Name: "CRDT.GCOUNTER.INCR", Handler: cmdCRDTGCOUNTERINCR})
	router.Register(&CommandDef{Name: "CRDT.GCOUNTER.GET", Handler: cmdCRDTGCOUNTERGET})
	router.Register(&CommandDef{Name: "CRDT.PNCounter.INCR", Handler: cmdCRDTPNCOUNTERINCR})
	router.Register(&CommandDef{Name: "CRDT.PNCounter.DECR", Handler: cmdCRDTPNCOUNTERDECR})
	router.Register(&CommandDef{Name: "CRDT.PNCounter.GET", Handler: cmdCRDTPNCOUNTERGET})
	router.Register(&CommandDef{Name: "CRDT.GSET.ADD", Handler: cmdCRDTGSETADD})
	router.Register(&CommandDef{Name: "CRDT.GSET.GET", Handler: cmdCRDTGSETGET})
	router.Register(&CommandDef{Name: "CRDT.ORSET.ADD", Handler: cmdCRDTORSETADD})
	router.Register(&CommandDef{Name: "CRDT.ORSET.REMOVE", Handler: cmdCRDTORSETREMOVE})
	router.Register(&CommandDef{Name: "CRDT.ORSET.GET", Handler: cmdCRDTORSETGET})
	router.Register(&CommandDef{Name: "CRDT.ORSET.TAGS", Handler: cmdCRDTORSETTAGS})

	router.Register(&CommandDef{Name: "DEDUP.ADD", Handler: cmdDEDUPADD})
	router.Register(&CommandDef{Name: "DEDUP.CHECK", Handler: cmdDEDUPCHECK})
	router.Register(&CommandDef{Name: "DEDUP.EXPIRE", Handler: cmdDEDUPEXPIRE})
	router.Register(&CommandDef{Name: "DEDUP.CLEAR", Handler: cmdDEDUPCLEAR})

	router.Register(&CommandDef{Name: "DEADLINE.SET", Handler: cmdDEADLINESET})
	router.Register(&CommandDef{Name: "DEADLINE.CHECK", Handler: cmdDEADLINECHECK})
	router.Register(&CommandDef{Name: "DEADLINE.CANCEL", Handler: cmdDEADLINECANCEL})
	router.Register(&CommandDef{Name: "DEADLINE.LIST", Handler: cmdDEADLINELIST})

	router.Register(&CommandDef{Name: "SANITIZE.STRING", Handler: cmdSANITIZESTRING})
	router.Register(&CommandDef{Name: "SANITIZE.HTML", Handler: cmdSANITIZEHTML})
	router.Register(&CommandDef{Name: "SANITIZE.JSON", Handler: cmdSANITIZEJSON})
	router.Register(&CommandDef{Name: "SANITIZE.SQL", Handler: cmdSANITIZESQL})

	router.Register(&CommandDef{Name: "MASK.CARD", Handler: cmdMASKCARD})
	router.Register(&CommandDef{Name: "MASK.EMAIL", Handler: cmdMASKEMAIL})
	router.Register(&CommandDef{Name: "MASK.PHONE", Handler: cmdMASKPHONE})
	router.Register(&CommandDef{Name: "MASK.IP", Handler: cmdMASKIP})

	router.Register(&CommandDef{Name: "GATEWAY.CREATE", Handler: cmdGATEWAYCREATE})
	router.Register(&CommandDef{Name: "GATEWAY.DELETE", Handler: cmdGATEWAYDELETE})
	router.Register(&CommandDef{Name: "GATEWAY.ROUTE", Handler: cmdGATEWAYROUTE})
	router.Register(&CommandDef{Name: "GATEWAY.LIST", Handler: cmdGATEWAYLIST})

	router.Register(&CommandDef{Name: "THRESHOLD.SET", Handler: cmdTHRESHOLDSET})
	router.Register(&CommandDef{Name: "THRESHOLD.CHECK", Handler: cmdTHRESHOLDCHECK})
	router.Register(&CommandDef{Name: "THRESHOLD.LIST", Handler: cmdTHRESHOLDLIST})
	router.Register(&CommandDef{Name: "THRESHOLD.DELETE", Handler: cmdTHRESHOLDDELETE})

	router.Register(&CommandDef{Name: "SWITCH.STATE", Handler: cmdSWITCHSTATE})
	router.Register(&CommandDef{Name: "SWITCH.TOGGLE", Handler: cmdSWITCHTOGGLE})
	router.Register(&CommandDef{Name: "SWITCH.ON", Handler: cmdSWITCHON})
	router.Register(&CommandDef{Name: "SWITCH.OFF", Handler: cmdSWITCHOFF})
	router.Register(&CommandDef{Name: "SWITCH.LIST", Handler: cmdSWITCHLIST})

	router.Register(&CommandDef{Name: "BOOKMARK.SET", Handler: cmdBOOKMARKSET})
	router.Register(&CommandDef{Name: "BOOKMARK.GET", Handler: cmdBOOKMARKGET})
	router.Register(&CommandDef{Name: "BOOKMARK.DELETE", Handler: cmdBOOKMARKDELETE})
	router.Register(&CommandDef{Name: "BOOKMARK.LIST", Handler: cmdBOOKMARKLIST})

	router.Register(&CommandDef{Name: "ROUTE.ADD", Handler: cmdROUTEADD})
	router.Register(&CommandDef{Name: "ROUTE.REMOVE", Handler: cmdROUTEREMOVE})
	router.Register(&CommandDef{Name: "ROUTE.MATCH", Handler: cmdROUTEMATCH})
	router.Register(&CommandDef{Name: "ROUTE.LIST", Handler: cmdROUTELIST})

	router.Register(&CommandDef{Name: "GHOST.CREATE", Handler: cmdGHOSTCREATE})
	router.Register(&CommandDef{Name: "GHOST.WRITE", Handler: cmdGHOSTWRITE})
	router.Register(&CommandDef{Name: "GHOST.READ", Handler: cmdGHOSTREAD})
	router.Register(&CommandDef{Name: "GHOST.DELETE", Handler: cmdGHOSTDELETE})

	router.Register(&CommandDef{Name: "GRID.CREATE", Handler: cmdGRIDCREATE})
	router.Register(&CommandDef{Name: "GRID.SET", Handler: cmdGRIDSET})
	router.Register(&CommandDef{Name: "GRID.GET", Handler: cmdGRIDGET})
	router.Register(&CommandDef{Name: "GRID.DELETE", Handler: cmdGRIDDELETE})
	router.Register(&CommandDef{Name: "GRID.QUERY", Handler: cmdGRIDQUERY})
	router.Register(&CommandDef{Name: "GRID.CLEAR", Handler: cmdGRIDCLEAR})

	router.Register(&CommandDef{Name: "TAPE.CREATE", Handler: cmdTAPECREATE})
	router.Register(&CommandDef{Name: "TAPE.WRITE", Handler: cmdTAPEWRITE})
	router.Register(&CommandDef{Name: "TAPE.READ", Handler: cmdTAPEREAD})
	router.Register(&CommandDef{Name: "TAPE.SEEK", Handler: cmdTAPESEEK})
	router.Register(&CommandDef{Name: "TAPE.DELETE", Handler: cmdTAPEDELETE})

	router.Register(&CommandDef{Name: "SLICE.CREATE", Handler: cmdSLICECREATE})
	router.Register(&CommandDef{Name: "SLICE.APPEND", Handler: cmdSLICEAPPEND})
	router.Register(&CommandDef{Name: "SLICE.GET", Handler: cmdSLICEGET})
	router.Register(&CommandDef{Name: "SLICE.DELETE", Handler: cmdSLICEDELETE})

	router.Register(&CommandDef{Name: "ROLLUPX.CREATE", Handler: cmdROLLUPXCREATE})
	router.Register(&CommandDef{Name: "ROLLUPX.ADD", Handler: cmdROLLUPXADD})
	router.Register(&CommandDef{Name: "ROLLUPX.GET", Handler: cmdROLLUPXGET})
	router.Register(&CommandDef{Name: "ROLLUPX.DELETE", Handler: cmdROLLUPXDELETE})

}

func cmdSANITIZESTRING(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	input := ctx.ArgString(0)
	result := make([]byte, 0)
	for _, c := range input {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == ' ' || c == '-' || c == '_' {
			result = append(result, byte(c))
		}
	}
	return ctx.WriteBulkString(string(result))
}

func cmdSANITIZEHTML(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	input := ctx.ArgString(0)
	var result strings.Builder
	for _, c := range input {
		switch c {
		case '<':
			result.WriteString("&lt;")
		case '>':
			result.WriteString("&gt;")
		case '&':
			result.WriteString("&amp;")
		case '"':
			result.WriteString("&quot;")
		case '\'':
			result.WriteString("&#39;")
		default:
			result.WriteString(string(c))
		}
	}
	return ctx.WriteBulkString(result.String())
}

func cmdSANITIZEJSON(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	input := ctx.ArgString(0)
	var result strings.Builder
	for _, c := range input {
		switch c {
		case '"':
			result.WriteString("\\\"")
		case '\\':
			result.WriteString("\\\\")
		case '\n':
			result.WriteString("\\n")
		case '\r':
			result.WriteString("\\r")
		case '\t':
			result.WriteString("\\t")
		default:
			result.WriteString(string(c))
		}
	}
	return ctx.WriteBulkString(result.String())
}

func cmdSANITIZESQL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	input := ctx.ArgString(0)
	var result strings.Builder
	for _, c := range input {
		switch c {
		case '\'':
			result.WriteString("''")
		case '\\':
			result.WriteString("\\\\")
		default:
			result.WriteString(string(c))
		}
	}
	return ctx.WriteBulkString(result.String())
}

func cmdMASKCARD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	card := ctx.ArgString(0)
	if len(card) < 4 {
		return ctx.WriteBulkString("****")
	}
	return ctx.WriteBulkString("****-****-****-" + card[len(card)-4:])
}

func cmdMASKEMAIL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	email := ctx.ArgString(0)
	atIdx := -1
	for i, c := range email {
		if c == '@' {
			atIdx = i
			break
		}
	}
	if atIdx <= 0 {
		return ctx.WriteBulkString("***@***.***")
	}
	masked := "***" + email[atIdx:]
	return ctx.WriteBulkString(masked)
}

func cmdMASKPHONE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	phone := ctx.ArgString(0)
	if len(phone) < 4 {
		return ctx.WriteBulkString("****")
	}
	return ctx.WriteBulkString("***-***-" + phone[len(phone)-4:])
}

func cmdMASKIP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ip := ctx.ArgString(0)
	parts := splitBy(ip, '.')
	if len(parts) != 4 {
		return ctx.WriteBulkString("***.***.***.***")
	}
	return ctx.WriteBulkString(parts[0] + ".***.***.***")
}

func splitBy(s string, sep byte) []string {
	parts := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

type Probe struct {
	ID      string
	Name    string
	Target  string
	Results []string
}
