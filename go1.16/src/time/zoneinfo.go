// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

import (
	"errors"
	"sync"
	"syscall"
)

//go:generate env ZONEINFO=$GOROOT/lib/time/zoneinfo.zip go run genzabbrs.go -output zoneinfo_abbrs_windows.go

// A Location maps time instants to the zone in use at that time.
// Typically, the Location represents the collection of time offsets
// in use in a geographical area. For many Locations the time offset varies
// depending on whether daylight savings time is in use at the time instant.
// 一个位置将时间实例映射到当时使用的区域。通常情况下，Location代表一个地理区域内使用的时间偏移的集合。
// 对于许多地点来说，时间偏移是不同的，这取决于该时间点是否在使用夏令时。
type Location struct {
	name string
	zone []zone
	tx   []zoneTrans

	// The tzdata information can be followed by a string that describes
	// how to handle DST transitions not recorded in zoneTrans.
	// The format is the TZ environment variable without a colon; see
	// https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap08.html.
	// Example string, for America/Los_Angeles: PST8PDT,M3.2.0,M11.1.0
	// tzdata信息后面可以有一个字符串，描述如何处理zoneTrans中没有记录的DST转换。其格式是不带冒号的TZ环境变量；见
	// https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap08.html。
	// 示例字符串，用于美国/洛杉矶。PST8PDT,M3.2.0,M11.1.0
	extend string

	// Most lookups will be for the current time.
	// To avoid the binary search through tx, keep a
	// static one-element cache that gives the correct
	// zone for the time when the Location was created.
	// if cacheStart <= t < cacheEnd,
	// lookup can return cacheZone.
	// The units for cacheStart and cacheEnd are seconds
	// since January 1, 1970 UTC, to match the argument
	// to lookup.
	// 大多数查找将是针对当前时间的。
	// 如果cacheStart <= t < cacheEnd, lookup可以返回cacheZone。
	// cacheStart和cacheEnd的单位是自1970年1月1日UTC以来的秒数，以配合lookup的参数。
	cacheStart int64
	cacheEnd   int64
	cacheZone  *zone
}

// A zone represents a single time zone such as CET.
// 一个区代表一个单一的时区，如CET。
type zone struct {
	name   string // abbreviated name, "CET",缩写名称，"CET"
	offset int    // seconds east of UTC, 在UTC以东的秒数
	isDST  bool   // is this zone Daylight Savings Time? 这个区是夏令时吗？
}

// A zoneTrans represents a single time zone transition.
// 一个zoneTrans表示一个单一的时区转换。
type zoneTrans struct {
	// 过渡时间，自格林尼治标准时间1970年起，以秒为单位
	when         int64 // transition time, in seconds since 1970 GMT
	// 届时生效的该区的指数
	index        uint8 // the index of the zone that goes into effect at that time
	// 无视 - 不知道这些是什么意思
	isstd, isutc bool  // ignored - no idea what these mean
}

// alpha and omega are the beginning and end of time for zone
// transitions.
// alpha和omega是区域转换的时间起点和终点。
const (
	alpha = -1 << 63  // math.MinInt64
	omega = 1<<63 - 1 // math.MaxInt64
)

// UTC represents Universal Coordinated Time (UTC).
// UTC代表世界协调时间（UTC）。
var UTC *Location = &utcLoc

// utcLoc is separate so that get can refer to &utcLoc
// and ensure that it never returns a nil *Location,
// even if a badly behaved client has changed UTC.
// utcLoc是独立的，这样get就可以引用&utcLoc，
// 并确保它永远不会返回一个nil *Location，
// 即使一个行为不良的客户端改变了UTC。
var utcLoc = Location{name: "UTC"}

// Local represents the system's local time zone.
// On Unix systems, Local consults the TZ environment
// variable to find the time zone to use. No TZ means
// use the system default /etc/localtime.
// TZ="" means use UTC.
// TZ="foo" means use file foo in the system timezone directory.
// Local代表系统的本地时区。
// 在Unix系统中，Local查询TZ环境变量来寻找要使用的时区。
// 没有TZ意味着使用系统默认的/etc/localtime。
// TZ=""表示使用UTC。TZ="foo "意味着使用系统时区目录中的文件foo。
var Local *Location = &localLoc

// localLoc is separate so that initLocal can initialize
// it even if a client has changed Local.
// localLoc是独立的，这样即使客户端改变了Local，initLocal也能初始化它。
var localLoc Location
var localOnce sync.Once

