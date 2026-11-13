//go:build !linux

package buildtagstructs

import (
	"github.com/example/buildtagstructs/dw"
	item "github.com/example/buildtagstructs/dw"
	"github.com/example/buildtagstructs/lx"
)

type W lx.Item

type WA = lx.Item

type WI item.Item

type Two dw.Other

type Loc dw.Item

// Same reaches dw.Item in both builds, so its copies agree.
type Same = dw.Item
