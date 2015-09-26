package server

import (
	"errors"
	"hash/fnv"
	"net/http"
	"sync"
	"time"

	"github.com/draftcode/isucon_misc/grizzly"
)

const mSize = 128

var (
	ErrBannedIP      = errors.New("Banned IP")
	ErrLockedUser    = errors.New("Locked user")
	ErrUserNotFound  = errors.New("Not found user")
	ErrWrongPassword = errors.New("Wrong password")

	hasUser   map[string]bool
	loginUser map[string]*User

	user  map[string]int
	ip    map[string]int
	userM = [mSize]sync.Mutex{}
	ipM   = [mSize]sync.Mutex{}

	lastLogin   map[string]*LastLogin
	secondLogin map[string]*LastLogin
	lastLoginM  = sync.Mutex{}
)

func init() {
	initDB()
}

func initDB() {
	hasUser = make(map[string]bool)
	loginUser = make(map[string]*User)
	user = make(map[string]int)
	ip = make(map[string]int)
	lastLogin = make(map[string]*LastLogin)
	secondLogin = make(map[string]*LastLogin)
}

func shard(s string) int {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int(h.Sum32() % mSize)
}

func CreateLoginLog(succeeded bool, remoteAddr, login string) {
	if succeeded {
		if remoteAddr != "" {
			i := shard(remoteAddr)
			ipM[i].Lock()
			ip[remoteAddr] = 0
			ipM[i].Unlock()
		}
		if login != "" {
			i := shard(login)
			userM[i].Lock()
			user[login] = 0
			userM[i].Unlock()
		}
	} else {
		if remoteAddr != "" {
			i := shard(remoteAddr)
			ipM[i].Lock()
			ip[remoteAddr]++
			ipM[i].Unlock()
		}
		if login != "" {
			i := shard(login)
			userM[i].Lock()
			user[login]++
			userM[i].Unlock()
		}
	}
}

func isLockedUser(login string) bool {
	return user[login] >= UserLockThreshold
}

func isBannedIP(i string) bool {
	return ip[i] >= IPBanThreshold
}

func UpdateLastLogin(ip, login string, createdAt time.Time) {
	lastLoginM.Lock()
	defer lastLoginM.Unlock()

	secondLogin[login] = lastLogin[login]
	lastLogin[login] = &LastLogin{
		Login:     login,
		IP:        ip,
		CreatedAt: createdAt,
	}
}

func getLastLogin(login string) *LastLogin {
	if secondLogin[login] != nil {
		return secondLogin[login]
	}
	return lastLogin[login]
}

func AddUser(u *User) {
	loginUser[u.Login] = u
}

func getUser(login string) *User {
	return loginUser[login]
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
		CreateLoginLog(succeeded, remoteAddr, loginName)
		if succeeded {
			UpdateLastLogin(remoteAddr, loginName, time.Now())
		}
		s.Close()
	}()

	s := grizzly.KeyedStopwatch(hDB, "/login:update")
	s.Close()
	user = getUser(loginName)
	if user == nil {
		return nil, ErrUserNotFound
	}

	if isBannedIP(remoteAddr) {
		return nil, ErrBannedIP
	}

	if isLockedUser(user.Login) {
		return nil, ErrLockedUser
	}

	if user.PasswordHash != calcPassHash(password, user.Salt) {
		return nil, ErrWrongPassword
	}

	succeeded = true
	return user, nil
}

func bannedIPs() []string {
	var res []string
	for i := range ip {
		if isBannedIP(i) {
			res = append(res, i)
		}
	}
	return res
}

func lockedUsers() []string {
	var res []string
	for i := range user {
		if isLockedUser(i) {
			res = append(res, i)
		}
	}
	return res
}
