package server

import (
	"time"
)

type User struct {
	Login        string
	PasswordHash string
	Salt         string
}

type LastLogin struct {
	Login     string
	IP        string
	CreatedAt time.Time
}
