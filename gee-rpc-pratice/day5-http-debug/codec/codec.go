package codec

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
)

type Header struct {
	ServiceMethod string
	Seq           uint64
	Error         string
}

type Codec interface {
	Write(header *Header, body interface{}) error
	ReadHeader(header *Header) error
	ReadBody(interface{}) error
	Close() error
}

type NewCodecFunc func(conn net.Conn) Codec

type Type int

const (
	GobType  Type = 0
	JsonType Type = 1
)

var NewCodecFuncMap = map[Type]NewCodecFunc{
	GobType:  NewGobCodec,
	JsonType: NewJsonCodec,
}

type JsonCodec struct {
	conn net.Conn
	buf  *bufio.Writer
	enc  *json.Encoder
	dec  *json.Decoder
}

func (h *JsonCodec) Write(header *Header, body interface{}) error {
	defer func() {
		err := h.buf.Flush()
		if err != nil {
			_ = h.conn.Close()
			return
		}
	}()
	err := h.enc.Encode(header)
	if err != nil {
		log.Println("encode header error: ", err)
		return err
	}

	if body == nil {
		return nil
	}
	err = h.enc.Encode(body)
	if err != nil {
		log.Println("encode body error: ", err)
		return err
	}
	return nil
}

func (h *JsonCodec) ReadHeader(header *Header) error {
	err := h.dec.Decode(header)
	if err != nil {
		log.Println("decode header error:", err)
		return err
	}
	return nil
}

func (h *JsonCodec) ReadBody(body interface{}) error {
	err := h.dec.Decode(body)
	if err != nil {
		log.Println("decode header error:", err)
		return err
	}
	return nil
}

func (h *JsonCodec) Close() error {
	return h.Close()
}

func NewJsonCodec(conn net.Conn) Codec {
	bufWriter := bufio.NewWriter(conn)
	return &JsonCodec{
		conn: conn,
		buf:  bufWriter,
		dec:  json.NewDecoder(conn),
		enc:  json.NewEncoder(bufWriter),
	}
}
