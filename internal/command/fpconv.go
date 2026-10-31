package command

import "math"

// fpconvDtoa is Redis' fpconv_dtoa, the Grisu2 conversion its replies use
// for doubles. Grisu2 does not always find the shortest digits that read
// back as the value, so Go's shortest formatting differs from it for a few
// values in ten thousand.
func fpconvDtoa(f float64) string {
	var dest []byte
	bits := math.Float64bits(f)
	neg := bits&fpSignMask != 0
	if neg {
		dest = append(dest, '-')
	}
	switch {
	case f == 0:
		return string(append(dest, '0'))
	case bits&fpExpMask == fpExpMask:
		if bits&fpFracMask != 0 {
			return string(append(dest, "nan"...))
		}
		return string(append(dest, "inf"...))
	}
	var digits [18]byte
	ndigits, k := grisu2(f, &digits)
	return string(emitDigits(dest, digits[:ndigits], k, neg))
}

const (
	fpFracMask  = 0x000FFFFFFFFFFFFF
	fpExpMask   = 0x7FF0000000000000
	fpHiddenBit = 0x0010000000000000
	fpSignMask  = 0x8000000000000000
	fpExpBias   = 1023 + 52
)

type fpNum struct {
	frac uint64
	exp  int
}

var fpTens = [20]uint64{10000000000000000000, 1000000000000000000, 100000000000000000, 10000000000000000,
	1000000000000000, 100000000000000, 10000000000000, 1000000000000, 100000000000, 10000000000,
	1000000000, 100000000, 10000000, 1000000, 100000, 10000, 1000, 100, 10, 1}

// fpPowersTen are Grisu's cached powers of ten, 10^-348 to 10^340 in steps
// of 8.
var fpPowersTen = [87]fpNum{
	{18054884314459144840, -1220}, {13451937075301367670, -1193}, {10022474136428063862, -1166},
	{14934650266808366570, -1140}, {11127181549972568877, -1113}, {16580792590934885855, -1087},
	{12353653155963782858, -1060}, {18408377700990114895, -1034}, {13715310171984221708, -1007},
	{10218702384817765436, -980}, {15227053142812498563, -954}, {11345038669416679861, -927},
	{16905424996341287883, -901}, {12595523146049147757, -874}, {9384396036005875287, -847},
	{13983839803942852151, -821}, {10418772551374772303, -794}, {15525180923007089351, -768},
	{11567161174868858868, -741}, {17236413322193710309, -715}, {12842128665889583758, -688},
	{9568131466127621947, -661}, {14257626930069360058, -635}, {10622759856335341974, -608},
	{15829145694278690180, -582}, {11793632577567316726, -555}, {17573882009934360870, -529},
	{13093562431584567480, -502}, {9755464219737475723, -475}, {14536774485912137811, -449},
	{10830740992659433045, -422}, {16139061738043178685, -396}, {12024538023802026127, -369},
	{17917957937422433684, -343}, {13349918974505688015, -316}, {9946464728195732843, -289},
	{14821387422376473014, -263}, {11042794154864902060, -236}, {16455045573212060422, -210},
	{12259964326927110867, -183}, {18268770466636286478, -157}, {13611294676837538539, -130},
	{10141204801825835212, -103}, {15111572745182864684, -77}, {11258999068426240000, -50},
	{16777216000000000000, -24}, {12500000000000000000, 3}, {9313225746154785156, 30},
	{13877787807814456755, 56}, {10339757656912845936, 83}, {15407439555097886824, 109},
	{11479437019748901445, 136}, {17105694144590052135, 162}, {12744735289059618216, 189},
	{9495567745759798747, 216}, {14149498560666738074, 242}, {10542197943230523224, 269},
	{15709099088952724970, 295}, {11704190886730495818, 322}, {17440603504673385349, 348},
	{12994262207056124023, 375}, {9681479787123295682, 402}, {14426529090290212157, 428},
	{10748601772107342003, 455}, {16016664761464807395, 481}, {11933345169920330789, 508},
	{17782069995880619868, 534}, {13248674568444952270, 561}, {9871031767461413346, 588},
	{14708983551653345445, 614}, {10959046745042015199, 641}, {16330252207878254650, 667},
	{12166986024289022870, 694}, {18130221999122236476, 720}, {13508068024458167312, 747},
	{10064294952495520794, 774}, {14996968138956309548, 800}, {11173611982879273257, 827},
	{16649979327439178909, 853}, {12405201291620119593, 880}, {9242595204427927429, 907},
	{13772540099066387757, 933}, {10261342003245940623, 960}, {15290591125556738113, 986},
	{11392378155556871081, 1013}, {16975966327722178521, 1039}, {12648080533535911531, 1066},
}

func fpBuild(d float64) fpNum {
	bits := math.Float64bits(d)
	fp := fpNum{frac: bits & fpFracMask, exp: int((bits & fpExpMask) >> 52)}
	if fp.exp != 0 {
		fp.frac += fpHiddenBit
		fp.exp -= fpExpBias
	} else {
		fp.exp = -fpExpBias + 1
	}
	return fp
}

func fpNormalize(fp *fpNum) {
	for fp.frac&fpHiddenBit == 0 {
		fp.frac <<= 1
		fp.exp--
	}
	const shift = 64 - 52 - 1
	fp.frac <<= shift
	fp.exp -= shift
}

