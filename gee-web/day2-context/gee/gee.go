package gee

import (
    "fmt"
    "net/http"
)

type H map[string]any

type HandlerFunc func(ctx *Context)

type SimpleEngine struct {
    routers map[string]HandlerFunc
}

func New() *SimpleEngine {
    return &SimpleEngine{routers: map[string]HandlerFunc{}}
}

func (engine *SimpleEngine) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
    method, path := request.Method, request.URL.Path
    key := method + "-" + path
    handlerFunc, ok := engine.routers[key]
    if ok {
        ctx := NewContext(writer, request)
        handlerFunc(ctx)
    } else {
        fmt.Fprintf(writer, "404 not fount, "+key)
    }
}

func (engine SimpleEngine) addRoute(method, path string, handlerFunc HandlerFunc) {
    key := method + "-" + path
    fmt.Println(key)
    engine.routers[key] = handlerFunc
}

func (engine *SimpleEngine) GET(path string, handlerFunc HandlerFunc) {
    engine.addRoute("GET", path, handlerFunc)
}

func (engine *SimpleEngine) POST(path string, handlerFunc HandlerFunc) {
    engine.addRoute("POST", path, handlerFunc)
}

func (engine *SimpleEngine) Run(addr string) error {
    err := http.ListenAndServe(addr, engine)
    return err
}