func (l *Location) get() *Location {
	if l == nil {
		return &utcLoc
	}
	if l == &localLoc {
		localOnce.Do(initLocal)
	}
	return l
}

// String returns a descriptive name for the time zone information,
// corresponding to the name argument to LoadLocation or FixedZone.
// String返回时区信息的描述性名称，对应于LoadLocation或FixedZone的名称参数。
func (l *Location) String() string {
	return l.get().name
}

// FixedZone returns a Location that always uses
// the given zone name and offset (seconds east of UTC).
// FixedZone返回一个Location，它总是使用给定的区域名称和偏移量（UTC以东的秒数）。
func FixedZone(name string, offset int) *Location {
	l := &Location{
		name:       name,
		zone:       []zone{{name, offset, false}},
		tx:         []zoneTrans{{alpha, 0, false, false}},
		cacheStart: alpha,
		cacheEnd:   omega,
	}
	l.cacheZone = &l.zone[0]
	return l
}

// lookup returns information about the time zone in use at an
// instant in time expressed as seconds since January 1, 1970 00:00:00 UTC.
//
// The returned information gives the name of the zone (such as "CET"),
// the start and end times bracketing sec when that zone is in effect,
// the offset in seconds east of UTC (such as -5*60*60), and whether
// the daylight savings is being observed at that time.
func (l *Location) lookup(sec int64) (name string, offset int, start, end int64) {
	l = l.get()

	if len(l.zone) == 0 {
		name = "UTC"
		offset = 0
		start = alpha
		end = omega
		return
	}

	if zone := l.cacheZone; zone != nil && l.cacheStart <= sec && sec < l.cacheEnd {
		name = zone.name
		offset = zone.offset
		start = l.cacheStart
		end = l.cacheEnd
		return
	}

	if len(l.tx) == 0 || sec < l.tx[0].when {
		zone := &l.zone[l.lookupFirstZone()]
		name = zone.name
		offset = zone.offset
		start = alpha
		if len(l.tx) > 0 {
			end = l.tx[0].when
		} else {
			end = omega
		}
		return
	}

	// Binary search for entry with largest time <= sec.
	// Not using sort.Search to avoid dependencies.
	tx := l.tx
	end = omega
	lo := 0
	hi := len(tx)
	for hi-lo > 1 {
		m := lo + (hi-lo)/2
		lim := tx[m].when
		if sec < lim {
			end = lim
			hi = m
		} else {
			lo = m
		}
	}
	zone := &l.zone[tx[lo].index]
	name = zone.name
	offset = zone.offset
	start = tx[lo].when
	// end = maintained during the search

	// If we're at the end of the known zone transitions,
	// try the extend string.
	if lo == len(tx)-1 && l.extend != "" {
		if ename, eoffset, estart, eend, ok := tzset(l.extend, end, sec); ok {
			return ename, eoffset, estart, eend
		}
	}

	return
}

// lookupFirstZone returns the index of the time zone to use for times
// before the first transition time, or when there are no transition
// times.
//
// The reference implementation in localtime.c from
// https://www.iana.org/time-zones/repository/releases/tzcode2013g.tar.gz
// implements the following algorithm for these cases:
// 1) If the first zone is unused by the transitions, use it.
// 2) Otherwise, if there are transition times, and the first
//    transition is to a zone in daylight time, find the first
//    non-daylight-time zone before and closest to the first transition
//    zone.
// 3) Otherwise, use the first zone that is not daylight time, if
//    there is one.
// 4) Otherwise, use the first zone.
func (l *Location) lookupFirstZone() int {
	// Case 1.
	if !l.firstZoneUsed() {
		return 0
	}

	// Case 2.
	if len(l.tx) > 0 && l.zone[l.tx[0].index].isDST {
		for zi := int(l.tx[0].index) - 1; zi >= 0; zi-- {
			if !l.zone[zi].isDST {
				return zi
			}
		}
	}

	// Case 3.
	for zi := range l.zone {
		if !l.zone[zi].isDST {
			return zi
		}
	}

	// Case 4.
	return 0
}

// firstZoneUsed reports whether the first zone is used by some
// transition.
// firstZoneUsed报告第一个区是否被某个过渡所使用。
func (l *Location) firstZoneUsed() bool {
	for _, tx := range l.tx {
		if tx.index == 0 {
			return true
		}
	}
	return false
}

