package search

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// tmTime is C's struct tm in UTC.
type tmTime struct {
	sec, min, hour, mday, mon, year, wday, yday int
}

func gmtime(t int64) tmTime {
	u := time.Unix(t, 0).UTC()
	return tmTime{sec: u.Second(), min: u.Minute(), hour: u.Hour(), mday: u.Day(), mon: int(u.Month()) - 1,
		year: u.Year() - 1900, wday: int(u.Weekday()), yday: u.YearDay() - 1}
}

// fastTimegm is RediSearch's fast_timegm, whose leap years are every
// fourth from 1972.
func (t tmTime) fastTimegm() float64 {
	years := int64(t.year - 70)
	leaps := (years + 1) / 4
	days := int64(t.yday) + years*365 + leaps
	return float64(days*86400 + int64(t.hour)*3600 + int64(t.min)*60 + int64(t.sec))
}

// timegm is C's timegm, normalizing the fields as mktime does.
func (t tmTime) timegm() int64 {
	return time.Date(t.year+1900, time.Month(t.mon+1), t.mday, t.hour, t.min, t.sec, 0, time.UTC).Unix()
}

// dateFunc reads a timestamp as RediSearch's date functions do: a number
// not below 0, truncated to whole seconds; anything else is null.
func dateFunc(f func(tmTime) float64) func(*exprEval, []xval) (xval, int) {
	return func(_ *exprEval, a []xval) (xval, int) {
		d, ok := a[0].toNumber()
		if !ok || d < 0 {
			return xval{t: xNull}, evalOK
		}
		t, ok := cGmtime(cTimeT(d))
		if !ok {
			return xval{t: xNull}, evalOK
		}
		return xnum(f(t)), evalOK
	}
}

// cTimeT is C's (time_t) cast on x86, which makes INT64_MIN of a value it
// cannot hold.
func cTimeT(d float64) int64 {
	if d != d || d >= 1<<63 || d < -1<<63 {
		return math.MinInt64
	}
	return int64(d)
}

// cGmtime is glibc's gmtime_r, which fails when the year does not fit an
// int.
func cGmtime(t int64) (tmTime, bool) {
	days := t / 86400
	if t%86400 < 0 {
		days--
	}
	year := 1970 + float64(days)/365.2425
	if year-1900 > math.MaxInt32 || year-1900 < math.MinInt32 {
		return tmTime{}, false
	}
	return gmtime(t), true
}

func fnDayOfWeek(_ *exprEval, a []xval) (xval, int) {
	d, ok := a[0].toNumber()
	if !ok || d < 0 {
		return xval{t: xNull}, evalOK
	}
	t := cTimeT(d)
	days := t / 86400
	if t%86400 < 0 {
		days--
	}
	w := (4 + days) % 7
	if w < 0 {
		w += 7
	}
	return xnum(float64(w)), evalOK
}

func fnMinute(_ *exprEval, a []xval) (xval, int) {
	d, ok := a[0].toNumber()
	if !ok || d < 0 {
		return xval{t: xNull}, evalOK
	}
	return xnum(math.Floor(d - cFmod(d, 60))), evalOK
}

func fnTimefmt(ev *exprEval, a []xval) (xval, int) {
	format := "%FT%TZ"
	if len(a) == 2 {
		if a[1].t != xStr {
			ev.st.setError(fmt.Sprintf("Invalid type (%d) for argument 1 in function 'time'. VALIDATE_ARG__TYPE(v, RSValue_String) was false.", a[1].t))
			return xval{}, evalErr
		}
		format = cString(a[1].s)
	}
	d, ok := a[0].toNumber()
	if !ok {
		return xval{t: xNull}, evalOK
	}
	tt := cTimeT(d)
	tm, ok := cGmtime(tt)
	if !ok {
		return xval{t: xNull}, evalOK
	}
	out := strftime(format, tm, tt)
	if out == "" || len(out) >= 1024 {
		return xval{t: xNull}, evalOK
	}
	return xstr(out), evalOK
}

var (
	cDays   = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	cMonths = []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
)

// isoWeek is the ISO 8601 week and its year, as glibc's %V and %G.
func isoWeek(t tmTime) (int, int) {
	y, w := time.Date(t.year+1900, time.Month(t.mon+1), t.mday, 0, 0, 0, 0, time.UTC).ISOWeek()
	return w, y
}

