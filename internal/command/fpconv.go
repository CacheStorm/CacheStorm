package command

import "github.com/cachestorm/cachestorm/internal/fpconv"

// fpconvDtoa is Redis' fpconv_dtoa (see package fpconv).
func fpconvDtoa(f float64) string { return fpconv.Dtoa(f) }
