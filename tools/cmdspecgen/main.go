// Command cmdspecgen writes internal/command/redis_command_specs.go from the
// COMMAND reply of a running Redis server:
//
//	docker run -d --rm --name cmdspec -p 127.0.0.1:16399:6379 redis:7.4-alpine
//	go run ./tools/cmdspecgen 127.0.0.1:16399 > internal/command/redis_command_specs.go
//	docker rm -f cmdspec
package main

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func main() {
	c, err := net.Dial("tcp", os.Args[1])
	if err != nil {
		panic(err)
	}
	if _, err := c.Write([]byte("*1\r\n$7\r\nCOMMAND\r\n")); err != nil {
		panic(err)
	}
	v, err := resp.NewReader(c).ReadValue()
	if err != nil {
		panic(err)
	}
	type row struct {
		name              string
		arity             int64
		flags             []string
		first, last, step int64
	}
	var rows []row
	for _, e := range v.Array {
		a := e.Array
		name := strings.ToUpper(string(a[0].Bulk))
		var flags []string
		for _, f := range a[2].Array {
			flags = append(flags, f.Str)
		}
		rows = append(rows, row{name, a[1].Int, flags, a[3].Int, a[4].Int, a[5].Int})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	fmt.Print("package command\n\n")
	fmt.Print("// Code generated from the COMMAND reply of Redis 7.4; DO NOT EDIT.\n\n")
	fmt.Println("// redisCommandSpecs holds the arity, flags and key positions Redis 7.4")
	fmt.Println("// reports for its own commands. Arity follows Redis: a positive value is an")
	fmt.Println("// exact argument count including the command name, a negative value a minimum.")
	fmt.Println("var redisCommandSpecs = map[string]commandSpec{")
	for _, r := range rows {
		fl := make([]string, len(r.flags))
		for i, f := range r.flags {
			fl[i] = fmt.Sprintf("%q", f)
		}
		fmt.Printf("\t%q: {Arity: %d, Flags: []string{%s}, FirstKey: %d, LastKey: %d, KeyStep: %d},\n", r.name, r.arity, strings.Join(fl, ", "), r.first, r.last, r.step)
	}
	fmt.Println("}")
}
