// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build aix darwin dragonfly freebsd linux netbsd openbsd solaris

package net

import (
	"context"
	"internal/bytealg"
	"sync"
	"syscall"

	"golang.org/x/net/dns/dnsmessage"
)

var onceReadProtocols sync.Once

// readProtocols loads contents of /etc/protocols into protocols map
// for quick access.
// readProtocols 将 /etc/protocols 的内容加载到 protocols map 中，以便快速访问。
/*
	针对 /etc/protocols（linux） 文件的格式数据，解析出内容，将其加载到 protocols map 中
*/
func readProtocols() {
	file, err := open("/etc/protocols")
	if err != nil {
		return
	}
	defer file.close()

	for line, ok := file.readLine(); ok; line, ok = file.readLine() {
		// tcp    6   TCP    # transmission control protocol
		/*
			比如文件中有一行：tcp    6   TCP    # transmission control protocol
			其数据：tcp\t6\tTCP\t# transmission control protocol

			if i := bytealg.IndexByteString(line, '#'); i >= 0 {
				line = line[0:i]
			}
			之后line=tcp\t6\tTCP\t"

			f := getFields(line)
			f的结果[]string{"tcp","6","TCP"}

			这个for循环结束后得到的结果
			protocols["tcp"] = 6
			protocols["TCP"] = 6
		*/
		/*
			bytealg.IndexByteString(line, '#')表示字符串line中'#'第一次出现的位置，汇编实现
		*/
		if i := bytealg.IndexByteString(line, '#'); i >= 0 {
			line = line[0:i]
		}
		/*
			getFields(line)对字符串line按 \r\t\n 中任何一个byte进行分割
			比如line="123\t\n23\r12\n"，经过getFields(line)后得到的结果
			[]string{"123","23","12"}
		*/
		f := getFields(line)
		if len(f) < 2 {
			continue
		}
		/*
			dtoi(f[1]) 字符串转成数字
		*/
		if proto, _, ok := dtoi(f[1]); ok {
			if _, ok := protocols[f[0]]; !ok {
				protocols[f[0]] = proto
			}
			for _, alias := range f[2:] {
				if _, ok := protocols[alias]; !ok {
					protocols[alias] = proto
				}
			}
		}
	}
}

// lookupProtocol looks up IP protocol name in /etc/protocols and
// returns correspondent protocol number.
func lookupProtocol(_ context.Context, name string) (int, error) {
	onceReadProtocols.Do(readProtocols)
	return lookupProtocolMap(name)
}

