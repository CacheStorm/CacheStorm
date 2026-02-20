package command

import (
	"fmt"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

var tsManager = store.NewTimeSeriesManager()

func RegisterTSCommands(router *Router) {
	router.Register(&CommandDef{Name: "TS.CREATE", Handler: cmdTSCREATE})
	router.Register(&CommandDef{Name: "TS.DEL", Handler: cmdTSDEL})
	router.Register(&CommandDef{Name: "TS.ADD", Handler: cmdTSADD})
	router.Register(&CommandDef{Name: "TS.MADD", Handler: cmdTSMADD})
	router.Register(&CommandDef{Name: "TS.RANGE", Handler: cmdTSRANGE})
	router.Register(&CommandDef{Name: "TS.REVRANGE", Handler: cmdTSREVRANGE})
	router.Register(&CommandDef{Name: "TS.GET", Handler: cmdTSGET})
	router.Register(&CommandDef{Name: "TS.INFO", Handler: cmdTSINFO})
	router.Register(&CommandDef{Name: "TS.QUERYINDEX", Handler: cmdTSQUERYINDEX})
	router.Register(&CommandDef{Name: "TS.ALTER", Handler: cmdTSALTER})
	router.Register(&CommandDef{Name: "TS.INCRBY", Handler: cmdTSINCRBY})
	router.Register(&CommandDef{Name: "TS.DECRBY", Handler: cmdTSDECRBY})
}

func cmdTSCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	retention := time.Duration(0)
	labels := make(map[string]string)

	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "RETENTION":
			i++
			if i < ctx.ArgCount() {
				ms := parseInt64(ctx.ArgString(i))
				retention = time.Duration(ms) * time.Millisecond
			}
		case "LABELS":
			for i+2 < ctx.ArgCount() {
				i++
				labelKey := ctx.ArgString(i)
				i++
				labelVal := ctx.ArgString(i)
				labels[labelKey] = labelVal
			}
		}
	}

	tsManager.Create(key, retention, labels)
	return ctx.WriteOK()
}

func cmdTSDEL(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	from := parseInt64(ctx.ArgString(1))
	to := parseInt64(ctx.ArgString(2))

	ts, ok := tsManager.Get(key)
	if !ok {
		return ctx.WriteInteger(0)
	}

	deleted := 0
	for t := from; t <= to; {
		deleted += ts.Delete(t)
		t++
	}

	return ctx.WriteInteger(int64(deleted))
}

func cmdTSADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	timestamp := parseInt64(ctx.ArgString(1))
	value := parseJSONFloat(ctx.ArgString(2))

	retention := time.Duration(0)
	labels := make(map[string]string)
	onDuplicate := ""

	for i := 3; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "RETENTION":
			i++
			if i < ctx.ArgCount() {
				ms := parseInt64(ctx.ArgString(i))
				retention = time.Duration(ms) * time.Millisecond
			}
		case "LABELS":
			for i+2 < ctx.ArgCount() {
				i++
				labelKey := ctx.ArgString(i)
				i++
				labelVal := ctx.ArgString(i)
				labels[labelKey] = labelVal
			}
		case "ON_DUPLICATE":
			i++
			if i < ctx.ArgCount() {
				onDuplicate = ctx.ArgString(i)
			}
		}
	}

	ts, ok := tsManager.Get(key)
	if !ok {
		tsManager.Create(key, retention, labels)
		ts, _ = tsManager.Get(key)
	}

	_ = onDuplicate

	resultTs := ts.Add(timestamp, value)
	return ctx.WriteInteger(resultTs)
}

func cmdTSMADD(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount()%3 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	results := make([]*resp.Value, 0)

	for i := 0; i < ctx.ArgCount(); i += 3 {
		key := ctx.ArgString(i)
		timestamp := parseInt64(ctx.ArgString(i + 1))
		value := parseJSONFloat(ctx.ArgString(i + 2))

		ts, ok := tsManager.Get(key)
		if !ok {
			tsManager.Create(key, 0, nil)
			ts, _ = tsManager.Get(key)
		}

		resultTs := ts.Add(timestamp, value)
		results = append(results, resp.IntegerValue(resultTs))
	}

	return ctx.WriteArray(results)
}

func cmdTSRANGE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	from := parseInt64(ctx.ArgString(1))
	to := parseInt64(ctx.ArgString(2))

	count := 0
	aggType := ""
	bucketSize := int64(0)

	for i := 3; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "COUNT":
			i++
			if i < ctx.ArgCount() {
				count = int(parseInt64(ctx.ArgString(i)))
			}
		case "AGGREGATION":
			i++
			if i < ctx.ArgCount() {
				aggType = strings.ToUpper(ctx.ArgString(i))
			}
			i++
			if i < ctx.ArgCount() {
				bucketSize = parseInt64(ctx.ArgString(i))
			}
		}
	}

	ts, ok := tsManager.Get(key)
	if !ok {
		return ctx.WriteArray([]*resp.Value{})
	}

	var samples []store.TimeSeriesSample
	if aggType != "" && bucketSize > 0 {
		samples = ts.Aggregation(from, to, aggType, bucketSize)
	} else if count > 0 {
		samples = ts.RangeWithCount(from, to, count)
	} else {
		samples = ts.Range(from, to)
	}

	results := make([]*resp.Value, 0, len(samples))
	for _, s := range samples {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.IntegerValue(s.Timestamp),
			resp.BulkString(fmt.Sprintf("%v", s.Value)),
		}))
	}

	return ctx.WriteArray(results)
}

