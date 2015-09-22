package server

import (
	"bytes"
	"encoding/gob"
	"log"
	"time"

	"github.com/garyburd/redigo/redis"
)

type User struct {
	ID           int
	Login        string
	PasswordHash string
	Salt         string

	LastLogin *LastLogin
}

type LastLogin struct {
	Login     string
	IP        string
	CreatedAt time.Time
}

func (u *User) getLastLogin() *LastLogin {
	conn := pool.Get()
	defer conn.Close()

	vs, err := redis.Values(conn.Do("lrange", "lastlogin:"+u.Login, 0, 1))
	if err != nil {
		log.Fatalln(err)
	}
	u.LastLogin = &LastLogin{}
	for _, v := range vs {
		r := bytes.NewReader(v.([]byte))
		dec := gob.NewDecoder(r)
		dec.Decode(u.LastLogin)
	}
	return u.LastLogin
}
