package id

import (
	"math/rand"
	"time"
)

var idLen = 10

func GenerateContainerId() string {
	letterBytes := "1234567890"
	rand.Seed(time.Now().UnixNano())

	b := make([]byte, idLen)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}

	return string(b)
}