func cmdTSREVRANGE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	from := parseInt64(ctx.ArgString(1))
	to := parseInt64(ctx.ArgString(2))

	ts, ok := tsManager.Get(key)
	if !ok {
		return ctx.WriteArray([]*resp.Value{})
	}

	samples := ts.Range(from, to)

	results := make([]*resp.Value, 0, len(samples))
	for i := len(samples) - 1; i >= 0; i-- {
		s := samples[i]
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.IntegerValue(s.Timestamp),
			resp.BulkString(fmt.Sprintf("%v", s.Value)),
		}))
	}

	return ctx.WriteArray(results)
}

func cmdTSGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	ts, ok := tsManager.Get(key)
	if !ok {
		return ctx.WriteArray([]*resp.Value{})
	}

	latest := ts.Latest()
	if latest == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	return ctx.WriteArray([]*resp.Value{
		resp.IntegerValue(latest.Timestamp),
		resp.BulkString(fmt.Sprintf("%v", latest.Value)),
	})
}

func cmdTSINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	ts, ok := tsManager.Get(key)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR key does not exist"))
	}

	first := ts.First()
	latest := ts.Latest()

	firstTs := int64(0)
	lastTs := int64(0)
	if first != nil {
		firstTs = first.Timestamp
	}
	if latest != nil {
		lastTs = latest.Timestamp
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("totalSamples"),
		resp.IntegerValue(int64(ts.Len())),
		resp.BulkString("memoryUsage"),
		resp.IntegerValue(ts.SizeOf()),
		resp.BulkString("firstTimestamp"),
		resp.IntegerValue(firstTs),
		resp.BulkString("lastTimestamp"),
		resp.IntegerValue(lastTs),
		resp.BulkString("retentionTime"),
		resp.IntegerValue(ts.Retention.Milliseconds()),
		resp.BulkString("labels"),
		formatLabels(ts.GetLabels()),
	})
}

func formatLabels(labels map[string]string) *resp.Value {
	result := make([]*resp.Value, 0, len(labels))
	for k, v := range labels {
		result = append(result, resp.ArrayValue([]*resp.Value{
			resp.BulkString(k),
			resp.BulkString(v),
		}))
	}
	return resp.ArrayValue(result)
}

func cmdTSQUERYINDEX(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	labels := make(map[string]string)
	for i := 0; i+1 < ctx.ArgCount(); i += 2 {
		labelKey := ctx.ArgString(i)
		labelVal := ctx.ArgString(i + 1)
		labels[labelKey] = labelVal
	}

	keys := tsManager.QueryByLabels(labels, "")
	results := make([]*resp.Value, 0, len(keys))
	for _, k := range keys {
		results = append(results, resp.BulkString(k))
	}

	return ctx.WriteArray(results)
}

func cmdTSALTER(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	retention := time.Duration(0)
	labels := make(map[string]string)

	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "RETENTION":
			i++
			if i < ctx.ArgCount() {
				ms := parseInt64(ctx.ArgString(i))
				retention = time.Duration(ms) * time.Millisecond
			}
		case "LABELS":
			for i+2 < ctx.ArgCount() {
				i++
				labelKey := ctx.ArgString(i)
				i++
				labelVal := ctx.ArgString(i)
				labels[labelKey] = labelVal
			}
		}
	}

	ts, ok := tsManager.Get(key)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR key does not exist"))
	}

	if retention > 0 {
		ts.SetRetention(retention)
	}

	if len(labels) > 0 {
		ts.SetLabels(labels)
	}

	return ctx.WriteOK()
}

func cmdTSINCRBY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	increment := parseJSONFloat(ctx.ArgString(1))

	ts, ok := tsManager.Get(key)
	if !ok {
		tsManager.Create(key, 0, nil)
		ts, _ = tsManager.Get(key)
	}

	latest := ts.Latest()
	var newValue float64
	if latest != nil {
		newValue = latest.Value + increment
	} else {
		newValue = increment
	}

	timestamp := ts.Add(0, newValue)
	return ctx.WriteInteger(timestamp)
}

func cmdTSDECRBY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	decrement := parseJSONFloat(ctx.ArgString(1))

	ts, ok := tsManager.Get(key)
	if !ok {
		tsManager.Create(key, 0, nil)
		ts, _ = tsManager.Get(key)
	}

	latest := ts.Latest()
	var newValue float64
	if latest != nil {
		newValue = latest.Value - decrement
	} else {
		newValue = -decrement
	}

	timestamp := ts.Add(0, newValue)
	return ctx.WriteInteger(timestamp)
}

func parseInt64(s string) int64 {
	var result int64
	var sign int64 = 1
	i := 0

	if len(s) > 0 && s[0] == '-' {
		sign = -1
		i = 1
	}

	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		result = result*10 + int64(s[i]-'0')
		i++
	}

	return result * sign
}
