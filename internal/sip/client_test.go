package sip

import "testing"

func TestRewriteTargetHost(t *testing.T) {
	got, err := rewriteTargetHost("<sip:1012@10.10.10.10>", "192.0.2.10:5060")
	if err != nil {
		t.Fatalf("rewriteTargetHost: %v", err)
	}
	want := "<sip:1012@192.0.2.10:5060>"
	if got != want {
		t.Fatalf("unexpected to header: got %q want %q", got, want)
	}
}

func TestBuildTargetURI(t *testing.T) {
	got, err := buildTargetURI("sip:2001@198.51.100.20:5080", "203.0.113.30:5070")
	if err != nil {
		t.Fatalf("buildTargetURI: %v", err)
	}
	want := "sip:2001@203.0.113.30:5070"
	if got != want {
		t.Fatalf("unexpected target uri: got %q want %q", got, want)
	}
}
