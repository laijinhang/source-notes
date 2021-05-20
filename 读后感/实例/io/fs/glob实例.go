package main

import (
	"fmt"
	"io/fs"
	"os"
)

// 匹配指定目录下的所有文件/目录
func matchFiles(f fs.FS, pattern string) {
	matches, err := fs.Glob(f, pattern)
	if err != nil {
		fmt.Printf("Glob error for %q: %s", pattern, err)
		return
	}
	fmt.Println(matches)
}

func main() {
	// 当前目录
	matchFiles(os.DirFS("."), "*")
	// 根目录
	matchFiles(os.DirFS("/"), "*")
	// 根目录下r开头
	matchFiles(os.DirFS("/"), "r*")
}
