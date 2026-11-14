//go:build linux

package buildtagstructs

import (
	"github.com/example/buildtagstructs/dw"
	item "github.com/example/buildtagstructs/lx"
)

type W dw.Item

type WA = dw.Item

type WI item.Item

type Two dw.Item

type Loc struct {
	K int `json:"k"`
}

type Same dw.Item
