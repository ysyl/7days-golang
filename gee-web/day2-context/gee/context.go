package gee

import (
    "encoding/json"
    "net/http"
)

type Context struct {
    writer http.ResponseWriter
    req    *http.Request
    Path   string
}

func NewContext(writer http.ResponseWriter, r *http.Request) *Context {
    return &Context{writer: writer, req: r, Path: r.URL.Path}
}

func (c *Context) HTML(ok int, s string) {
    c.writer.Header().Set("Content-Type", "text/html")
    c.writer.WriteHeader(ok)
    c.writer.Write([]byte(s))
}

func (c *Context) String(ok int, s string, i interface{}, path interface{}) {
    c.writer.Header().Set("Content-Type", "text/plain")
    c.writer.WriteHeader(ok)
    c.writer.Write([]byte(s))
}

func (c *Context) JSON(ok int, i map[string]interface{}) {
    c.writer.Header().Set("Content-Type", "application/json")
    c.writer.WriteHeader(ok)
    marshal, err := json.Marshal(i)
    if err != nil {
        panic(err)
    }
    c.writer.Write(marshal)
}

func (c *Context) PostForm(s string) string {
    return c.req.FormValue(s)
}

func (c *Context) Query(s string) string {
    return c.req.URL.Query().Get(s)
}
