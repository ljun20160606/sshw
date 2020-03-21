package sshwctl

import (
	"github.com/ljun20160606/eventbus"
)

type EventContext struct {
	Node       *Node
	Attachment map[string]interface{}
}

func NewEventContext(node *Node) *EventContext {
	return &EventContext{
		Node: node,
	}
}

func (e *EventContext) Put(key string, value interface{}) {
	if e.Attachment == nil {
		e.Attachment = make(map[string]interface{})
	}
	e.Attachment[key] = value
}

func (e *EventContext) Get(key string) (interface{}, bool) {
	if e.Attachment == nil {
		return nil, false
	}
	v, has := e.Attachment[key]
	return v, has
}

var bus = eventbus.New()

const (
	PostInitClientConfig = "PostInitClientConfig"
	PostSSHDial          = "PostSSHDial"
	PostNewSession       = "PostNewSession"
	OnStdout             = "OnStdout"
	OnStderr             = "OnStderr"
	PostShell            = "PostShell"
)
