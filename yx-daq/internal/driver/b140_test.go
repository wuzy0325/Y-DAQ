package driver

import (
	"net"
	"testing"
	"time"
)

func TestB140DriverSendCommandReadsColonTerminatedResponse(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	d := NewB140Driver("", 0, 100)
	d.conn = client
	d.reader = nil
	d.connected.Store(true)

	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 16)
		n, _ := server.Read(buf)
		done <- string(buf[:n])
		_, _ = server.Write([]byte(":"))
	}()

	resp, err := d.SendCommand("SH")
	if err != nil {
		t.Fatalf("SendCommand failed: %v", err)
	}
	if resp != ":" {
		t.Fatalf("expected ':' response, got %q", resp)
	}
	select {
	case sent := <-done:
		if sent != "SH\r" {
			t.Fatalf("expected command with CR terminator, got %q", sent)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not receive command")
	}
}

func TestB140DriverSendCommandTreatsQuestionTerminatedResponseAsError(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	d := NewB140Driver("", 0, 100)
	d.conn = client
	d.reader = nil
	d.connected.Store(true)

	go func() {
		buf := make([]byte, 16)
		_, _ = server.Read(buf)
		_, _ = server.Write([]byte("BAD?"))
	}()

	resp, err := d.SendCommand("BAD")
	if err == nil {
		t.Fatal("expected error for question-terminated response")
	}
	if resp != "BAD?" {
		t.Fatalf("expected raw error response, got %q", resp)
	}
}
