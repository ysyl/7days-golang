package codec

type Codec interface {
	Write(header *Header, body interface{}) error
	ReadHeader(header *Header) error
	ReadBody(interface{}) error
	Close() error
}

type Header struct {
	ServiceMethod string
	Seq           uint64
	Error         string
}
