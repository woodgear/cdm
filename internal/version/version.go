// Package version resolves CDM version metadata from Go build information.
package version

import (
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

var (
	// Value can still be overridden with ldflags, but normal go install does
	// not need that path. The default is resolved from Go build metadata.
	Value     = "auto"
	GitCommit = "unknown"
	GitBranch = "unknown"
	BuildDate = "auto"
)

type Info struct {
	Version   string
	GitCommit string
	GitBranch string
	BuildDate string
}

var (
	currentOnce sync.Once
	currentInfo Info

	pseudoVersionRE = regexp.MustCompile(`(?:^|[.-])(\d{14})-([0-9a-fA-F]{12,})$`)
	dateVersionRE   = regexp.MustCompile(`^v?(\d{4})\.(\d{1,2})\.(\d{1,2})(?:[.-].*)?$`)
)

func Current() Info {
	currentOnce.Do(func() {
		currentInfo = resolve()
	})
	return currentInfo
}

func resolve() Info {
	info := Info{
		Version:   strings.TrimSpace(Value),
		GitCommit: strings.TrimSpace(GitCommit),
		GitBranch: strings.TrimSpace(GitBranch),
		BuildDate: strings.TrimSpace(BuildDate),
	}

	if info.GitCommit == "" {
		info.GitCommit = "unknown"
	}
	if info.GitBranch == "" {
		info.GitBranch = "unknown"
	}

	buildInfo, ok := debug.ReadBuildInfo()
	if ok {
		settings := buildSettings(buildInfo.Settings)

		if shouldAuto(info.GitCommit) {
			info.GitCommit = firstNonEmpty(settings["vcs.revision"], pseudoCommit(buildInfo.Main.Version), "unknown")
		}

		if shouldAuto(info.BuildDate) {
			info.BuildDate = firstNonEmpty(settings["vcs.time"], pseudoBuildDate(buildInfo.Main.Version), "unknown")
		}

		if shouldAuto(info.Version) {
			info.Version = firstNonEmpty(
				dateFromRFC3339(settings["vcs.time"]),
				dateFromModuleVersion(buildInfo.Main.Version),
			)
		}
	}

	info.Version = firstNonEmpty(info.Version, "unknown")
	if shouldAuto(info.BuildDate) {
		info.BuildDate = "unknown"
	}

	return info
}

func buildSettings(settings []debug.BuildSetting) map[string]string {
	result := make(map[string]string, len(settings))
	for _, setting := range settings {
		result[setting.Key] = setting.Value
	}
	return result
}

func shouldAuto(value string) bool {
	switch strings.TrimSpace(value) {
	case "", "auto", "unknown", "(devel)":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func dateFromRFC3339(value string) string {
	if value == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return ""
	}
	return t.UTC().Format("2006.01.02")
}

func dateFromModuleVersion(value string) string {
	if value == "" || value == "(devel)" {
		return ""
	}
	if date := dateFromPseudoVersion(value); date != "" {
		return date
	}
	return dateFromDateVersion(value)
}

func dateFromPseudoVersion(value string) string {
	matches := pseudoVersionRE.FindStringSubmatch(value)
	if len(matches) != 3 {
		return ""
	}
	t, err := time.Parse("20060102150405", matches[1])
	if err != nil {
		return ""
	}
	return t.UTC().Format("2006.01.02")
}

func pseudoBuildDate(value string) string {
	matches := pseudoVersionRE.FindStringSubmatch(value)
	if len(matches) != 3 {
		return ""
	}
	t, err := time.Parse("20060102150405", matches[1])
	if err != nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func pseudoCommit(value string) string {
	matches := pseudoVersionRE.FindStringSubmatch(value)
	if len(matches) != 3 {
		return ""
	}
	return matches[2]
}

func dateFromDateVersion(value string) string {
	matches := dateVersionRE.FindStringSubmatch(value)
	if len(matches) != 4 {
		return ""
	}
	t, err := time.Parse("2006.1.2", strings.Join(matches[1:4], "."))
	if err != nil {
		return ""
	}
	return t.Format("2006.01.02")
}
