// Copyright 2017 Capsule8 Inc. All rights reserved.

package stan

type Ack struct {
	Inbox    string
	Subject  string
	Sequence uint64
}
