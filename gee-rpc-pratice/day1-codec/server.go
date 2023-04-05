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
	"reflect"
	"sync"
)

const MagicNumber = 0x3bef5c

type Type int

const (
	GobType  Type = 0
	JsonType Type = 1
)

type Options struct {
	MagicNumber int
	CodecType   Type
}

type request struct {
	Header *codec.Header
	Body   interface{}
}

var DefaultOption = Options{
	MagicNumber: MagicNumber,
	CodecType:   GobType,
}

type Server struct {
}

func (s *Server) Accept(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go s.serveConn(conn)
	}
}

type NewCodecFunc func(conn net.Conn) codec.Codec

var NewCodecFuncMap = map[Type]NewCodecFunc{
	GobType: codec.NewGobCodec,
}

func (s *Server) serveConn(conn net.Conn) {
	// 校验建立options
	option, err := s.readOption(conn)
	if err != nil {
		return
	}

	// 根据codec类型解码和返回
	codecFunc, ok := NewCodecFuncMap[option.CodecType]
	if !ok {
		log.Println("codec is not found, codeType: " + string(option.CodecType))
		return
	}
	c := codecFunc(conn)
	s.serveCodec(c)
}

func (s *Server) readOption(conn net.Conn) (*Options, error) {
	var options = &Options{}
	err := json.NewDecoder(conn).Decode(options)
	if err != nil {
		return nil, err
	}
	if options.MagicNumber != MagicNumber {
		return nil, errors.New("magicnumber is invalid")
	}
	return options, nil
}

func (s *Server) serveCodec(c codec.Codec) {
	// 读取如果碰到多个request要并行，发送要串行
	// readQueue只是等待所有req都完成
	sendLock, readQueue := new(sync.Mutex), new(sync.WaitGroup)
	for {
		// 读取req
		readRequest, err := s.readRequest(c)
		if err != nil {
			if readRequest == nil {
				log.Println("error: ", err)
				continue
			}
			// 报错，返回
			s.sendResponse(c, readRequest.Header, readRequest.Body, readQueue, sendLock)
		}
		readQueue.Add(1)
		// 并发处理req，串行发送res
		go s.handleRequest(c, readRequest.Header, readRequest.Body, readQueue, sendLock)
	}
	fmt.Println("end service")
	readQueue.Wait()
	_ = c.Close()
}

var DefaultServer = &Server{}

func Accept(l net.Listener) {
	DefaultServer.Accept(l)
}

func (s *Server) readRequest(c codec.Codec) (*request, error) {
	h := &codec.Header{}
	err := c.ReadHeader(h)
	if err != nil {
		log.Println("error occur in readRequest, ", err)
		return nil, err
	}
	body := reflect.New(reflect.TypeOf(""))
	err = c.ReadBody(body.Interface())
	if err != nil {
		log.Println("readBody error：", err)
		return nil, err
	}

	return &request{
		Header: h,
		Body:   body,
	}, nil
}

func (s *Server) sendResponse(c codec.Codec, header *codec.Header, body interface{}, queue *sync.WaitGroup, sendLock *sync.Mutex) {
	method := header.ServiceMethod
	// 随便塞点返回
	sendLock.Lock()
	defer sendLock.Unlock()

	c.Write(header, fmt.Sprintf("method: %s, argv: %s, return: good", method, body))
}

func (s *Server) handleRequest(c codec.Codec, header *codec.Header, arg interface{}, queue *sync.WaitGroup, lock *sync.Mutex) {
	defer queue.Done()
	s.sendResponse(c, header, arg, queue, lock)
}
