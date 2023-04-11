package codec

import (
	"bufio"
	"encoding/gob"
	"net"
)

type GobCodec struct {
	conn net.Conn
	buf  *bufio.Writer
	enc  *gob.Encoder
	dec  *gob.Decoder
}

func (g *GobCodec) Write(header *Header, body interface{}) (err error) {
	defer func() {
		_ = g.buf.Flush()
		if err != nil {
			_ = g.conn.Close()
		}
	}()
	err = g.enc.Encode(header)
	if err != nil {
		return err
	}
	if body == nil {
		return nil
	}
	err = g.enc.Encode(body)
	if err != nil {
		return err
	}
	return nil
}

func (g *GobCodec) ReadHeader(header *Header) error {
	return g.dec.Decode(header)
}

func (g *GobCodec) ReadBody(a interface{}) error {
	return g.dec.Decode(a)
}

func NewGobCodec(conn net.Conn) Codec {
	bufWriter := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  bufWriter,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(bufWriter),
	}
}

func (g *GobCodec) Close() error {
	return g.conn.Close()
}
