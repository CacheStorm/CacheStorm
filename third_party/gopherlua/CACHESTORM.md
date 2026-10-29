# gopher-lua for CacheStorm

This is [gopher-lua](https://github.com/yuin/gopher-lua) v1.1.1 (MIT, see
LICENSE), imported as a package of this module so its behavior can follow the
Lua 5.1 that Redis embeds. The tests, the `glua` command and the go-inline
templates (`_vm.go`, `_state.go`) were left out; `vm.go` and `state.go` are
edited directly.

Changes from upstream:

- Numbers convert to strings as Lua 5.1's `%.14g` (`inf`, `-inf`, `nan`), and
  strings to numbers as `luaO_str2d` (`strtod`, then `strtoul` for hex).
- `tonumber`, `string.format`, `string.char`, `next`, `setmetatable`,
  `table.insert`, `math.huge`, `math.random` and `math.randomseed` follow Lua
  5.1 and Redis (`redisLrand48`).
- Error messages follow Lua 5.1: `bad argument #n to 'name'`, `got no value`,
  method argument numbering, positions as `luaL_where` and `luaG_runerror`
  give them, and variable names from a port of `getobjname` (`varinfo.go`).
- Tables can be made readonly, as Redis' `lua_enablereadonlytable` does.
- `string.rep` and concatenation refuse results over 512 MB.
- `go vet` fixes: constant format strings, unreachable code, self-assignments.
