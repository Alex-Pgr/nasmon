package tui

import (
	"sort"
	"strings"

	"nasmon/internal/model"
)

type DockerSortMode int

const (
	DockerSortDefault DockerSortMode = iota
	DockerSortNameAsc
	DockerSortNameDesc
	DockerSortRAMDesc
	DockerSortRAMAsc
)

const mobileCompactMaxRows = 30

func dockerSortLabels(mode DockerSortMode) (string, string) {
	names, ram := "NAMES", "RAM"
	switch mode {
	case DockerSortNameAsc:
		names = "NAMES↑"
	case DockerSortNameDesc:
		names = "NAMES↓"
	case DockerSortRAMDesc:
		ram = "RAM↓"
	case DockerSortRAMAsc:
		ram = "RAM↑"
	}
	return names, ram
}

func containerSortKey(c model.Container) string {
	if c.ID != "" {
		return "id:" + c.ID
	}
	return "name:" + c.Name
}

func sortDockerContainers(containers, original []model.Container, mode DockerSortMode) []model.Container {
	out := append([]model.Container(nil), containers...)
	switch mode {
	case DockerSortNameAsc, DockerSortNameDesc:
		desc := mode == DockerSortNameDesc
		sort.SliceStable(out, func(i, j int) bool {
			li, lj := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
			if li == lj {
				if desc {
					return out[i].Name > out[j].Name
				}
				return out[i].Name < out[j].Name
			}
			if desc {
				return li > lj
			}
			return li < lj
		})
	case DockerSortRAMDesc, DockerSortRAMAsc:
		asc := mode == DockerSortRAMAsc
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].MemoryBytes != out[j].MemoryBytes {
				if asc {
					return out[i].MemoryBytes < out[j].MemoryBytes
				}
				return out[i].MemoryBytes > out[j].MemoryBytes
			}
			return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		})
	default:
		order := make(map[string]int, len(original))
		for i, c := range original {
			order[containerSortKey(c)] = i
		}
		sort.SliceStable(out, func(i, j int) bool {
			ii, iok := order[containerSortKey(out[i])]
			jj, jok := order[containerSortKey(out[j])]
			if iok != jok {
				return iok
			}
			if iok && ii != jj {
				return ii < jj
			}
			return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		})
	}
	return out
}

func configuredTerminalSize(r Renderer, rows, cols int) (int, int) {
	if r.Config.ForceCols > 0 && r.Config.ForceRows > 0 {
		return r.Config.ForceRows, r.Config.ForceCols
	}
	return rows, cols
}

func mobileCompactPortrait(r Renderer, rows, cols int) bool {
	rows, cols = configuredTerminalSize(r, rows, cols)
	return rows > 0 && rows <= mobileCompactMaxRows && cols > 0 && cols < 80
}

func effectiveLayout(r Renderer, rows, cols int) (int, int, bool) {
	rows, cols = configuredTerminalSize(r, rows, cols)
	if cols < r.Config.MinTermWidth {
		cols = r.Config.MinTermWidth
	}
	draw := cols - r.Config.RightMargin
	if draw < 32 {
		draw = 32
	}
	return rows, draw, rows <= 30 && cols >= 80
}

func (r Renderer) regularDockerPageSize(s model.Snapshot, rows int) int {
	collectorWarnings := r.collectorWarningRows(s, false)
	fullRows := regularFixedRows(s, 5) + len(collectorWarnings) + len(s.Containers)
	if rows <= 0 || fullRows <= rows {
		return len(s.Containers)
	}
	page := rows - regularFixedRows(s, 1) - len(collectorWarnings)
	if page < 0 {
		page = 0
	}
	if page > len(s.Containers) {
		page = len(s.Containers)
	}
	return page
}

