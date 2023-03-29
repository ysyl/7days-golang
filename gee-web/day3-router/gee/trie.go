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
	return t.root.Search(method, strings.Split(path, "/")[1:])
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
		return
	}

	child, ok := n.childs[part[0]]
	if !ok {
		child = newNode(part[0], handler, options)
		n.childs[part[0]] = child
	}
	child.insert(part[1:], handler, options)
	return
}

func (n *node) Search(method string, path []string) (HandlerFunc, error) {
	ctx := Context{Params: make(map[string]string)}
	resultNode, err := n.recurSearch(method, path, &ctx)
	if err != nil {
		return nil, err
	}

	handler := func(c *Context) {
		c.Params = ctx.Params
		resultNode.handler(c)
	}

	return handler, nil
}

func (n *node) recurSearch(method string, path []string, ctx *Context) (*node, error) {
	if len(path) == 0 {
		return n, nil
	}
	child, ok := n.childs[path[0]]
	// 判断方法
	if ok && (*child.options)["Method"] != method {
		return nil, errors.New("405, method is not supported")
	}
	// 找不到匹配参数，寻找参数化路径
	if !ok {
		for key, child := range n.childs {
			if strings.HasPrefix(key, ":") {
				// 提取参数，设置到上下文
				paramName := strings.Split(key, ":")[1]
				paramValue := path[0]
				ctx.Params[paramName] = paramValue
				return child.recurSearch(method, path[1:], ctx)
			}
		}
		return nil, errors.New("error")
	}
	return child.recurSearch(method, path[1:], ctx)
}
