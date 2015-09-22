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

	rows, err := DB.Query(
		"SELECT login, ip, created_at FROM login_log WHERE succeeded = 1 AND user_id = ? ORDER BY id DESC LIMIT 2",
		u.ID,
	)

	if err != nil {
		return nil
	}

	defer rows.Close()
	for rows.Next() {
		u.LastLogin = &LastLogin{}
		err = rows.Scan(&u.LastLogin.Login, &u.LastLogin.IP, &u.LastLogin.CreatedAt)
		if err != nil {
			u.LastLogin = nil
			return nil
		}
	}

	return u.LastLogin
}
