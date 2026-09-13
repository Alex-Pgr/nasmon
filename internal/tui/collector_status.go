package tui

import (
	"fmt"
	"time"

	"nasmon/internal/model"
)

func collectorIssue(status model.CollectorStatus, interval time.Duration) string {
	if !status.Enabled || status.LastAttempt.IsZero() {
		return ""
	}
	if status.LastSuccess.IsZero() {
		return "UNAVAILABLE"
	}
	age := time.Since(status.LastSuccess)
	if age < 0 {
		age = 0
	}
	if interval > 0 && age > 4*interval {
		return "STALE " + dur(age)
	}
	if status.LastAttempt.After(status.LastSuccess) {
		return "ERROR"
	}
	return ""
}

func collectorTitleSuffix(status model.CollectorStatus, interval time.Duration) string {
	issue := collectorIssue(status, interval)
	if issue == "" {
		return ""
	}
	return " " + yellow + "[" + issue + "]" + reset
}

func (r Renderer) collectorWarningRows(s model.Snapshot, landscape bool) []string {
	type item struct {
		name     string
		status   model.CollectorStatus
		interval time.Duration
	}
	items := []item{
		{"GPU", s.GPUCollector, r.Config.GPUInterval},
		{"Disk temp", s.DiskTempCollector, r.Config.DiskTempInterval},
		{"SMART", s.SMARTCollector, r.Config.SMARTInterval},
		{"systemd", s.SystemdCollector, r.Config.SystemdInterval},
	}
	rows := make([]string, 0, len(items))
	for _, item := range items {
		issue := collectorIssue(item.status, item.interval)
		if issue == "" {
			continue
		}
		text := fmt.Sprintf("%s%s %s%s", yellow, item.name, issue, reset)
		if landscape {
			rows = append(rows, " "+text)
		} else {
			rows = append(rows, white+"│ "+text)
		}
	}
	return rows
}