// strftime is glibc's in the C locale, for UTC.
func strftime(format string, t tmTime, epoch int64) string {
	var b strings.Builder
	hour12 := t.hour % 12
	if hour12 == 0 {
		hour12 = 12
	}
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' || i+1 >= len(format) {
			b.WriteByte(c)
			continue
		}
		i++
		switch format[i] {
		case 'a':
			b.WriteString(cDays[t.wday][:3])
		case 'A':
			b.WriteString(cDays[t.wday])
		case 'b', 'h':
			b.WriteString(cMonths[t.mon][:3])
		case 'B':
			b.WriteString(cMonths[t.mon])
		case 'c':
			b.WriteString(strftime("%a %b %e %H:%M:%S %Y", t, epoch))
		case 'C':
			fmt.Fprintf(&b, "%02d", (t.year+1900)/100)
		case 'd':
			fmt.Fprintf(&b, "%02d", t.mday)
		case 'D':
			b.WriteString(strftime("%m/%d/%y", t, epoch))
		case 'e':
			fmt.Fprintf(&b, "%2d", t.mday)
		case 'F':
			b.WriteString(strftime("%Y-%m-%d", t, epoch))
		case 'g':
			_, y := isoWeek(t)
			fmt.Fprintf(&b, "%02d", y%100)
		case 'G':
			_, y := isoWeek(t)
			fmt.Fprintf(&b, "%d", y)
		case 'H':
			fmt.Fprintf(&b, "%02d", t.hour)
		case 'I':
			fmt.Fprintf(&b, "%02d", hour12)
		case 'j':
			fmt.Fprintf(&b, "%03d", t.yday+1)
		case 'k':
			fmt.Fprintf(&b, "%2d", t.hour)
		case 'l':
			fmt.Fprintf(&b, "%2d", hour12)
		case 'm':
			fmt.Fprintf(&b, "%02d", t.mon+1)
		case 'M':
			fmt.Fprintf(&b, "%02d", t.min)
		case 'n':
			b.WriteByte('\n')
		case 'p':
			if t.hour < 12 {
				b.WriteString("AM")
			} else {
				b.WriteString("PM")
			}
		case 'P':
			if t.hour < 12 {
				b.WriteString("am")
			} else {
				b.WriteString("pm")
			}
		case 'r':
			b.WriteString(strftime("%I:%M:%S %p", t, epoch))
		case 'R':
			b.WriteString(strftime("%H:%M", t, epoch))
		case 's':
			fmt.Fprintf(&b, "%d", epoch)
		case 'S':
			fmt.Fprintf(&b, "%02d", t.sec)
		case 't':
			b.WriteByte('\t')
		case 'T':
			b.WriteString(strftime("%H:%M:%S", t, epoch))
		case 'u':
			fmt.Fprintf(&b, "%d", (t.wday+6)%7+1)
		case 'U':
			fmt.Fprintf(&b, "%02d", (t.yday+7-t.wday)/7)
		case 'V':
			w, _ := isoWeek(t)
			fmt.Fprintf(&b, "%02d", w)
		case 'w':
			fmt.Fprintf(&b, "%d", t.wday)
		case 'W':
			fmt.Fprintf(&b, "%02d", (t.yday+7-(t.wday+6)%7)/7)
		case 'x':
			b.WriteString(strftime("%m/%d/%y", t, epoch))
		case 'X':
			b.WriteString(strftime("%H:%M:%S", t, epoch))
		case 'y':
			fmt.Fprintf(&b, "%02d", (t.year+1900)%100)
		case 'Y':
			fmt.Fprintf(&b, "%d", t.year+1900)
		case 'z':
			b.WriteString("+0000")
		case 'Z':
			b.WriteString("GMT")
		case '%':
			b.WriteByte('%')
		default:
			b.WriteByte('%')
			b.WriteByte(format[i])
		}
	}
	return b.String()
}

func fnParsetime(ev *exprEval, a []xval) (xval, int) {
	if !validate(ev, "parsetime", a, 0, false) || !validate(ev, "parsetime", a, 1, false) {
		return xval{}, evalErr
	}
	t, ok := strptime(cString(a[0].s), cString(a[1].s))
	if !ok {
		return xval{t: xNull}, evalOK
	}
	return xnum(float64(t.timegm())), evalOK
}

