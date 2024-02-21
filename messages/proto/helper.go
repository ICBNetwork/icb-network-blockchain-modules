// Package proto defines the code for protocol buffer
package proto

import "google.golang.org/protobuf/proto"

// PayloadNoSig returns marshaled message without signature
func (m *Message) PayloadNoSig() ([]byte, error) {
	mm, _ := proto.Clone(m).(*Message)
	mm.Signature = nil

	raw, err := proto.Marshal(mm)
	if err != nil {
		return nil, err
	}

	return raw, nil
}
