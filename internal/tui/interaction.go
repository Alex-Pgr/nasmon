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

func effectiveLayout(r Renderer, rows, cols int) (int, int, bool) {
	if r.Config.ForceCols > 0 && r.Config.ForceRows > 0 {
		cols = r.Config.ForceCols
		rows = r.Config.ForceRows
	}
	if cols < r.Config.MinTermWidth {
		cols = r.Config.MinTermWidth
	}
	draw := cols - r.Config.RightMargin
	if draw < 32 {
		draw = 32
	}
	return rows, draw, rows <= 30 && cols >= 80
}

func (r Renderer) RenderInteractive(s model.Snapshot, rows, cols int, mode DockerSortMode) string {
	effectiveRows, w, landscape := effectiveLayout(r, rows, cols)
	if landscape {
		return r.renderLandscape(s, w, effectiveRows, mode)
	}
	return r.renderRegular(s, w, effectiveRows, mode)
}

func terminalFrameLine(content string) string {
	return "\033[2K\r" + content + "\n"
}

// DockerSortForClick maps an SGR mouse cell coordinate to the Docker header.
// The empty space within each column is intentionally clickable too, which is
// much easier to hit on a phone than the label glyphs alone.
func DockerSortForClick(frame string, x, y int, current DockerSortMode) (DockerSortMode, bool) {
	if x < 1 || y < 1 {
		return current, false
	}
	lines := strings.Split(frame, "\n")
	if y > len(lines) {
		return current, false
	}
	line := lines[y-1]
	nameAt := cellIndex(line, "NAMES")
	ramAt := cellIndex(line, "RAM")
	statusAt := cellIndex(line, "STATUS")
	if nameAt < 0 || ramAt <= nameAt || statusAt <= ramAt {
		return current, false
	}

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
