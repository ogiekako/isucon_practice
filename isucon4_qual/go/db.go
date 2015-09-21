package main

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/garyburd/redigo/redis"
)

var (
	ErrBannedIP      = errors.New("Banned IP")
	ErrLockedUser    = errors.New("Locked user")
	ErrUserNotFound  = errors.New("Not found user")
	ErrWrongPassword = errors.New("Wrong password")

	conn redis.Conn
)

func init() {
	var err error
	conn, err = redis.Dial("tcp", ":6379")
	if err != nil {
		panic(err)
	}
}

func createLoginLog(succeeded bool, remoteAddr, login string, user *User) error {
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
	succ := 0
	if succeeded {
		succ = 1
	}

	var userId sql.NullInt64
	if user != nil {
		userId.Int64 = int64(user.ID)
		userId.Valid = true
	}

	_, err := db.Exec(
		"INSERT INTO login_log (`created_at`, `user_id`, `login`, `ip`, `succeeded`) "+
			"VALUES (?,?,?,?,?)",
		time.Now(), userId, login, remoteAddr, succ,
	)

	return err
}

func isLockedUser(user *User) bool {
	res, err := redis.Bool(conn.Do("sismember", "banned_user", user.Login))
	if err != nil {
		log.Fatalln(err)
	}
	return res
}

func isBannedIP(ip string) bool {
	res, err := redis.Bool(conn.Do("sismember", "banned_ip", ip))
	if err != nil {
		log.Fatalln(err)
	}
	return res
}

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
		createLoginLog(succeeded, remoteAddr, loginName, user)
	}()

	row := db.QueryRow(
		"SELECT id, login, password_hash, salt FROM users WHERE login = ?",
		loginName,
	)
	err := row.Scan(&user.ID, &user.Login, &user.PasswordHash, &user.Salt)

	switch {
	case err == sql.ErrNoRows:
		user = nil
	case err != nil:
		return nil, err
	}

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

func getCurrentUser(userId interface{}) *User {
	user := &User{}
	row := db.QueryRow(
		"SELECT id, login, password_hash, salt FROM users WHERE id = ?",
		userId,
	)
	err := row.Scan(&user.ID, &user.Login, &user.PasswordHash, &user.Salt)

	if err != nil {
		return nil
	}

	return user
}

func bannedIPs() []string {
	ips, err := redis.Strings(conn.Do("smembers", "banned_ip"))
	if err != nil {
		log.Fatalln(err)
	}
	return ips
}

func lockedUsers() []string {
	users, err := redis.Strings(conn.Do("smembers", "banned_user"))
	if err != nil {
		log.Fatalln(err)
	}
	return users
}
