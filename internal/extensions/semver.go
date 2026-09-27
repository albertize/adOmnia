package extensions

import (
	"fmt"
	"strconv"
	"strings"
)

type semanticVersion struct{ major, minor, patch int }

func CheckEngineCompatibility(appVersion, constraint string) error {
	if appVersion == "" || appVersion == "dev" || appVersion == "unknown" {
		return nil
	}
	current, err := parseSemanticVersion(appVersion)
	if err != nil {
		return fmt.Errorf("invalid adOmnia application version %q", appVersion)
	}
	for _, token := range strings.Fields(constraint) {
		if token == "" {
			continue
		}
		operator := "="
		value := token
		for _, candidate := range []string{">=", "<=", "^", "~", ">", "<", "="} {
			if strings.HasPrefix(token, candidate) {
				operator, value = candidate, strings.TrimPrefix(token, candidate)
				break
			}
		}
		expected, err := parseSemanticVersion(value)
		if err != nil {
			return fmt.Errorf("invalid engine range token %q", token)
		}
		comparison := compareSemanticVersion(current, expected)
		matches := false
		switch operator {
		case ">=":
			matches = comparison >= 0
		case "<=":
			matches = comparison <= 0
		case ">":
			matches = comparison > 0
		case "<":
			matches = comparison < 0
		case "=":
			matches = comparison == 0
		case "^":
			upper := semanticVersion{major: expected.major + 1}
			if expected.major == 0 {
				upper = semanticVersion{major: 0, minor: expected.minor + 1}
			}
			matches = comparison >= 0 && compareSemanticVersion(current, upper) < 0
		case "~":
			upper := semanticVersion{major: expected.major, minor: expected.minor + 1}
			matches = comparison >= 0 && compareSemanticVersion(current, upper) < 0
		}
		if !matches {
			return fmt.Errorf("extension requires adOmnia %s; current version is %s", constraint, appVersion)
		}
	}
	return nil
}

func parseSemanticVersion(value string) (semanticVersion, error) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	value = strings.SplitN(value, "+", 2)[0]
	value = strings.SplitN(value, "-", 2)[0]
	parts := strings.Split(value, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return semanticVersion{}, fmt.Errorf("invalid semantic version")
	}
	values := [3]int{}
	for index, part := range parts {
		if part == "" || part == "*" || strings.EqualFold(part, "x") {
			part = "0"
		}
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return semanticVersion{}, fmt.Errorf("invalid semantic version")
		}
		values[index] = number
	}
	return semanticVersion{major: values[0], minor: values[1], patch: values[2]}, nil
}

func compareSemanticVersion(left, right semanticVersion) int {
	if left.major != right.major {
		if left.major < right.major {
			return -1
		}
		return 1
	}
	if left.minor != right.minor {
		if left.minor < right.minor {
			return -1
		}
		return 1
	}
	if left.patch != right.patch {
		if left.patch < right.patch {
			return -1
		}
		return 1
	}
	return 0
}
