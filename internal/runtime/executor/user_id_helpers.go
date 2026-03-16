package executor

import (
	"strings"

	"github.com/google/uuid"
)

func generateFakeUserID() string {
	return "user_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func isValidUserID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return strings.HasPrefix(value, "user_")
}