func fpBoundaries(fp *fpNum) (lower, upper fpNum) {
	upper.frac = fp.frac<<1 + 1
	upper.exp = fp.exp - 1
	for upper.frac&(fpHiddenBit<<1) == 0 {
		upper.frac <<= 1
		upper.exp--
	}
	const uShift = 64 - 52 - 2
	upper.frac <<= uShift
	upper.exp -= uShift
	lShift := 1
	if fp.frac == fpHiddenBit {
		lShift = 2
	}
	lower.frac = fp.frac<<lShift - 1
	lower.exp = fp.exp - lShift
	lower.frac <<= uint(lower.exp - upper.exp)
	lower.exp = upper.exp
	return lower, upper
}

func fpMultiply(a, b fpNum) fpNum {
	const lomask = 0x00000000FFFFFFFF
	ahBl := (a.frac >> 32) * (b.frac & lomask)
	alBh := (a.frac & lomask) * (b.frac >> 32)
	alBl := (a.frac & lomask) * (b.frac & lomask)
	ahBh := (a.frac >> 32) * (b.frac >> 32)
	tmp := ahBl&lomask + alBh&lomask + alBl>>32
	tmp += 1 << 31
	return fpNum{frac: ahBh + ahBl>>32 + alBh>>32 + tmp>>32, exp: a.exp + b.exp + 64}
}

func fpCachedPow10(exp int) (fpNum, int) {
	const oneLogTen = 0.30102999566398114
	approx := int(float64(-(exp + 87)) * oneLogTen)
	idx := (approx + 348) / 8
	for {
		current := exp + fpPowersTen[idx].exp + 64
		switch {
		case current < -60:
			idx++
		case current > -32:
			idx--
		default:
			return fpPowersTen[idx], -348 + idx*8
		}
	}
}

func fpRoundDigit(digits []byte, delta, rem, kappa, frac uint64) {
	for rem < frac && delta-rem >= kappa && (rem+kappa < frac || frac-rem > rem+kappa-frac) {
		digits[len(digits)-1]--
		rem += kappa
	}
}

func fpGenerateDigits(fp, upper, lower fpNum, digits *[18]byte, k *int) int {
	wfrac := upper.frac - fp.frac
	delta := upper.frac - lower.frac
	oneExp := uint(-upper.exp)
	oneFrac := uint64(1) << oneExp
	part1 := upper.frac >> oneExp
	part2 := upper.frac & (oneFrac - 1)
	idx, kappa := 0, 10
	for divp := 10; kappa > 0; divp++ {
		div := fpTens[divp]
		digit := part1 / div
		if digit != 0 || idx != 0 {
			digits[idx] = byte(digit) + '0'
			idx++
		}
		part1 -= digit * div
		kappa--
		tmp := part1<<oneExp + part2
		if tmp <= delta {
			*k += kappa
			fpRoundDigit(digits[:idx], delta, tmp, div<<oneExp, wfrac)
			return idx
		}
	}
	unit := 18
	for {
		part2 *= 10
		delta *= 10
		kappa--
		digit := part2 >> oneExp
		if digit != 0 || idx != 0 {
			digits[idx] = byte(digit) + '0'
			idx++
		}
		part2 &= oneFrac - 1
		if part2 < delta {
			*k += kappa
			fpRoundDigit(digits[:idx], delta, part2, oneFrac, wfrac*fpTens[unit])
			return idx
		}
		unit--
	}
}

func grisu2(d float64, digits *[18]byte) (int, int) {
	w := fpBuild(d)
	lower, upper := fpBoundaries(&w)
	fpNormalize(&w)
	cp, k := fpCachedPow10(upper.exp)
	w = fpMultiply(w, cp)
	upper = fpMultiply(upper, cp)
	lower = fpMultiply(lower, cp)
	lower.frac++
	upper.frac--
	K := -k
	n := fpGenerateDigits(w, upper, lower, digits, &K)
	return n, K
}

func emitDigits(dest, digits []byte, k int, neg bool) []byte {
	ndigits := len(digits)
	exp := k + ndigits - 1
	if exp < 0 {
		exp = -exp
	}
	if k >= 0 && exp < ndigits+7 {
		dest = append(dest, digits...)
		for range k {
			dest = append(dest, '0')
		}
		return dest
	}
	if k < 0 && (k > -7 || exp < 4) {
		offset := ndigits + k
		if offset <= 0 {
			dest = append(dest, '0', '.')
			for range -offset {
				dest = append(dest, '0')
			}
			return append(dest, digits...)
		}
		dest = append(dest, digits[:offset]...)
		dest = append(dest, '.')
		return append(dest, digits[offset:]...)
	}
	limit := 18
	if neg {
		limit = 17
	}
	ndigits = min(ndigits, limit)
	dest = append(dest, digits[0])
	if ndigits > 1 {
		dest = append(dest, '.')
		dest = append(dest, digits[1:ndigits]...)
	}
	dest = append(dest, 'e')
	if k+ndigits-1 < 0 {
		dest = append(dest, '-')
	} else {
		dest = append(dest, '+')
	}
	cent := 0
	if exp > 99 {
		cent = exp / 100
		dest = append(dest, byte(cent)+'0')
		exp -= cent * 100
	}
	if exp > 9 {
		dec := exp / 10
		dest = append(dest, byte(dec)+'0')
		exp -= dec * 10
	} else if cent != 0 {
		dest = append(dest, '0')
	}
	return append(dest, byte(exp%10)+'0')
}
