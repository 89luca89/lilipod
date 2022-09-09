package createutils

import (
	"math/rand"
	"time"
)

// GetRandomName returns a 12 string char of random characters.
func GetRandomName() string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

	rand.Seed(time.Now().UnixNano())

	s1 := make([]rune, 6)
	for i := range s1 {
		s1[i] = letters[rand.Intn(len(letters))]
	}

	s2 := make([]rune, 6)
	for i := range s2 {
		s2[i] = letters[rand.Intn(len(letters))]
	}

	result := append(s1, '_')
	result = append(result, s2...)

	return string(result)
}
