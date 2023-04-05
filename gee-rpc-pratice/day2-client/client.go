// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package geerpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"geerpc/codec"
	"log"
	"net"
	"sync"
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
	Option Options
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

func Dial(protocol string, serviceLocation string) (*Client, error) {
	conn, err := net.Dial(protocol, serviceLocation)
	if err != nil {
		return nil, err
	}
	client, err := newClient(conn, &Options{
		MagicNumber: MagicNumber,
		CodecType:   GobType,
	})
	// 启动发送轮询和接收轮询
	return client, err
}

func newClient(conn net.Conn, opt *Options) (*Client, error) {
	ccFunc, ok := NewCodecFuncMap[opt.CodecType]
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

func (c *Client) Call(method string, args string, replyV *string) error {
	if c.closing {
		return ErrShutdown
	}
	call, err := c.send(method, args, replyV)
	if err != nil {
		return err
	}

	called := <-call.Done

	if called.Error != nil {
		return called.Error
	}
	return nil
}

func (c *Client) send(method string, args string, replyV *string) (*Call, error) {
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
			err = c.cc.ReadBody(nil)
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
