package collect

import "testing"

func TestSmartctlInvocation(t *testing.T) {
	if !smartctlInvocation("smartctl", []string{"-H", "/dev/sda"}) {
		t.Fatal("direct smartctl invocation not detected")
	}
	if !smartctlInvocation("sudo", []string{"-n", "smartctl", "-H", "/dev/sda"}) {
		t.Fatal("sudo smartctl invocation not detected")
	}
	if smartctlInvocation("sudo", []string{"-n", "hdparm", "-C", "/dev/sda"}) {
		t.Fatal("hdparm invocation misdetected as smartctl")
	}
}

func TestSmartctlUsableOutput(t *testing.T) {
	good := []string{
		"SMART overall-health self-assessment test result: FAILED!",
		"SMART Health Status: OK",
		"SMART/Health Information (NVMe Log 0x02)",
		"194 Temperature_Celsius 0x0022 100 100 000 Old_age Always - 40",
		"Device is in STANDBY mode, exit(2)",
	}
	for _, text := range good {
		if !smartctlUsableOutput(text) {
			t.Fatalf("usable SMART output rejected: %q", text)
		}
	}

	bad := []string{
		"",
		"smartctl: Permission denied",
		"sudo: a password is required",
		"sudo: smartctl: command not found",
	}
	for _, text := range bad {
		if smartctlUsableOutput(text) {
			t.Fatalf("execution failure accepted as SMART output: %q", text)
		}
	}
}
