package server

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/draftcode/isucon_misc/grizzly"
	"github.com/garyburd/redigo/redis"
)

var (
	ErrBannedIP      = errors.New("Banned IP")
	ErrLockedUser    = errors.New("Locked user")
	ErrUserNotFound  = errors.New("Not found user")
	ErrWrongPassword = errors.New("Wrong password")

	pool *redis.Pool
)

func newPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:     8,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", ":6379")
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("ping")
			return err
		},
	}
}

func init() {
	pool = newPool()
}

func CreateRedisLoginLog(succeeded bool, remoteAddr, login string) {
	conn := pool.Get()
	defer conn.Close()
	if succeeded {
		if remoteAddr != "" {
			conn.Do("hdel", "ip", remoteAddr)
			conn.Do("srem", "banned_ip", remoteAddr)
		}
		if login != "" {
			conn.Do("hdel", "user", login)
			conn.Do("srem", "banned_user", login)
		}
	} else {
		if remoteAddr != "" {
			n, _ := redis.Int(conn.Do("hincrby", "ip", remoteAddr, 1))
			if n >= IPBanThreshold {
				conn.Do("sadd", "banned_ip", remoteAddr)
			}
		}
		if login != "" {
			n, _ := redis.Int(conn.Do("hincrby", "user", login, 1))
			if n >= UserLockThreshold {
				conn.Do("sadd", "banned_user", login)
			}
		}
	}
}

func isLockedUser(user *User) bool {
	if user == nil {
		return false
	}
	conn := pool.Get()
	defer conn.Close()
	res, err := redis.Bool(conn.Do("sismember", "banned_user", user.Login))
	if err != nil {
		log.Fatalln(err)
	}
	return res
}

func isBannedIP(ip string) bool {
	conn := pool.Get()
	defer conn.Close()
	res, err := redis.Bool(conn.Do("sismember", "banned_ip", ip))
	if err != nil {
		log.Fatalln(err)
	}
	return res
}

func UpdateLastLogin(ip, login string, createdAt time.Time) {
	conn := pool.Get()
	defer conn.Close()

	lastLogin := LastLogin{
		Login:     login,
		IP:        ip,
		CreatedAt: createdAt,
	}
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(lastLogin)
	if err != nil {
		log.Fatalln(err)
	}
	_, err = conn.Do("lpush", "lastlogin:"+login, buf.Bytes())
	if err != nil {
		log.Fatalln(err)
	}
}

var hDB = grizzly.KeyedHistogram("/login")

func attemptLogin(req *http.Request) (*User, error) {
	succeeded := false
	user := &User{}

	loginName := req.PostFormValue("login")
	password := req.PostFormValue("password")

	remoteAddr := req.RemoteAddr
	if xForwardedFor := req.Header.Get("X-Forwarded-For"); len(xForwardedFor) > 0 {
		remoteAddr = xForwardedFor
	}

	defer func() {
		s := grizzly.KeyedStopwatch(hDB, "/login:createLog")
		defer s.Close()
		CreateRedisLoginLog(succeeded, remoteAddr, loginName)
		if succeeded {
			UpdateLastLogin(remoteAddr, loginName, time.Now())
		}
	}()

	s := grizzly.KeyedStopwatch(hDB, "/login:scanUser")
	row := DB.QueryRow(
		"SELECT id, login, password_hash, salt FROM users WHERE login = ?",
		loginName,
	)
	err := row.Scan(&user.ID, &user.Login, &user.PasswordHash, &user.Salt)
	s.Close()

	switch {
	case err == sql.ErrNoRows:
		user = nil
	case err != nil:
		return nil, err
	}

	s = grizzly.KeyedStopwatch(hDB, "/login:errorCheck")
	defer s.Close()
	if banned := isBannedIP(remoteAddr); banned {
		return nil, ErrBannedIP
	}

	if locked := isLockedUser(user); locked {
		return nil, ErrLockedUser
	}

	if user == nil {
		return nil, ErrUserNotFound
	}

	if user.PasswordHash != calcPassHash(password, user.Salt) {
		return nil, ErrWrongPassword
	}

	succeeded = true
	return user, nil
}

func getCurrentUser(login interface{}) *User {
	user := &User{}
	row := DB.QueryRow(
		"SELECT id, login, password_hash, salt FROM users WHERE login = ?",
		login,
	)
	err := row.Scan(&user.ID, &user.Login, &user.PasswordHash, &user.Salt)

	if err != nil {
		return nil
	}

	return user
}

func bannedIPs() []string {
	conn := pool.Get()
	defer conn.Close()
	ips, err := redis.Strings(conn.Do("smembers", "banned_ip"))
	if err != nil {
		log.Fatalln(err)
	}
	return ips
}

func lockedUsers() []string {
	conn := pool.Get()
	defer conn.Close()
	users, err := redis.Strings(conn.Do("smembers", "banned_user"))
	if err != nil {
		log.Fatalln(err)
	}
	return users
}
