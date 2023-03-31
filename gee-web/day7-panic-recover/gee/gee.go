package gee

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"runtime"
	"time"
)

type H map[string]any

type HandlerFunc func(ctx *Context)

// Engine Engine是统一的门面
type Engine struct {
	routers     *Router
	groups      []*RouterGroup
	middlewares []HandlerFunc
	funcMap     template.FuncMap
	template    *template.Template
	statics     []string
}

// RouterGroup 是分组代理，也有注册方法
type RouterGroup struct {
	prefix      string
	engine      *Engine
	middlewares []HandlerFunc
}

type HttpHandlerRegistry interface {
	GET(path string, handler HandlerFunc)
	POST(path string, handler HandlerFunc)
}

func New() *Engine {
	return &Engine{routers: NewRouter()}
}

func Default() *Engine {
	engine := New()
	engine.Use(Logger())
	engine.Use(panicRecover())
	return engine
}

func panicRecover() HandlerFunc {
	return func(ctx *Context) {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				log.Printf("Panic: %+v\n%s", r, buf[:n])
				ctx.Fail(500, "panic")
			}
		}()
		ctx.Next()
	}
}

func (engine *Engine) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	method, path := request.Method, request.URL.Path
	handlerFunc, err := engine.routers.Search(method, path)
	if err != nil {
		fmt.Fprintf(writer, err.Error())
		return
	}
	ctx := NewContext(writer, request)
	ctx.engine = engine
	// 处理中间件
	pushMiddleware(ctx, engine, handlerFunc)
	// 启动
	ctx.Next()
}

func pushMiddleware(ctx *Context, engine *Engine, handlerFunc HandlerFunc) {
	assembleMiddlewares := make([]HandlerFunc, 0)
	// 加入组中间件
	for _, group := range engine.groups {
		assembleMiddlewares = append(assembleMiddlewares, group.middlewares...)
	}
	// 加入global中间件
	assembleMiddlewares = append(assembleMiddlewares, engine.middlewares...)
	assembleMiddlewares = append(assembleMiddlewares, handlerFunc)
	ctx.handlers = assembleMiddlewares
}

// 注册时，实际注册的handlerFunc要包装middlewares
func (engine *Engine) addRoute(method, path string, handlerFunc HandlerFunc) {
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
	r := &RouterGroup{prefix: prefix, engine: engine}
	engine.groups = append(engine.groups, r)
	return r
}

func (r *RouterGroup) GET(path string, handler HandlerFunc) {
	r.engine.GET(r.prefix+path, handler)
}

func (r *RouterGroup) POST(path string, handler HandlerFunc) {
	r.engine.POST(r.prefix+path, handler)
}

func (engine *Engine) Use(middleware HandlerFunc) {
	engine.middlewares = append(engine.middlewares, middleware)
}

func (engine *Engine) SetFuncMap(funcMap template.FuncMap) {
	engine.funcMap = funcMap
}

func (engine *Engine) LoadHTMLGlob(pattern string) {
	tmpl, err := template.New("template").Funcs(engine.funcMap).ParseGlob(pattern)
	if err != nil {
		return
	}
	engine.template = tmpl
}

func (engine *Engine) Static(httpPath, realPath string) {
	engine.GET(httpPath+"/*filename", func(ctx *Context) {
		http.ServeFile(ctx.Writer, ctx.Req, realPath+"/"+ctx.Param("filename"))
	})
}

func (r *RouterGroup) Use(middleware HandlerFunc) {
	r.middlewares = append(r.middlewares, middleware)
}

// 2019/08/17 01:37:38 [200] / in 3.14µs
func Logger() HandlerFunc {
	return func(ctx *Context) {
		before := time.Now()
		ctx.Next()
		duration := time.Now().Sub(before)
		log.Default().Printf("[%d] %s in %s", ctx.StatusCode, ctx.Path, duration.String())
	}
}