// strptime is glibc's for the C locale's conversions RediSearch users
// write; the fields it does not set stay 0, so a missing day is the 0th.
func strptime(s, format string) (tmTime, bool) {
	var t tmTime
	pm, have12 := false, false
	century, haveCentury := 0, false
	i := 0
	skipSpace := func() {
		for i < len(s) && isCSpace(s[i]) {
			i++
		}
	}
	// number is glibc's get_number: digits after spaces, at most n of
	// them, and no more once another would pass hi.
	number := func(n int, lo, hi int) (int, bool) {
		skipSpace()
		if i >= len(s) || !isDigit(s[i]) {
			return 0, false
		}
		v := 0
		for {
			v = v*10 + int(s[i]-'0')
			i++
			n--
			if n <= 0 || v*10 > hi || i >= len(s) || !isDigit(s[i]) {
				break
			}
		}
		return v, v >= lo && v <= hi
	}
	name := func(names []string) (int, bool) {
		for k, n := range names {
			for _, cand := range []string{n, n[:3]} {
				if len(s)-i >= len(cand) && strings.EqualFold(s[i:i+len(cand)], cand) {
					i += len(cand)
					return k, true
				}
			}
		}
		return 0, false
	}
	var conv func(f string) bool
	conv = func(f string) bool {
		for j := 0; j < len(f); j++ {
			c := f[j]
			if isCSpace(c) {
				skipSpace()
				continue
			}
			if c != '%' || j+1 >= len(f) {
				if i >= len(s) || s[i] != c {
					return false
				}
				i++
				continue
			}
			j++
			var ok bool
			switch f[j] {
			case '%':
				ok = i < len(s) && s[i] == '%'
				i++
			case 'a', 'A':
				skipSpace()
				t.wday, ok = name(cDays)
			case 'b', 'B', 'h':
				skipSpace()
				t.mon, ok = name(cMonths)
			case 'C':
				century, ok = number(2, 0, 99)
				haveCentury = true
			case 'd', 'e':
				t.mday, ok = number(2, 1, 31)
			case 'D':
				ok = conv("%m/%d/%y")
			case 'F':
				ok = conv("%Y-%m-%d")
			case 'H', 'k':
				t.hour, ok = number(2, 0, 23)
				have12 = false
			case 'I', 'l':
				t.hour, ok = number(2, 1, 12)
				t.hour %= 12
				have12 = true
			case 'j':
				var d int
				d, ok = number(3, 1, 366)
				t.yday = d - 1
			case 'm':
				var m int
				m, ok = number(2, 1, 12)
				t.mon = m - 1
			case 'M':
				t.min, ok = number(2, 0, 59)
			case 'n', 't':
				skipSpace()
				ok = true
			case 'p', 'P':
				skipSpace()
				switch {
				case len(s)-i >= 2 && strings.EqualFold(s[i:i+2], "AM"):
					pm, ok = false, true
				case len(s)-i >= 2 && strings.EqualFold(s[i:i+2], "PM"):
					pm, ok = true, true
				}
				i += 2
			case 'r':
				ok = conv("%I:%M:%S %p")
			case 'R':
				ok = conv("%H:%M")
			case 's':
				neg := i < len(s) && s[i] == '-'
				if neg {
					i++
				}
				var v int
				v, ok = number(19, 0, math.MaxInt)
				if neg {
					v = -v
				}
				if ok {
					t = gmtime(int64(v))
				}
			case 'S':
				t.sec, ok = number(2, 0, 61)
			case 'T':
				ok = conv("%H:%M:%S")
			case 'u':
				var d int
				d, ok = number(1, 1, 7)
				t.wday = d % 7
			case 'w':
				t.wday, ok = number(1, 0, 6)
			case 'y':
				var y int
				y, ok = number(2, 0, 99)
				if y < 69 {
					y += 100
				}
				t.year = y
			case 'Y':
				var y int
				y, ok = number(4, 0, 9999)
				t.year = y - 1900
			case 'z':
				skipSpace()
				if i < len(s) && s[i] == 'Z' {
					i++
					ok = true
					break
				}
				if i < len(s) && (s[i] == '+' || s[i] == '-') {
					i++
					start := i
					for i < len(s) && i-start < 4 && (isDigit(s[i]) || s[i] == ':') {
						i++
					}
					ok = i > start
				}
			case 'Z':
				for i < len(s) && !isCSpace(s[i]) {
					i++
				}
				ok = true
			default:
				return false
			}
			if !ok {
				return false
			}
		}
		return true
	}
	if !conv(format) {
		return t, false
	}
	if have12 && pm {
		t.hour += 12
	}
	if haveCentury {
		t.year = century*100 + t.year%100 - 1900
	}
	return t, true
}
