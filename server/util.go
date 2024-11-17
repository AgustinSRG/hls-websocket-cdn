// Utilities

package main

import (
	"os"
	"strconv"
	"strings"
)

func GetEnvBool(key string, defaultVal bool) bool {
	v := strings.ToUpper(os.Getenv(key))

	if v == "YES" {
		return true
	} else if v == "NO" {
		return false
	} else {
		return defaultVal
	}
}

func GetEnvString(key string, defaultVal string) string {
	v := os.Getenv(key)

	if v == "" {
		v = defaultVal
	}

	return v
}

func GetEnvInt(key string, defaultVal int) int {
	vString := os.Getenv(key)

	if vString == "" {
		return defaultVal
	}

	v, e := strconv.Atoi(vString)

	if e != nil {
		return defaultVal
	}

	return v
}

func GetEnvInt64(key string, defaultVal int64) int64 {
	vString := os.Getenv(key)

	if vString == "" {
		return defaultVal
	}

	v, e := strconv.ParseInt(vString, 10, 64)

	if e != nil {
		return defaultVal
	}

	return v
}
