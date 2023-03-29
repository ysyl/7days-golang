package gee

import (
	"fmt"
	"net/http"
)

type H map[string]any

type HandlerFunc func(ctx *Context)

// Engine Engine是统一的门面
type Engine struct {
	routers *Router
}

// RouterGroup 是分组代理，也有注册方法
type RouterGroup struct {
	prefix string
	engine *Engine
}

type HttpHandlerRegistry interface {
	GET(path string, handler HandlerFunc)
	POST(path string, handler HandlerFunc)
}

func New() *Engine {
	return &Engine{routers: NewRouter()}
}

func (engine *Engine) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	method, path := request.Method, request.URL.Path
	handlerFunc, err := engine.routers.Search(method, path)
	if err == nil {
		ctx := NewContext(writer, request)
		handlerFunc(ctx)
	} else {
		fmt.Fprintf(writer, err.Error())
	}
}

func (engine Engine) addRoute(method, path string, handlerFunc HandlerFunc) {
	engine.routers.AddRouter(method, path, handlerFunc)
}

func (engine *Engine) GET(path string, handlerFunc HandlerFunc) {
	engine.addRoute("GET", path, handlerFunc)
}

func (engine *Engine) POST(path string, handlerFunc HandlerFunc) {
	engine.addRoute("POST", path, handlerFunc)
}

func (engine *Engine) Run(addr string) error {
	err := http.ListenAndServe(addr, engine)
	return err
}

func (engine *Engine) Group(prefix string) *RouterGroup {
	return &RouterGroup{prefix: prefix, engine: engine}
}

func (r *RouterGroup) GET(path string, handler HandlerFunc) {
	r.engine.GET(r.prefix+path, handler)
}

func (r *RouterGroup) POST(path string, handler HandlerFunc) {
	r.engine.POST(r.prefix+path, handler)
}
