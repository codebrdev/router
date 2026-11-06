package router

import (
	"net/http"
	"slices"

	"github.com/codebrdev/middleware"
)

// RouteGroup represents a collection of routes with a common prefix and shared middlewares.
type RouteGroup struct {
	mux         *http.ServeMux
	prefix      string
	middlewares []middleware.Middleware
}

func NewRouteGroup(mux *http.ServeMux, prefix string, middlewares ...middleware.Middleware) *RouteGroup {
	rg := &RouteGroup{
		mux:         mux,
		prefix:      prefix,
		middlewares: slices.Clone(middlewares),
	}

	return rg
}

func (rg *RouteGroup) Handle(method string, path string, handler http.Handler, middlewares ...middleware.Middleware) {
	allMiddlewares := slices.Concat(rg.middlewares, middlewares)

	finalHandler := middleware.Chain(
		allMiddlewares...,
	)(handler)

	// Se o path for exatamente "/", blindamos com o {$} do Go 1.22+
	targetPath := path
	if targetPath == "" || targetPath == "/" {
		targetPath = "/{$}"
	}

	fullPattern := method + " " + rg.prefix + targetPath
	rg.mux.Handle(fullPattern, finalHandler)

	// Com prefixo, "/v1/users" (sem barra final) também deve casar exatamente.
	if rg.prefix != "" && targetPath == "/{$}" {
		rg.mux.Handle(method+" "+rg.prefix, finalHandler)
	}
}

func (rg *RouteGroup) HandleFunc(method string, path string, handlerFunc http.HandlerFunc, middlewares ...middleware.Middleware) {
	rg.Handle(method, path, http.HandlerFunc(handlerFunc), middlewares...)
}
