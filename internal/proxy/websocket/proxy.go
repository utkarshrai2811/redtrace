// Package websocket provides transparent pass-through tunneling for WebSocket
// (and other protocol-upgrade) connections that traverse the proxy. Frame-level
// inspection is a later phase; for now upgraded connections are relayed
// bidirectionally without modification.
package websocket

import (
	"io"
	"net"
)

// Tunnel relays bytes in both directions between client and upstream. When
// either direction closes, both connections are closed to unblock the other.
// It blocks until the relay completes.
func Tunnel(client, upstream net.Conn) {
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, client); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, upstream); done <- struct{}{} }()

	<-done
	_ = client.Close()
	_ = upstream.Close()
	<-done
}
