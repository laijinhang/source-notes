#!/usr/bin/env bash
# Copyright 2009 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# set -e 的作用是在后面脚本出现错误的时候，就不继续执行下去
set -e
# 如果make.bash文件不存在，或者不是一个普通文件，则输出echo 后面的内容
if [ ! -f make.bash ]; then
  # 0（stdin，标准输入）、1（stdout，标准输出）、2（stderr，标准错误输出）
  # &是文件描述符，&2 表示错误通道2，echo 内容 1>&2 表示把 内容 重定向输出到错误通道2
	echo 'all.bash must be run from $GOROOT/src' 1>&2
	exit 1
fi
OLDPATH="$PATH"
. ./make.bash "$@" --no-banner
bash run.bash --no-rebuild
PATH="$OLDPATH"
$GOTOOLDIR/dist banner  # print build info
