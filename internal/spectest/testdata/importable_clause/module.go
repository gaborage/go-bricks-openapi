// Package importableclause pins that an in-module package is named after its
// importable files' package clause: b's first file by name is a //go:build
// ignore generator in package main, which neither renames the unaliased import
// nor lends b a declaration.
package importableclause

import (
	"github.com/example/importableclause/b"
	bb "github.com/example/importableclause/b"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "importable" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// WrapL promotes b.Level's text pair: a string both ways.
type WrapL struct {
	b.Level
}

// Embeds promotes b.Base's and b.Addr's fields.
type Embeds struct {
	b.Base
	*b.Addr
	Note string `json:"note"`
}

// Rows holds one field per qualified position.
type Rows struct {
	Addr     b.Addr            `json:"addr"`
	AddrList []b.Addr          `json:"addrList"`
	AddrPtr  *b.Addr           `json:"addrPtr"`
	AddrMap  map[string]b.Addr `json:"addrMap"`
	Cents    b.Cents           `json:"cents"`
	Tags     b.Tags            `json:"tags"`
	Level    b.Level           `json:"level"`
	Money    b.Money           `json:"money"`
	Wrap     WrapL             `json:"wrap"`
	Embeds   Embeds            `json:"embeds"`
	Aliased  bb.Addr           `json:"aliased"`
}

func (m *Module) rows(ctx server.HandlerContext) (server.Result[Rows], server.IAPIError) {
	return server.Result[Rows]{}, nil
}

func (m *Module) getAddr(ctx server.HandlerContext) (server.Result[b.Addr], server.IAPIError) {
	return server.Result[b.Addr]{}, nil
}

func (m *Module) createAddr(req b.Addr, ctx server.HandlerContext) (server.Result[b.Cents], server.IAPIError) {
	return server.Result[b.Cents]{}, nil
}

func (m *Module) wrap(ctx server.HandlerContext) (server.Result[WrapL], server.IAPIError) {
	return server.Result[WrapL]{}, nil
}

func (m *Module) aliased(ctx server.HandlerContext) (server.Result[bb.Addr], server.IAPIError) {
	return server.Result[bb.Addr]{}, nil
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/rows", m.rows)
	server.GET(hr, r, "/addr", m.getAddr)
	server.POST(hr, r, "/addr", m.createAddr)
	server.GET(hr, r, "/wrap", m.wrap)
	server.GET(hr, r, "/aliased", m.aliased)
}
