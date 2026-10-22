module cachestorm-demo

go 1.26.10

// The root module requires go >= 1.26.10 and this module replaces it
// with the local checkout, so its go directive has to match: with the
// older 1.22 floor the go command refuses to load the module at all
// ("module ../.. requires go >= 1.26.10").

require github.com/cachestorm/cachestorm v0.1.27

replace github.com/cachestorm/cachestorm => ../..
