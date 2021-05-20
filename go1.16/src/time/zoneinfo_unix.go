// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build aix darwin,!ios dragonfly freebsd linux,!android netbsd openbsd solaris

// Parse "zoneinfo" time zone file.
// This is a fairly standard file format used on OS X, Linux, BSD, Sun, and others.
// See tzfile(5), https://en.wikipedia.org/wiki/Zoneinfo,
// and ftp://munnari.oz.au/pub/oldtz/

package time

import (
	"runtime"
	"syscall"
)

// Many systems use /usr/share/zoneinfo, Solaris 2 has
// /usr/share/lib/zoneinfo, IRIX 6 has /usr/lib/locale/TZ.
// 许多系统使用/usr/share/zoneinfo，Solaris 2有
// /usr/share/lib/zoneinfo，IRIX 6有/usr/lib/locale/TZ。
var zoneSources = []string{
	"/usr/share/zoneinfo/",
	"/usr/share/lib/zoneinfo/",
	"/usr/lib/locale/TZ/",
	runtime.GOROOT() + "/lib/time/zoneinfo.zip",
}

func initLocal() {
	// consult $TZ to find the time zone to use.
	// no $TZ means use the system default /etc/localtime.
	// $TZ="" means use UTC.
	// $TZ="foo" or $TZ=":foo" if foo is an absolute path, then the file pointed
	// by foo will be used to initialize timezone; otherwise, file
	// /usr/share/zoneinfo/foo will be used.
	// 查阅$TZ以找到要使用的时区。没有$TZ意味着使用系统默认的/etc/localtime。
	// $TZ=""表示使用UTC。$TZ="foo "或$TZ=":foo "如果foo是一个绝对路径，
	// 那么foo所指向的文件将被用来初始化时区；否则，将使用文件/usr/share/zoneinfo/foo。

	tz, ok := syscall.Getenv("TZ")
	switch {
	case !ok:
		z, err := loadLocation("localtime", []string{"/etc"})
		if err == nil {
			localLoc = *z
			localLoc.name = "Local"
			return
		}
	case tz != "":
		if tz[0] == ':' {
			tz = tz[1:]
		}
		if tz != "" && tz[0] == '/' {
			if z, err := loadLocation(tz, []string{""}); err == nil {
				localLoc = *z
				if tz == "/etc/localtime" {
					localLoc.name = "Local"
				} else {
					localLoc.name = tz
				}
				return
			}
		} else if tz != "" && tz != "UTC" {
			if z, err := loadLocation(tz, zoneSources); err == nil {
				localLoc = *z
				return
			}
		}
	}

	// Fall back to UTC.
	localLoc.name = "UTC"
}
