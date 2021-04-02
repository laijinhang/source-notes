// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build aix darwin dragonfly freebsd js,wasm linux netbsd openbsd solaris plan9

// Unix environment variables.
// UNIX环境变量。

/*
一共向外部暴露了五个接口
1. 通过key获取环境变量
2. 通过key删除环境变量
3. 设置环境变量
4. 获取全部环境变量
5. 删除全部环境变量

使用读写锁来保证并发安全
使用envs（[]string）来获取到系统里的环境变量
使用env进行映射key=>环境变量值，建立映射关系并不会在初始化时就建立好，
而是经过在第一次调用环境变量进行初始化，对于多次key一样的，只会以第一次的为准，并会从envs中将其删除
*/

package syscall

import (
	"runtime"
	"sync"
)

var (
	// envOnce guards initialization by copyenv, which populates env.
	// envOnce保护copyenv的初始化，后者会填充env。
	/*
		这个变量只在这个文件中被使用了，一共五个地方，并且都是执行envOnce.Do(copyenv)，也就是让copyenv函数只被执行一次
	*/
	envOnce sync.Once

	// envLock guards env and envs.
	// envLock保护env和envs。
	envLock sync.RWMutex

	// env maps from an environment variable to its first occurrence in envs.
	// env从一个环境变量映射到它在envs中的第一次出现。
	env map[string]int

	// envs is provided by the runtime. elements are expected to
	// be of the form "key=value". An empty string means deleted
	// (or a duplicate to be ignored).
	envs []string = runtime_envs()
)

func runtime_envs() []string // in package runtime

// setenv_c and unsetenv_c are provided by the runtime but are no-ops
// if cgo isn't loaded.
// setenv_c和unsetenv_c是运行时提供的，但如果cgo没有加载，它们是无操作的
func setenv_c(k, v string)
func unsetenv_c(k string)

/*
这个函数只被这个文件中使用到了，一共五个地方

它把环境变量[]string的环境变量进行解析，以key=>val的方式存储到env中，但有多个出现一样的key时，
只取第一个到env，并把后面出现的重复key从envs中清除掉
*/
func copyenv() {
	env = make(map[string]int)
	for i, s := range envs {
		for j := 0; j < len(s); j++ {
			if s[j] == '=' {
				key := s[:j]
				if _, ok := env[key]; !ok {
					// 对于多次一样的key，只取第一次出现的
					env[key] = i // first mention of key
				} else {
					// Clear duplicate keys. This permits Unsetenv to
					// safely delete only the first item without
					// worrying about unshadowing a later one,
					// which might be a security problem.
					// 清除重复的key。这允许Unsetenv仅安全地删除第一个项目，
					// 而不必担心取消后一个项目的阴影，这可能会出现安全问题。
					envs[i] = ""
				}
				break
			}
		}
	}
}

func Unsetenv(key string) error {
	envOnce.Do(copyenv)

	envLock.Lock()
	defer envLock.Unlock()

	if i, ok := env[key]; ok {
		envs[i] = ""
		delete(env, key)
	}
	unsetenv_c(key)
	return nil
}

func Getenv(key string) (value string, found bool) {
	envOnce.Do(copyenv)
	if len(key) == 0 {
		return "", false
	}

	envLock.RLock()
	defer envLock.RUnlock()

	i, ok := env[key]
	if !ok {
		return "", false
	}
	s := envs[i]
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return s[i+1:], true
		}
	}
	return "", false
}

func Setenv(key, value string) error {
	envOnce.Do(copyenv)
	if len(key) == 0 {
		return EINVAL
	}
	for i := 0; i < len(key); i++ {
		if key[i] == '=' || key[i] == 0 {
			return EINVAL
		}
	}
	// On Plan 9, null is used as a separator, eg in $path.
	if runtime.GOOS != "plan9" {
		for i := 0; i < len(value); i++ {
			if value[i] == 0 {
				return EINVAL
			}
		}
	}

	envLock.Lock()
	defer envLock.Unlock()

	i, ok := env[key]
	kv := key + "=" + value
	if ok {
		envs[i] = kv
	} else {
		i = len(envs)
		envs = append(envs, kv)
	}
	env[key] = i
	setenv_c(key, value)
	return nil
}

func Clearenv() {
	envOnce.Do(copyenv) // prevent copyenv in Getenv/Setenv

	envLock.Lock()
	defer envLock.Unlock()

	for k := range env {
		unsetenv_c(k)
	}
	env = make(map[string]int)
	envs = []string{}
}

func Environ() []string {
	envOnce.Do(copyenv)
	envLock.RLock()
	defer envLock.RUnlock()
	a := make([]string, 0, len(envs))
	for _, env := range envs {
		if env != "" {
			a = append(a, env)
		}
	}
	return a
}