func (r *Resolver) dial(ctx context.Context, network, server string) (Conn, error) {
	// Calling Dial here is scary -- we have to be sure not to
	// dial a name that will require a DNS lookup, or Dial will
	// call back here to translate it. The DNS config parser has
	// already checked that all the cfg.servers are IP
	// addresses, which Dial will use without a DNS lookup.
	var c Conn
	var err error
	if r != nil && r.Dial != nil {
		c, err = r.Dial(ctx, network, server)
	} else {
		var d Dialer
		c, err = d.DialContext(ctx, network, server)
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return c, nil
}

func (r *Resolver) lookupHost(ctx context.Context, host string) (addrs []string, err error) {
	order := systemConf().hostLookupOrder(r, host)
	if !r.preferGo() && order == hostLookupCgo {
		if addrs, err, ok := cgoLookupHost(ctx, host); ok {
			return addrs, err
		}
		// cgo not available (or netgo); fall back to Go's DNS resolver
		order = hostLookupFilesDNS
	}
	return r.goLookupHostOrder(ctx, host, order)
}

func (r *Resolver) lookupIP(ctx context.Context, network, host string) (addrs []IPAddr, err error) {
	if r.preferGo() {
		return r.goLookupIP(ctx, host)
	}
	order := systemConf().hostLookupOrder(r, host)
	if order == hostLookupCgo {
		if addrs, err, ok := cgoLookupIP(ctx, network, host); ok {
			return addrs, err
		}
		// cgo not available (or netgo); fall back to Go's DNS resolver
		order = hostLookupFilesDNS
	}
	ips, _, err := r.goLookupIPCNAMEOrder(ctx, host, order)
	return ips, err
}

func (r *Resolver) lookupPort(ctx context.Context, network, service string) (int, error) {
	if !r.preferGo() && systemConf().canUseCgo() {
		if port, err, ok := cgoLookupPort(ctx, network, service); ok {
			if err != nil {
				// Issue 18213: if cgo fails, first check to see whether we
				// have the answer baked-in to the net package.
				if port, err := goLookupPort(network, service); err == nil {
					return port, nil
				}
			}
			return port, err
		}
	}
	return goLookupPort(network, service)
}

func (r *Resolver) lookupCNAME(ctx context.Context, name string) (string, error) {
	if !r.preferGo() && systemConf().canUseCgo() {
		if cname, err, ok := cgoLookupCNAME(ctx, name); ok {
			return cname, err
		}
	}
	return r.goLookupCNAME(ctx, name)
}

func (r *Resolver) lookupSRV(ctx context.Context, service, proto, name string) (string, []*SRV, error) {
	var target string
	if service == "" && proto == "" {
		target = name
	} else {
		target = "_" + service + "._" + proto + "." + name
	}
	p, server, err := r.lookup(ctx, target, dnsmessage.TypeSRV)
	if err != nil {
		return "", nil, err
	}
	var srvs []*SRV
	var cname dnsmessage.Name
	for {
		h, err := p.AnswerHeader()
		if err == dnsmessage.ErrSectionDone {
			break
		}
		if err != nil {
			return "", nil, &DNSError{
				Err:    "cannot unmarshal DNS message",
				Name:   name,
				Server: server,
			}
		}
		if h.Type != dnsmessage.TypeSRV {
			if err := p.SkipAnswer(); err != nil {
				return "", nil, &DNSError{
					Err:    "cannot unmarshal DNS message",
					Name:   name,
					Server: server,
				}
			}
			continue
		}
		if cname.Length == 0 && h.Name.Length != 0 {
			cname = h.Name
		}
		srv, err := p.SRVResource()
		if err != nil {
			return "", nil, &DNSError{
				Err:    "cannot unmarshal DNS message",
				Name:   name,
				Server: server,
			}
		}
		srvs = append(srvs, &SRV{Target: srv.Target.String(), Port: srv.Port, Priority: srv.Priority, Weight: srv.Weight})
	}
	byPriorityWeight(srvs).sort()
	return cname.String(), srvs, nil
}

func (r *Resolver) lookupMX(ctx context.Context, name string) ([]*MX, error) {
	p, server, err := r.lookup(ctx, name, dnsmessage.TypeMX)
	if err != nil {
		return nil, err
	}
	var mxs []*MX
	for {
		h, err := p.AnswerHeader()
		if err == dnsmessage.ErrSectionDone {
			break
		}
		if err != nil {
			return nil, &DNSError{
				Err:    "cannot unmarshal DNS message",
				Name:   name,
				Server: server,
			}
		}
		if h.Type != dnsmessage.TypeMX {
			if err := p.SkipAnswer(); err != nil {
				return nil, &DNSError{
					Err:    "cannot unmarshal DNS message",
					Name:   name,
					Server: server,
				}
			}
			continue
		}
		mx, err := p.MXResource()
		if err != nil {
			return nil, &DNSError{
				Err:    "cannot unmarshal DNS message",
				Name:   name,
				Server: server,
			}
		}
		mxs = append(mxs, &MX{Host: mx.MX.String(), Pref: mx.Pref})

	}
	byPref(mxs).sort()
	return mxs, nil
}

func (r *Resolver) lookupNS(ctx context.Context, name string) ([]*NS, error) {
	p, server, err := r.lookup(ctx, name, dnsmessage.TypeNS)
	if err != nil {
		return nil, err
	}
	var nss []*NS
	for {
		h, err := p.AnswerHeader()
		if err == dnsmessage.ErrSectionDone {
			break
		}
		if err != nil {
			return nil, &DNSError{
				Err:    "cannot unmarshal DNS message",
				Name:   name,
				Server: server,
			}
		}
		if h.Type != dnsmessage.TypeNS {
			if err := p.SkipAnswer(); err != nil {
				return nil, &DNSError{
					Err:    "cannot unmarshal DNS message",
					Name:   name,
					Server: server,
				}
			}
			continue
		}
		ns, err := p.NSResource()
		if err != nil {
			return nil, &DNSError{
				Err:    "cannot unmarshal DNS message",
				Name:   name,
				Server: server,
			}
		}
		nss = append(nss, &NS{Host: ns.NS.String()})
	}
	return nss, nil
}

func (r *Resolver) lookupTXT(ctx context.Context, name string) ([]string, error) {
	p, server, err := r.lookup(ctx, name, dnsmessage.TypeTXT)
	if err != nil {
		return nil, err
	}
	var txts []string
	for {
		h, err := p.AnswerHeader()
		if err == dnsmessage.ErrSectionDone {
			break
		}
		if err != nil {
			return nil, &DNSError{
				Err:    "cannot unmarshal DNS message",
				Name:   name,
				Server: server,
			}
		}
		if h.Type != dnsmessage.TypeTXT {
			if err := p.SkipAnswer(); err != nil {
				return nil, &DNSError{
					Err:    "cannot unmarshal DNS message",
					Name:   name,
					Server: server,
				}
			}
			continue
		}
		txt, err := p.TXTResource()
		if err != nil {
			return nil, &DNSError{
				Err:    "cannot unmarshal DNS message",
				Name:   name,
				Server: server,
			}
		}
		// Multiple strings in one TXT record need to be
		// concatenated without separator to be consistent
		// with previous Go resolver.
		n := 0
		for _, s := range txt.TXT {
			n += len(s)
		}
		txtJoin := make([]byte, 0, n)
		for _, s := range txt.TXT {
			txtJoin = append(txtJoin, s...)
		}
		if len(txts) == 0 {
			txts = make([]string, 0, 1)
		}
		txts = append(txts, string(txtJoin))
	}
	return txts, nil
}

func (r *Resolver) lookupAddr(ctx context.Context, addr string) ([]string, error) {
	if !r.preferGo() && systemConf().canUseCgo() {
		if ptrs, err, ok := cgoLookupPTR(ctx, addr); ok {
			return ptrs, err
		}
	}
	return r.goLookupPTR(ctx, addr)
}

// concurrentThreadsLimit returns the number of threads we permit to
// run concurrently doing DNS lookups via cgo. A DNS lookup may use a
// file descriptor so we limit this to less than the number of
// permitted open files. On some systems, notably Darwin, if
// getaddrinfo is unable to open a file descriptor it simply returns
// EAI_NONAME rather than a useful error. Limiting the number of
// concurrent getaddrinfo calls to less than the permitted number of
// file descriptors makes that error less likely. We don't bother to
// apply the same limit to DNS lookups run directly from Go, because
// there we will return a meaningful "too many open files" error.
// concurrentThreadsLimit返回我们允许通过cgo并发运行DNS查询的线程数。
// 一个DNS查询可能会使用一个文件描述符，所以我们将其限制在小于允许打开的文件数量。
// 在某些系统上，特别是Darwin，如果getaddrinfo不能打开一个文件描述符，它只是
// 返回EAI_NONAME而不是一个有用的错误。限制并发的getaddrinfo调用的数量，使其
// 少于允许的文件描述符的数量，可以减少这种错误的发生。我们懒得对直接从Go运行的
// DNS查询应用同样的限制，因为在那里我们会返回一个有意义的 "太多打开的文件 "的错误。
/*
	1、先通过系统调用拿到进程能够打开的最大文件描述数，如果读取中有错误返回，则返回限制为500
	2、如果进程能够打开的最大文件描述符超过500，则返回500
	3、如果进程能够打开的最大文件描述符数小于等于500，并且大于30，则返回这个基础上减去30
*/
func concurrentThreadsLimit() int {
	var rlim syscall.Rlimit
	// 读取进程能打开的最大文件描述符数，并把它放入到rlim变量里
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlim); err != nil {
		return 500
	}
	r := int(rlim.Cur)
	if r > 500 {
		r = 500
	} else if r > 30 {
		r -= 30
	}
	return r
}