func (r Renderer) DockerPageSize(s model.Snapshot, rows, cols int) int {
	mobileCompact := mobileCompactPortrait(r, rows, cols)
	effectiveRows, _, landscape := effectiveLayout(r, rows, cols)
	if mobileCompact {
		return ultraCompactDockerPageSize(len(s.Containers), effectiveRows)
	}
	if !landscape || effectiveRows <= 0 {
		return r.regularDockerPageSize(s, effectiveRows)
	}
	if effectiveRows <= ultraCompactMaxRows {
		return ultraCompactDockerPageSize(len(s.Containers), effectiveRows)
	}
	healthRows := len(buildHealthRows(s, true)) + len(r.collectorWarningRows(s, true))
	lowerRows := len(s.DiskUsage)
	if healthRows > lowerRows {
		lowerRows = healthRows
	}
	upperRows := landscapeSystemRows(s)
	if dockerRows := 1 + len(s.Containers); dockerRows > upperRows {
		upperRows = dockerRows
	}
	if 8+upperRows+lowerRows <= effectiveRows {
		return len(s.Containers)
	}
	page := effectiveRows - 7 - lowerRows
	if page < 0 {
		page = 0
	}
	if page > len(s.Containers) {
		page = len(s.Containers)
	}
	return page
}

func ClampDockerOffset(offset, total, page int) int {
	if total <= 0 || page <= 0 || page >= total {
		return 0
	}
	maxOffset := total - page
	if offset < 0 {
		return 0
	}
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func (r Renderer) RenderInteractive(s model.Snapshot, rows, cols int, mode DockerSortMode) string {
	return r.RenderInteractiveView(s, rows, cols, mode, 0)
}

func (r Renderer) RenderInteractiveView(s model.Snapshot, rows, cols int, mode DockerSortMode, dockerOffset int) string {
	mobileCompact := mobileCompactPortrait(r, rows, cols)
	effectiveRows, w, landscape := effectiveLayout(r, rows, cols)
	if mobileCompact || (landscape && effectiveRows <= ultraCompactMaxRows) {
		return r.renderUltraCompact(s, w, effectiveRows, mode, dockerOffset)
	}
	if landscape {
		return r.renderLandscape(s, w, effectiveRows, mode, dockerOffset)
	}
	return r.renderRegular(s, w, effectiveRows, mode, dockerOffset)
}

func terminalFrameLine(content string) string {
	return "\033[2K\r" + content + "\n"
}

func dockerSortHeader(lines []string, y int) (string, bool) {
	for _, offset := range []int{0, -1, 1, -2, 2} {
		candidate := y + offset
		if candidate < 1 || candidate > len(lines) {
			continue
		}
		line := lines[candidate-1]
		nameAt := cellIndex(line, "NAMES")
		ramAt := cellIndex(line, "RAM")
		statusAt := cellIndex(line, "STATUS")
		if nameAt >= 0 && ramAt > nameAt && statusAt > ramAt {
			return line, true
		}
	}
	return "", false
}

func DockerSortForClick(frame string, x, y int, current DockerSortMode) (DockerSortMode, bool) {
	if x < 1 || y < 1 {
		return current, false
	}
	lines := strings.Split(frame, "\n")
	line, ok := dockerSortHeader(lines, y)
	if !ok {
		return current, false
	}
	nameAt := cellIndex(line, "NAMES")
	ramAt := cellIndex(line, "RAM")
	statusAt := cellIndex(line, "STATUS")

	pos := x - 1
	next := current
	switch {
	case pos >= nameAt && pos < ramAt:
		if current == DockerSortNameAsc {
			next = DockerSortNameDesc
		} else {
			next = DockerSortNameAsc
		}
	case pos >= ramAt && pos < statusAt:
		if current == DockerSortRAMDesc {
			next = DockerSortRAMAsc
		} else {
			next = DockerSortRAMDesc
		}
	case pos >= statusAt && pos < cellWidth(line):
		next = DockerSortDefault
	default:
		return current, false
	}
	return next, next != current
}
