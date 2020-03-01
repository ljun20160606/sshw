/*
 * Reference https://github.com/bramvdbogaerde/go-scp/blob/master/protocol.go
 */
package sshwctl

import (
	"bufio"
	"io"
)

type ResponseType = uint8

const (
	Ok      ResponseType = 0
	Warning ResponseType = 1
	Error   ResponseType = 2
)

// There are tree types of responses that the remote can send back:
// ok, warning and error
//
// The difference between warning and error is that the connection is not closed by the remote,
// however, a warning can indicate a file transfer failure (such as invalid destination directory)
// and such be handled as such.
//
// All responses except for the `Ok` type always have a message (although these can be empty)
//
// The remote sends a confirmation after every SCP command, because a failure can occur after every
// command, the response should be read and checked after sending them.
type Response struct {
	Type    ResponseType
	Message string
}

// Reads from the given reader (assuming it is the output of the remote) and parses it into a Response structure
func ParseResponse(reader io.Reader) (Response, error) {
	buffer := make([]uint8, 1)
	_, err := reader.Read(buffer)
	if err != nil {
		return Response{}, err
	}

	responseType := buffer[0]
	var message string
	if responseType > 0 {
		bufferedReader := bufio.NewReader(reader)
		message, err = bufferedReader.ReadString('\n')
		if err != nil {
			return Response{}, err
		}
	}

	return Response{responseType, message}, nil
}

func (r *Response) IsOk() bool {
	return r.Type == Ok
}

func (r *Response) IsWarning() bool {
	return r.Type == Warning
}

// Returns true when the remote responded with an error
func (r *Response) IsError() bool {
	return r.Type == Error
}

// Returns true when the remote answered with a warning or an error
func (r *Response) IsFailure() bool {
	return r.Type > 0
}

// Returns the message the remote sent back
func (r *Response) GetMessage() string {
	return r.Message
}
