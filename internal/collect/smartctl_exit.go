package collect

import "strings"

func smartctlInvocation(name string, args []string) bool {
	if name == "smartctl" {
		return true
	}
	if name != "sudo" {
		return false
	}
	for _, arg := range args {
		if arg == "smartctl" {
			return true
		}
	}
	return false
}

func smartctlUsableOutput(text string) bool {
	low := strings.ToLower(text)
	for _, bad := range []string{
		"permission denied",
		"must be root",
		"operation not permitted",
		"password is required",
		"a terminal is required",
		"no tty present",
		"not allowed to execute",
		"command not found",
	} {
		if strings.Contains(low, bad) {
			return false
		}
	}
	for _, marker := range []string{
		"smart overall-health",
		"smart health status:",
		"smart/health information",
		"smart attributes data structure",
		"temperature_celsius",
		"temperature:",
		"device is in standby",
		"standby mode",
		"sleep mode",
		"low-power mode",
	} {
		if strings.Contains(low, marker) {
			return true
		}
	}
	return false
}
