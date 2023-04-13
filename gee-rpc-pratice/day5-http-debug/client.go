// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package geerpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"geerpc/codec"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Call 客户端一次调用都是一次call
// 内容是方法签名，func (t *T) MethodName(argv interface{}, rtrV interface{}) error
type Call struct {
	Seq           uint64
	ServiceMethod string
	Args          interface{}
	Reply         interface{}
	Error         error
	Done          chan *Call
}

func (call *Call) done() {
	call.Done <- call
}

type Client struct {
	cc codec.Codec
	// 发送锁
	sending *sync.Mutex
	// 客户端状态监视器锁
	mu *sync.Mutex
	// Option
	Option Option
	// 已发送但未回复的call
	pending map[uint64]*Call
	// closing
	closing bool
	// shutdown
	shutdown bool
	// 当前最近一次请求的seq
	seq uint64
	// 当前已回复、正在处理的 header
	header *codec.Header
}

// 异步结果
type clientResult struct {
	client *Client
	err    error
}

// 不知道为什么要声明这个接口，创建client也要抽象吗
type newClientFunc func(conn net.Conn, opt *Option) (client *Client, err error)

func DialHTTP(protocol string, address string) (*Client, error) {
	cl, err := dialTimeout(NewHttpClient, protocol, address, DefaultOption)
	if err != nil {
		log.Println("DialHTTP error,", err)
		return nil, err
	}
	return cl, nil
}

func Dial(protocol string, args ...interface{}) (*Client, error) {
	addr, opt := args[0].(string), DefaultOption

	if len(args) == 2 {
		opt = args[1].(*Option)
	}

	cl, err := dialTimeout(newClient, protocol, addr, opt)
	if err != nil {
		return nil, err
	}
	return cl, nil
}

func XDial(rpcAddr string, opt ...*Option) (*Client, error) {
	split := strings.Split(rpcAddr, "@")
	if len(split) != 2 {
		return nil, errors.New("rpcAddr is invalid: wrong format," + rpcAddr)
	}

	protocol, addr := split[0], split[1]

	var client *Client
	var err error

	switch protocol {
	case "http":
		client, err = DialHTTP("tcp", addr)
	default:
		if len(opt) == 1 {
			client, err = Dial(protocol, addr, opt)
		} else {
			client, err = Dial(protocol, addr, DefaultOption)
		}
	}
	if err != nil {
		return nil, err
	}

	return client, err
}

func dialTimeout(f newClientFunc, network, address string, opt *Option) (*Client, error) {
	clientCh := make(chan clientResult)
	go func() {
		var conn net.Conn
		var err error
		if opt.ConnectTimeout > 0 {
			conn, err = net.DialTimeout(network, address, opt.ConnectTimeout)
		} else if opt.ConnectTimeout == 0 {
			conn, err = net.Dial(network, address)
		} else {
			err = errors.New("opt.ConnectTimeout is invalid")
			return
		}
		defer func() {
			if err != nil {
				_ = conn.Close()
			}
		}()
		client, err := f(conn, opt)
		clientCh <- clientResult{
			client: client,
			err:    err,
		}
	}()
	if opt.ConnectTimeout == 0 {
		res := <-clientCh
		return res.client, res.err
	}
	select {
	case res := <-clientCh:
		return res.client, res.err
	case <-time.After(opt.ConnectTimeout):
		return nil, errors.New("connect timeout")
	}
}

const (
	defaultDebugPath = "/debug/geerpc"
	defaultRPCPath   = "/_geeprc_"
	connected        = "200 Connect to Gee RPC"
)

var NewClientFuncMap = map[string]newClientFunc{
	"http": NewHttpClient,
	"rpc":  newClient,
}

// NewHttpClient 方法仅仅是通过Connect http请求创建rpc链接
func NewHttpClient(conn net.Conn, opt *Option) (*Client, error) {
	_, err := io.WriteString(conn, fmt.Sprintf("CONNECT %s HTTP/1.0\n\n", defaultRPCPath))
	if err != nil {
		return nil, err
	}
	// 读响应
	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err != nil {
		return nil, err
	}

	if response.Status == connected {
		return newClient(conn, opt)
	}

	return nil, errors.New("response error, " + response.Status)
}

func newClient(conn net.Conn, opt *Option) (*Client, error) {
	ccFunc, ok := codec.NewCodecFuncMap[opt.CodecType]
	if !ok {
		return nil, errors.New("codecFunc is not found")
	}
	cc := ccFunc(conn)

	// 发送 option
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("client establish creation failure: ", err)
		return nil, err
	}
	client := Client{
		cc:      cc,
		sending: new(sync.Mutex),
		mu:      new(sync.Mutex),
		pending: make(map[uint64]*Call),
		closing: false,
	}
	go client.receive()
	return &client, nil
}

var ErrShutdown = errors.New("connection is shutdown")

func (c *Client) Close() error {
	// 关闭cc
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing {
		return ErrShutdown
	}
	c.closing = true
	return c.cc.Close()
}

func (c *Client) Call(ctx context.Context, method string, args interface{}, replyV interface{}) error {
	if c.closing {
		return ErrShutdown
	}
	call, err := c.send(method, args, replyV)
	if err != nil {
		return err
	}

	select {
	case <-call.Done:
		return call.Error
	case <-ctx.Done():
		c.removeCall(call.Seq)
		return errors.New("rpc client: call failed: " + ctx.Err().Error())
	}
}

func (c *Client) send(method string, args interface{}, replyV interface{}) (*Call, error) {
	call := &Call{
		ServiceMethod: method,
		Args:          args,
		Reply:         replyV,
		Done:          make(chan *Call),
	}

	c.sending.Lock()
	defer c.sending.Unlock()
	err := c.registerCall(call)
	if err != nil {
		return nil, err
	}

	header := &codec.Header{
		Seq:           call.Seq,
		ServiceMethod: call.ServiceMethod,
	}
	if err := c.cc.Write(header, args); err != nil {
		// 调用失败则删除call
		removeCall := c.removeCall(header.Seq)
		if removeCall != nil {
			removeCall.Error = err
			removeCall.done()
		}
		return nil, err
	}

	return call, nil
}

// 轮询处理call
func (c *Client) receive() {
	var err error
	for err == nil {
		header := &codec.Header{}
		err = c.cc.ReadHeader(header)
		if err != nil {
			log.Println("fatal error: ", err)
			break
		}
		call := c.removeCall(header.Seq)
		switch {
		case call == nil:
			// call不存在
			//err = errors.New("call is not found")
			err = c.cc.ReadBody(nil)
		case header.Error != "":
			call.Error = fmt.Errorf(header.Error)
			err = errors.New(header.Error)
			call.done()
		default:
			err = c.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body: " + err.Error())
			}
			call.done()
		}
	}

	c.terminalCall(err)
}

func (c *Client) registerCall(call *Call) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	call.Seq = c.seq

	if c.closing {
		return ErrShutdown
	}
	c.seq++
	c.pending[call.Seq] = call
	return nil
}

func (c *Client) removeCall(seq uint64) *Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	res := c.pending[seq]
	delete(c.pending, seq)
	return res
}

// 终止
func (c *Client) terminalCall(err error) {
	// 所有call删除并done
	for _, call := range c.pending {
		call.Error = err
		call.done()
	}
	c.pending = make(map[uint64]*Call)
}
