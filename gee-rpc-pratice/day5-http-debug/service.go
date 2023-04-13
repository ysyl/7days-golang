package geerpc

import (
	"reflect"
)

type Service struct {
	name     string
	typ      reflect.Type
	instance reflect.Value
	method   map[string]*methodType
}
type methodType struct {
	method reflect.Method
	arg    reflect.Type
	replyV reflect.Type
	// 用于统计调用次数
	numCalls uint64
}

func (mt *methodType) newArgv() reflect.Value {
	var val reflect.Value
	if mt.arg.Kind() == reflect.Pointer {
		val = reflect.New(mt.arg.Elem())
	} else {
		val = reflect.New(mt.arg).Elem()
	}

	return val
}

func (mt *methodType) newReplyv() reflect.Value {
	replyv := reflect.New(mt.replyV.Elem())
	return replyv
}

func (mt *methodType) NumCalls() uint64 {
	return mt.numCalls
}

func newService(instancePtr interface{}) *Service {
	service := Service{}
	ptrValue := reflect.ValueOf(instancePtr)
	service.name = ptrValue.Elem().Type().Name()
	service.typ = ptrValue.Type()
	service.instance = ptrValue.Elem()

	// 注册方法
	service.registerMethods()
	return &service
}

func (s *Service) registerMethods() {
	s.method = make(map[string]*methodType)
	for i := 0; i < s.typ.NumMethod(); i++ {
		methodRType := s.typ.Method(i).Type
		mtype := methodType{}
		mtype.method = s.typ.Method(i)
		mtype.arg = methodRType.In(1)
		mtype.replyV = methodRType.In(2)
		mtype.numCalls = 0
		s.method[mtype.method.Name] = &mtype
	}
}

func (s *Service) call(mType *methodType, argv reflect.Value, replyv reflect.Value) error {
	method := mType.method
	argvArray := make([]reflect.Value, 3)
	argvArray[0] = s.instance.Addr()
	argvArray[1] = argv
	argvArray[2] = replyv
	err := method.Func.Call(argvArray)
	if !err[0].IsNil() {
		return err[0].Interface().(error)
	}

	mType.numCalls++

	return nil
}

func (s *Service) LoadMethod(methodName string) *methodType {
	return s.method[methodName]
}
