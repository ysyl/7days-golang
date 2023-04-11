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
	"strings"
	"sync"
	"time"
)

const MagicNumber = 0x3bef5c

type Type int

const (
	GobType  Type = 0
	JsonType Type = 1
)

type Option struct {
	MagicNumber    int
	CodecType      Type
	ConnectTimeout time.Duration
	HandleTimeout  time.Duration
}

type request struct {
	Header  *codec.Header
	Body    interface{}
	Service *Service
	Method  *methodType
}

var DefaultOption = &Option{
	MagicNumber:    MagicNumber,
	CodecType:      GobType,
	ConnectTimeout: 0,
	HandleTimeout:  0,
}

type Server struct {
	serviceMap sync.Map
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
		return
	}
	c := codecFunc(conn)
	s.serveCodec(c)
}

func (s *Server) readOption(conn net.Conn) (*Option, error) {
	var options = &Option{}
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
		go s.handleRequest(readRequest, c, readRequest.Header, readRequest.Body, readQueue, sendLock)
	}
	fmt.Println("end service")
	readQueue.Wait()
	_ = c.Close()
}

var DefaultServer = &Server{
	serviceMap: sync.Map{},
}

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
	names := strings.Split(h.ServiceMethod, ".")
	service, _ := s.Load(names[0])
	method := service.LoadMethod(names[1])

	var argv = method.newArgv()
	argvi := argv.Interface()
	if argv.Type().Kind() != reflect.Ptr {
		argvi = argv.Addr().Interface()
	}
	err = c.ReadBody(argvi)
	if err != nil {
		log.Println("readBody error：", err)
		return nil, err
	}

	return &request{
		Header:  h,
		Body:    argv.Interface(),
		Method:  method,
		Service: service,
	}, nil
}

func (s *Server) handleRequest(request *request, c codec.Codec, header *codec.Header, arg interface{}, queue *sync.WaitGroup, lock *sync.Mutex) {
	defer queue.Done()
	service, method := request.Service, request.Method

	argV := reflect.ValueOf(arg)
	replyV := method.newReplyv()

	err := service.call(method, argV, replyV)
	if err != nil {
		header.Error = err.Error()
		s.sendResponse(c, header, nil, queue, lock)
		return
	}
	// 获取service
	s.sendResponse(c, header, replyV.Elem().Interface(), queue, lock)
}

func (s *Server) sendResponse(c codec.Codec, header *codec.Header, body interface{}, queue *sync.WaitGroup, sendLock *sync.Mutex) {
	// 随便塞点返回
	sendLock.Lock()
	defer sendLock.Unlock()

	err := c.Write(header, body)
	if err != nil {
		log.Println("sendResponse error : ", err)
		return
	}
}

func extractMethodName(serviceMethod string) (string, string, error) {
	split := strings.Split(serviceMethod, ".")
	return split[0], split[1], nil
}

func (s *Server) Register(instancePtr interface{}) error {
	service := newService(instancePtr)
	_, loaded := s.serviceMap.LoadOrStore(service.name, service)
	if !loaded {
		log.Println("store success, name: ", service.name)
	}

	return nil
}

func (s *Server) Load(serviceName string) (*Service, bool) {
	value, ok := s.serviceMap.Load(serviceName)
	if !ok {
		log.Println("service loaded failure, serviceName: ", serviceName)
		return nil, false
	}
	return value.(*Service), true
}

func (s *Server) LoadMethod(serviceName, methodName string) (*methodType, bool) {
	load, b := s.Load(serviceName)
	return load.LoadMethod(methodName), b
}

func Register(classPtr interface{}) error {
	return DefaultServer.Register(classPtr)
}
