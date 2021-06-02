// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http

import (
	"net/url"
)

// A CookieJar manages storage and use of cookies in HTTP requests.
// CookieJar管理HTTP请求中cookie的存储和使用。
//
// Implementations of CookieJar must be safe for concurrent use by multiple
// goroutines.
// CookieJar的实现必须是安全的，以便多个goroutines并发使用。
//
// The net/http/cookiejar package provides a CookieJar implementation.
//net/http/cookiejar包提供了一个CookieJar实现。
type CookieJar interface {
	// SetCookies handles the receipt of the cookies in a reply for the
	// given URL.  It may or may not choose to save the cookies, depending
	// on the jar's policy and implementation.
	// SetCookies在给定URL的回复中处理cookie的接收。
	// 它可以选择保存cookie，也可以不保存，这取决于JAR的策略和实现。
	SetCookies(u *url.URL, cookies []*Cookie)

	// Cookies returns the cookies to send in a request for the given URL.
	// It is up to the implementation to honor the standard cookie use
	// restrictions such as in RFC 6265.
	// cookie返回在给定网址的请求中发送的cookie。
	// 这取决于实现是否遵守标准cookie使用限制，
	// 如RFC 6265中的限制。
	Cookies(u *url.URL) []*Cookie
}

/*
	cookie jar的作用是将cookie缓存起来，发送一个请求，那么发送完成之后，会将这次的cookie缓存起来，下一次请求的时候，会携带cookie。
	如果一个http客户端需要多次请求，并且需要之前响应的cookie，如登录之后才能访问的。
*/
