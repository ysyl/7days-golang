package gee

import (
	"errors"
	"strings"
)

type Trie struct {
	root *node
}

func NewTrie() *Trie {
	return &Trie{root: &node{childs: make(map[string]*node)}}
}

func (t Trie) Insert(path string, handler HandlerFunc, options *Options) {
	t.root.insert(strings.Split(path, "/")[1:], handler, options)
}

func (t Trie) Search(method string, path string) (HandlerFunc, error) {
	ctx := &Context{Path: path}
	return t.root.Search(method, strings.Split(path, "/")[1:], ctx)
}

type Options map[string]string

type node struct {
	path    string
	handler HandlerFunc
	options *Options
	childs  map[string]*node
}

func newNode(param string, handler HandlerFunc, options *Options) *node {
	return &node{path: param, handler: handler, childs: make(map[string]*node), options: options}
}

func (n *node) insert(part []string, handler HandlerFunc, options *Options) {
	if len(part) == 0 {
		n.handler = handler
		return
	}

	child, ok := n.childs[part[0]]
	if !ok {
		child = newNode(part[0], nil, options)
		n.childs[part[0]] = child
	}
	child.insert(part[1:], handler, options)
	return
}

func (n *node) Search(method string, path []string, ctx *Context) (HandlerFunc, error) {
	ctx.Params = make(map[string]string)
	resultNode, err := n.recurSearch(method, path, ctx)
	if err != nil {
		return nil, err
	}

	if resultNode.handler == nil {
		return nil, errors.New("handler not found")
	}

	handler := func(c *Context) {
		c.Params = ctx.Params
		c.Path = ctx.Path
		resultNode.handler(c)
	}

	return handler, nil
}

func (n *node) recurSearch(method string, pathList []string, ctx *Context) (*node, error) {
	if len(pathList) == 0 {
		return n, nil
	}
	if strings.HasPrefix(pathList[0], "*") {
	}

	child, ok := n.childs[pathList[0]]
	// 判断方法
	if ok && (*child.options)["Method"] != method {
		return nil, errors.New("405, method is not supported")
	}
	// 找不到匹配参数，寻找参数化路径
	if ok {
		return child.recurSearch(method, pathList[1:], ctx)
	}
	for path, child := range n.childs {
		// 存在:匹配参数，然后将参数填进去
		if strings.HasPrefix(path, ":") {
			// 提取参数，设置到上下文
			paramName := strings.Split(path, ":")[1]
			paramValue := pathList[0]
			ctx.Params[paramName] = paramValue
			return child.recurSearch(method, pathList[1:], ctx)
		}
		// 存在*匹配参数，这时将req对应的后续路径全部塞入参数中
		if strings.HasPrefix(path, "*") {
			paramName := strings.Split(path, "*")[1]
			paramValue := strings.Join(pathList, "/")
			ctx.Params[paramName] = paramValue
			return child, nil
		}
	}
	return nil, errors.New("error")
}
