package logstream

import "strings"

func ParseLevel(source, line string) string {
	up := strings.ToUpper(line)
	switch source {
	case "xray":
		switch {
		case strings.Contains(up, "[ERROR]"):
			return "ERROR"
		case strings.Contains(up, "[WARNING]"):
			return "WARN"
		case strings.Contains(up, "[DEBUG]"):
			return "DEBUG"
		default:
			return "INFO"
		}
	default:
		switch {
		case strings.Contains(up, "FATAL"), strings.Contains(up, "PANIC"), strings.Contains(up, "ERROR"):
			return "ERROR"
		case strings.Contains(up, "WARN"):
			return "WARN"
		case strings.Contains(up, "DEBUG"), strings.Contains(up, "TRACE"):
			return "DEBUG"
		default:
			return "INFO"
		}
	}
}
