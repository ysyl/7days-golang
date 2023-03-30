package gee

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Context struct {
	Writer     http.ResponseWriter
	Req        *http.Request
	Path       string
	Params     map[string]string
	StatusCode HttpStatus
	Before     time.Time
	After      time.Time
}

type HttpStatus int

func (s HttpStatus) Message() string {
	if s == 200 {
		return "success"
	}
	return ""
}

var (
	SUCCESS = HttpStatus(200)
)

func NewContext(writer http.ResponseWriter, r *http.Request) *Context {
	return &Context{Writer: writer, Req: r, Path: r.URL.Path, StatusCode: SUCCESS}
}

func (c *Context) HTML(ok int, tmplPath string, param any) {
}

func (c *Context) String(ok int, s string, i interface{}, path string) {
	c.Writer.Header().Set("Content-Type", "text/plain")
	c.Writer.WriteHeader(ok)
	fmt.Fprintf(c.Writer, s, i, path)
}

func (c *Context) JSON(ok int, i map[string]interface{}) {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(ok)
	marshal, err := json.Marshal(i)
	if err != nil {
		panic(err)
	}
	c.Writer.Write(marshal)
}

func (c *Context) PostForm(s string) string {
	return c.Req.FormValue(s)
}

func (c *Context) Query(s string) string {
	return c.Req.URL.Query().Get(s)
}

func (c *Context) Param(key string) string {
	return c.Params[key]
}

func (c *Context) Fail(i int, s string) {

}
