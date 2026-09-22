package meowcaller

import "testing"

// liveOutgoingRelay is the relay block whatsapp-rust captured from a live outgoing
// call (wacore/src/voip_control/relay_parse.rs): one endpoint on the web client port
// against lower-RTT ones on 3478.
func liveOutgoingRelay() *relayData {
	ep := func(name string, id uint32, fna bool, token, auth uint32, ip string, port uint16) relayEndpoint {
		return relayEndpoint{
			relayID: id, relayName: name, tokenID: token, authTokenID: auth, isFNA: fna,
			addresses: []relayAddress{{ipv4: ip, port: port}},
		}
	}
	return &relayData{
		relayTokens: [][]byte{{0xA0}, {0xA1}, {0xA2}},
		endpoints: []relayEndpoint{
			ep("fimp3c01", 0, true, 0, 0, "170.78.54.98", webClientRelayPort),
			ep("for2c01", 1, false, 1, 1, "57.144.129.57", 3478),
			ep("bsb1c01", 2, false, 2, 1, "57.144.137.57", 3478),
		},
	}
}

// Selecting by tier alone would pick the lowest-RTT non-FNA endpoint, which is the
// one-way silent case: it carries our uplink and never forwards the peer's media.
func TestMediaRelayPrefersTheWebClientPortEndpoint(t *testing.T) {
	rd := liveOutgoingRelay()
	got := getMediaRelayEndpoint(rd)
	if got == nil || got.relayName != "fimp3c01" {
		t.Fatalf("must dial the relay on the web client port, got %+v", got)
	}
	if got.addresses[0].port != webClientRelayPort {
		t.Fatalf("port = %d, want %d", got.addresses[0].port, webClientRelayPort)
	}
}

// The preference must not override usability: an endpoint on the web client port whose
// token we do not hold is undialable, so a usable endpoint elsewhere still wins.
func TestWebClientPortPreferenceYieldsToUsability(t *testing.T) {
	rd := liveOutgoingRelay()
	rd.endpoints[0].tokenID = 9
	if got := getMediaRelayEndpoint(rd); got == nil || got.relayName != "for2c01" {
		t.Fatalf("got %+v, want for2c01", got)
	}
}

// Some blocks carry no endpoint on the web client port: fall back to the previous
// order instead of failing the call.
func TestFallsBackWhenNoEndpointUsesTheWebClientPort(t *testing.T) {
	rd := liveOutgoingRelay()
	rd.endpoints = rd.endpoints[1:]
	if got := getMediaRelayEndpoint(rd); got == nil || got.relayName != "for2c01" {
		t.Fatalf("got %+v, want for2c01", got)
	}
}

// With no token at all nothing is usable; the tiers run again without the filter so
// connectAndAllocate still reports the missing token instead of "no usable endpoint".
func TestNothingUsableStillPicksByTier(t *testing.T) {
	rd := liveOutgoingRelay()
	rd.relayTokens = nil
	if got := getMediaRelayEndpoint(rd); got == nil || got.relayName != "fimp3c01" {
		t.Fatalf("got %+v, want fimp3c01", got)
	}
	if got := getMediaRelayEndpoint(&relayData{}); got != nil {
		t.Fatalf("empty block must give nil, got %+v", got)
	}
}