// tzset takes a timezone string like the one found in the TZ environment
// variable, the end of the last time zone transition expressed as seconds
// since January 1, 1970 00:00:00 UTC, and a time expressed the same way.
// We call this a tzset string since in C the function tzset reads TZ.
// The return values are as for lookup, plus ok which reports whether the
// parse succeeded.
func tzset(s string, initEnd, sec int64) (name string, offset int, start, end int64, ok bool) {
	var (
		stdName, dstName     string
		stdOffset, dstOffset int
	)

	stdName, s, ok = tzsetName(s)
	if ok {
		stdOffset, s, ok = tzsetOffset(s)
	}
	if !ok {
		return "", 0, 0, 0, false
	}

	// The numbers in the tzset string are added to local time to get UTC,
	// but our offsets are added to UTC to get local time,
	// so we negate the number we see here.
	stdOffset = -stdOffset

	if len(s) == 0 || s[0] == ',' {
		// No daylight savings time.
		return stdName, stdOffset, initEnd, omega, true
	}

	dstName, s, ok = tzsetName(s)
	if ok {
		if len(s) == 0 || s[0] == ',' {
			dstOffset = stdOffset + secondsPerHour
		} else {
			dstOffset, s, ok = tzsetOffset(s)
			dstOffset = -dstOffset // as with stdOffset, above
		}
	}
	if !ok {
		return "", 0, 0, 0, false
	}

	if len(s) == 0 {
		// Default DST rules per tzcode.
		s = ",M3.2.0,M11.1.0"
	}
	// The TZ definition does not mention ';' here but tzcode accepts it.
	if s[0] != ',' && s[0] != ';' {
		return "", 0, 0, 0, false
	}
	s = s[1:]

	var startRule, endRule rule
	startRule, s, ok = tzsetRule(s)
	if !ok || len(s) == 0 || s[0] != ',' {
		return "", 0, 0, 0, false
	}
	s = s[1:]
	endRule, s, ok = tzsetRule(s)
	if !ok || len(s) > 0 {
		return "", 0, 0, 0, false
	}

	year, _, _, yday := absDate(uint64(sec+unixToInternal+internalToAbsolute), false)

	ysec := int64(yday*secondsPerDay) + sec%secondsPerDay

	// Compute start of year in seconds since Unix epoch.
	d := daysSinceEpoch(year)
	abs := int64(d * secondsPerDay)
	abs += absoluteToInternal + internalToUnix

	startSec := int64(tzruleTime(year, startRule, stdOffset))
	endSec := int64(tzruleTime(year, endRule, dstOffset))
	if endSec < startSec {
		startSec, endSec = endSec, startSec
		stdName, dstName = dstName, stdName
		stdOffset, dstOffset = dstOffset, stdOffset
	}

	// The start and end values that we return are accurate
	// close to a daylight savings transition, but are otherwise
	// just the start and end of the year. That suffices for
	// the only caller that cares, which is Date.
	if ysec < startSec {
		return stdName, stdOffset, abs, startSec + abs, true
	} else if ysec >= endSec {
		return stdName, stdOffset, endSec + abs, abs + 365*secondsPerDay, true
	} else {
		return dstName, dstOffset, startSec + abs, endSec + abs, true
	}
}

// tzsetName returns the timezone name at the start of the tzset string s,
// and the remainder of s, and reports whether the parsing is OK.
func tzsetName(s string) (string, string, bool) {
	if len(s) == 0 {
		return "", "", false
	}
	if s[0] != '<' {
		for i, r := range s {
			switch r {
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', ',', '-', '+':
				if i < 3 {
					return "", "", false
				}
				return s[:i], s[i:], true
			}
		}
		if len(s) < 3 {
			return "", "", false
		}
		return s, "", true
	} else {
		for i, r := range s {
			if r == '>' {
				return s[1:i], s[i+1:], true
			}
		}
		return "", "", false
	}
}

