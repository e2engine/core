package util

import "slices"

func IsStringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	leftClone := slices.Clone(left)
	rightClone := slices.Clone(right)

	slices.Sort(leftClone)
	slices.Sort(rightClone)

	for i := range leftClone {
		if leftClone[i] != rightClone[i] {
			return false
		}
	}

	return true
}
