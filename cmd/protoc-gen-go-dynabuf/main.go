package main

import (
	"github.com/bufbuild/protoplugin"
	dynabuf "github.com/picatz/dynabuf/internal"
)

func main() {
	protoplugin.Main(protoplugin.HandlerFunc(dynabuf.Handle))
}
