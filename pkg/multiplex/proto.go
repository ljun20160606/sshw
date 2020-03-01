package multiplex

import (
	"bufio"
	"encoding/json"
	"io"
)

const Delim = '\n'

type ProtoReader interface {
	Read(i interface{}) error
}

type ProtoWriter interface {
	Write(i interface{}) error
}

type JsonProtoReader struct {
	R io.Reader
}

func NewJsonProtoReader(Reader io.Reader) ProtoReader {
	return &JsonProtoReader{R: Reader}
}

func (j *JsonProtoReader) Read(i interface{}) error {
	newReader := bufio.NewReader(j.R)
	text, err := newReader.ReadBytes(Delim)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(text[:len(text)-1], i); err != nil {
		return err
	}
	return nil
}

type JsonProtoWriter struct {
	W io.Writer
}

func NewJsonProtoWriter(Writer io.Writer) ProtoWriter {
	return &JsonProtoWriter{W: Writer}
}

func (j *JsonProtoWriter) Write(i interface{}) error {
	if err := json.NewEncoder(j.W).Encode(i); err != nil {
		return err
	}
	_, _ = j.W.Write([]byte{Delim})
	return nil
}
