package cli

import "testing"

func TestCheckHidesOKByDefaultAndCanShowOK(t *testing.T) {
	ignoreOK := checkCmd.Flags().Lookup("ignore-ok")
	if ignoreOK == nil {
		t.Fatal("expected check --ignore-ok flag")
	}
	if ignoreOK.DefValue != "true" {
		t.Fatalf("expected check --ignore-ok to default true, got %q", ignoreOK.DefValue)
	}

	showOK := checkCmd.Flags().Lookup("show-ok")
	if showOK == nil {
		t.Fatal("expected check --show-ok flag")
	}
	if showOK.DefValue != "false" {
		t.Fatalf("expected check --show-ok to default false, got %q", showOK.DefValue)
	}
}
