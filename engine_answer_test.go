package meowcaller

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	waBinary "go.mau.fi/whatsmeow/binary"
)

func testEngineWithIncomingCall() (*engine, *Call) {
	c := &Client{log: zerolog.Nop()}
	c.eng = newEngine(c)
	call := &Call{eng: c.eng, id: "CID", peer: peerJID(), phase: CallPhaseRinging}
	c.eng.calls["CID"] = &engineCall{
		call: call, direction: CallDirectionIncoming,
		from: peerJID(), creator: creatorJID(), callKey: make([]byte, 32),
	}
	return c.eng, call
}

// The callee <accept> goes out on Answer, before the media comes up, and advertises
// the MLOW capability without metadata (whatsapp-rust build_answer_signaling). The
// deferred path on the caller's first <mute_v2> must then stay silent.
func TestAnswerSendsAcceptBeforeMediaWithCapability(t *testing.T) {
	eng, call := testEngineWithIncomingCall()
	var sent []waBinary.Node
	eng.sendCallNode = func(_ context.Context, node waBinary.Node) error {
		sent = append(sent, node)
		return nil
	}

	if err := call.Answer(); err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if call.State() != CallPhaseConnecting {
		t.Fatalf("phase = %d, want connecting", call.State())
	}
	if len(sent) != 1 {
		t.Fatalf("sent %d nodes on Answer, want the accept alone", len(sent))
	}
	if id, _ := sent[0].Attrs["id"].(string); id == "" {
		t.Fatal("accept without a <call> id: the server drops it")
	}
	kids := sent[0].GetChildren()
	if len(kids) != 1 || kids[0].Tag != "accept" {
		t.Fatalf("accept envelope = %#v", sent[0])
	}
	var capability, metadata, audio bool
	for _, child := range kids[0].GetChildren() {
		switch child.Tag {
		case "capability":
			capability = true
		case "metadata":
			metadata = true
		case "audio":
			audio = true
		}
	}
	if !capability || metadata || !audio {
		t.Fatalf("audio accept children: capability=%t metadata=%t audio=%t", capability, metadata, audio)
	}
	if eng.calls["CID"].acceptPending {
		t.Fatal("accept still marked pending after Answer")
	}

	eng.onCallRaw(&waBinary.Node{
		Tag: "call", Attrs: waBinary.Attrs{"from": peerJID()},
		Content: []waBinary.Node{{
			Tag: "mute_v2", Attrs: waBinary.Attrs{"call-id": "CID", "mute-state": "0", "call-creator": creatorJID()},
		}},
	})
	if len(sent) != 1 {
		t.Fatalf("mute_v2 after Answer re-sent the accept: %d nodes", len(sent))
	}
}
