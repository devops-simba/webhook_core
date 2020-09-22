package webhook_core

import (
	"strings"
)

// FindFirst find specified value in a list of strings
// return index of found item or -1 if value is not in the collection
func Find(collection []string, value string) int {
	for i := 0; i < len(collection); i++ {
		if collection[i] == value {
			return i
		}
	}
	return -1
}

// FindIgnoreCase find specified value in a list of string ignoring case of the strings
func FindIgnoreCase(collection []string, value string) int {
	value = strings.ToLower(value)
	for i := 0; i < len(collection); i++ {
		if strings.ToLower(collection[i]) == value {
			return i
		}
	}
	return -1
}

// Contains check whether a collection contains a value or not
func Contains(collection []string, value string) bool {
	return Find(collection, value) != -1
}

// ContainsIgnoreCase check whether a collection contains a value or not ignoring the case of the values
func ContainsIgnoreCase(collection []string, value string) bool {
	return FindIgnoreCase(collection, value) != -1
}