// tzsetOffset returns the timezone offset at the start of the tzset string s,
// and the remainder of s, and reports whether the parsing is OK.
// The timezone offset is returned as a number of seconds.
func tzsetOffset(s string) (offset int, rest string, ok bool) {
	if len(s) == 0 {
		return 0, "", false
	}
	neg := false
	if s[0] == '+' {
		s = s[1:]
	} else if s[0] == '-' {
		s = s[1:]
		neg = true
	}

	var hours int
	hours, s, ok = tzsetNum(s, 0, 24)
	if !ok {
		return 0, "", false
	}
	off := hours * secondsPerHour
	if len(s) == 0 || s[0] != ':' {
		if neg {
			off = -off
		}
		return off, s, true
	}

	var mins int
	mins, s, ok = tzsetNum(s[1:], 0, 59)
	if !ok {
		return 0, "", false
	}
	off += mins * secondsPerMinute
	if len(s) == 0 || s[0] != ':' {
		if neg {
			off = -off
		}
		return off, s, true
	}

	var secs int
	secs, s, ok = tzsetNum(s[1:], 0, 59)
	if !ok {
		return 0, "", false
	}
	off += secs

	if neg {
		off = -off
	}
	return off, s, true
}

// ruleKind is the kinds of rules that can be seen in a tzset string.
// ruleKind是在tzset字符串中可以看到的规则的种类。
type ruleKind int

const (
	ruleJulian ruleKind = iota
	ruleDOY
	ruleMonthWeekDay
)

// rule is a rule read from a tzset string.
// rule是一个从tzset字符串中读取的规则。
type rule struct {
	kind ruleKind
	day  int
	week int
	mon  int
	time int // transition time
}

// tzsetRule parses a rule from a tzset string.
// It returns the rule, and the remainder of the string, and reports success.
func tzsetRule(s string) (rule, string, bool) {
	var r rule
	if len(s) == 0 {
		return rule{}, "", false
	}
	ok := false
	if s[0] == 'J' {
		var jday int
		jday, s, ok = tzsetNum(s[1:], 1, 365)
		if !ok {
			return rule{}, "", false
		}
		r.kind = ruleJulian
		r.day = jday
	} else if s[0] == 'M' {
		var mon int
		mon, s, ok = tzsetNum(s[1:], 1, 12)
		if !ok || len(s) == 0 || s[0] != '.' {
			return rule{}, "", false

		}
		var week int
		week, s, ok = tzsetNum(s[1:], 1, 5)
		if !ok || len(s) == 0 || s[0] != '.' {
			return rule{}, "", false
		}
		var day int
		day, s, ok = tzsetNum(s[1:], 0, 6)
		if !ok {
			return rule{}, "", false
		}
		r.kind = ruleMonthWeekDay
		r.day = day
		r.week = week
		r.mon = mon
	} else {
		var day int
		day, s, ok = tzsetNum(s, 0, 365)
		if !ok {
			return rule{}, "", false
		}
		r.kind = ruleDOY
		r.day = day
	}

	if len(s) == 0 || s[0] != '/' {
		r.time = 2 * secondsPerHour // 2am is the default
		return r, s, true
	}

	offset, s, ok := tzsetOffset(s[1:])
	if !ok || offset < 0 {
		return rule{}, "", false
	}
	r.time = offset

	return r, s, true
}

// tzsetNum parses a number from a tzset string.
// It returns the number, and the remainder of the string, and reports success.
// The number must be between min and max.
func tzsetNum(s string, min, max int) (num int, rest string, ok bool) {
	if len(s) == 0 {
		return 0, "", false
	}
	num = 0
	for i, r := range s {
		if r < '0' || r > '9' {
			if i == 0 || num < min {
				return 0, "", false
			}
			return num, s[i:], true
		}
		num *= 10
		num += int(r) - '0'
		if num > max {
			return 0, "", false
		}
	}
	if num < min {
		return 0, "", false
	}
	return num, "", true
}

// tzruleTime takes a year, a rule, and a timezone offset,
// and returns the number of seconds since the start of the year
// that the rule takes effect.
func tzruleTime(year int, r rule, off int) int {
	var s int
	switch r.kind {
	case ruleJulian:
		s = (r.day - 1) * secondsPerDay
		if isLeap(year) && r.day >= 60 {
			s += secondsPerDay
		}
	case ruleDOY:
		s = r.day * secondsPerDay
	case ruleMonthWeekDay:
		// Zeller's Congruence.
		m1 := (r.mon+9)%12 + 1
		yy0 := year
		if r.mon <= 2 {
			yy0--
		}
		yy1 := yy0 / 100
		yy2 := yy0 % 100
		dow := ((26*m1-2)/10 + 1 + yy2 + yy2/4 + yy1/4 - 2*yy1) % 7
		if dow < 0 {
			dow += 7
		}
		// Now dow is the day-of-week of the first day of r.mon.
		// Get the day-of-month of the first "dow" day.
		d := r.day - dow
		if d < 0 {
			d += 7
		}
		for i := 1; i < r.week; i++ {
			if d+7 >= daysIn(Month(r.mon), year) {
				break
			}
			d += 7
		}
		d += int(daysBefore[r.mon-1])
		if isLeap(year) && r.mon > 2 {
			d++
		}
		s = d * secondsPerDay
	}

	return s + r.time - off
}

