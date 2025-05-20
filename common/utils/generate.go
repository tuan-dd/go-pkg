package utils

import "github.com/google/uuid"

func GenerateUUIDV7() string {
	newCid, _ := Retry(uuid.NewV7, 5)

	return newCid.String()
}
