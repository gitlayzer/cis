package main

import (
	"cis/pkg/cmd"
	"fmt"
)

func main() {
	if err := cmd.Execute(); err != nil {
		panic(fmt.Sprintf("执行命令失败: %v", err))
	}
}