// lookupName returns information about the time zone with
// the given name (such as "EST") at the given pseudo-Unix time
// (what the given time of day would be in UTC).
func (l *Location) lookupName(name string, unix int64) (offset int, ok bool) {
	l = l.get()

	// First try for a zone with the right name that was actually
	// in effect at the given time. (In Sydney, Australia, both standard
	// and daylight-savings time are abbreviated "EST". Using the
	// offset helps us pick the right one for the given time.
	// It's not perfect: during the backward transition we might pick
	// either one.)
	for i := range l.zone {
		zone := &l.zone[i]
		if zone.name == name {
			nam, offset, _, _ := l.lookup(unix - int64(zone.offset))
			if nam == zone.name {
				return offset, true
			}
		}
	}

	// Otherwise fall back to an ordinary name match.
	for i := range l.zone {
		zone := &l.zone[i]
		if zone.name == name {
			return zone.offset, true
		}
	}

	// Otherwise, give up.
	return
}

// NOTE(rsc): Eventually we will need to accept the POSIX TZ environment
// syntax too, but I don't feel like implementing it today.

var errLocation = errors.New("time: invalid location name")

var zoneinfo *string
var zoneinfoOnce sync.Once

// LoadLocation returns the Location with the given name.
// LoadLocation返回具有给定名称的Location。
//
// If the name is "" or "UTC", LoadLocation returns UTC.
// If the name is "Local", LoadLocation returns Local.
// 如果名称是""或 "UTC"，LoadLocation返回UTC。
// 如果名称是 "Local"，LoadLocation返回Local。
//
// Otherwise, the name is taken to be a location name corresponding to a file
// in the IANA Time Zone database, such as "America/New_York".
// 否则，该名称将被视为与IANA时区数据库中的文件相对应的位置名称，如 "America/New_York"。
//
// The time zone database needed by LoadLocation may not be
// present on all systems, especially non-Unix systems.
// LoadLocation所需要的时区数据库可能并不存在于所有的系统中，尤其是非Unix系统。
// LoadLocation looks in the directory or uncompressed zip file
// named by the ZONEINFO environment variable, if any, then looks in
// known installation locations on Unix systems,
// and finally looks in $GOROOT/lib/time/zoneinfo.zip.
// LoadLocation在ZONEINFO环境变量命名的目录或未压缩的压缩文件中寻找，如果有的话，
// 然后在Unix系统中寻找已知的安装位置，最后在$GOROOT/lib/time/zoneinfo.zip中寻找。
func LoadLocation(name string) (*Location, error) {
	if name == "" || name == "UTC" {
		return UTC, nil
	}
	if name == "Local" {
		return Local, nil
	}
	if containsDotDot(name) || name[0] == '/' || name[0] == '\\' {
		// No valid IANA Time Zone name contains a single dot,
		// much less dot dot. Likewise, none begin with a slash.
		// 没有一个有效的IANA时区名称包含一个点，更不用说点点了。同样，也没有一个是以斜线开头的。
		return nil, errLocation
	}
	zoneinfoOnce.Do(func() {
		env, _ := syscall.Getenv("ZONEINFO")
		zoneinfo = &env
	})
	var firstErr error
	if *zoneinfo != "" {
		if zoneData, err := loadTzinfoFromDirOrZip(*zoneinfo, name); err == nil {
			if z, err := LoadLocationFromTZData(name, zoneData); err == nil {
				return z, nil
			}
			firstErr = err
		} else if err != syscall.ENOENT {
			firstErr = err
		}
	}
	if z, err := loadLocation(name, zoneSources); err == nil {
		return z, nil
	} else if firstErr == nil {
		firstErr = err
	}
	return nil, firstErr
}

// containsDotDot reports whether s contains "..".
// containsDotDot报告s是否包含".."。
func containsDotDot(s string) bool {
	if len(s) < 2 {
		return false
	}
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '.' && s[i+1] == '.' {
			return true
		}
	}
	return false
}
