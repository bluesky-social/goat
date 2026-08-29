package main

import "testing"

func TestPortableFilenameComponentKeepsHandleReadable(t *testing.T) {
	got := portableFilenameComponent("jay.bsky.team")
	if got != "jay.bsky.team" {
		t.Fatalf("expected handle to be unchanged, got %q", got)
	}
}

func TestPortableFilenameComponentReplacesWindowsReservedCharacters(t *testing.T) {
	got := portableFilenameComponent(`did:plc:abc<def>"ghi/jkl\mno|pqr?stu*vw`)
	want := "did_plc_abc_def__ghi_jkl_mno_pqr_stu_vw"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestPortableFilenameComponentAvoidsWindowsReservedNames(t *testing.T) {
	got := portableFilenameComponent("CON.car")
	if got != "_CON.car" {
		t.Fatalf("expected reserved name to be prefixed, got %q", got)
	}
}

func TestPortableFilenameComponentHandlesEmptyAfterTrim(t *testing.T) {
	got := portableFilenameComponent("...")
	if got != "_" {
		t.Fatalf("expected empty filename fallback, got %q", got)
	}
}
