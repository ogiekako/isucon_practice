package main

import (
	"database/sql"
	"fmt"
	"github.com/garyburd/redigo/redis"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"os"
	"strconv"
)

var db *sql.DB
var (
	UserLockThreshold int
	IPBanThreshold    int
)

func getEnv(key string, def string) string {
	v := os.Getenv(key)
	if len(v) == 0 {
		return def
	}

	return v
}
func main() {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Local",
		getEnv("ISU4_DB_USER", "root"),
		getEnv("ISU4_DB_PASSWORD", ""),
		getEnv("ISU4_DB_HOST", "localhost"),
		getEnv("ISU4_DB_PORT", "3306"),
		getEnv("ISU4_DB_NAME", "isu4_qualifier"),
	)

	var err error

	db, err = sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}

	UserLockThreshold, err = strconv.Atoi(getEnv("ISU4_USER_LOCK_THRESHOLD", "3"))
	if err != nil {
		panic(err)
	}

	IPBanThreshold, err = strconv.Atoi(getEnv("ISU4_IP_BAN_THRESHOLD", "10"))
	if err != nil {
		panic(err)
	}

	conn, err := redis.Dial("tcp", ":6379")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	_, err = conn.Do("flushdb")
	if err != nil {
		log.Fatalln(err)
	}

	rows, err := db.Query("select ip, login, succeeded from login_log order by id")
	if err != nil {
		log.Fatalln(err)
	}
	defer rows.Close()
	for rows.Next() {
		var remoteAddr, login string
		var succeeded bool
		err = rows.Scan(&remoteAddr, &login, &succeeded)
		if err != nil {
			log.Fatalln(err)
		}

		if succeeded {
			if remoteAddr != "" {
				_, err = conn.Do("hdel", "ip", remoteAddr)
				if err != nil {
					log.Fatalln(err)
				}
				_, err = conn.Do("srem", "banned_ip", remoteAddr)
				if err != nil {
					log.Fatalln(err)
				}
			}
			if login != "" {
				_, err = conn.Do("hdel", "user", login)
				if err != nil {
					log.Fatalln(err)
				}
				_, err = conn.Do("srem", "banned_user", login)
				if err != nil {
					log.Fatalln(err)
				}
			}
		} else {
			if remoteAddr != "" {
				n, err := redis.Int(conn.Do("hincrby", "ip", remoteAddr, 1))
				if err != nil {
					log.Fatalln(err)
				}
				if n >= IPBanThreshold {
					conn.Do("sadd", "banned_ip", remoteAddr)
				}
			}
			if login != "" {
				n, err := redis.Int(conn.Do("hincrby", "user", login, 1))
				if err != nil {
					log.Fatalln(err)
				}
				if n >= UserLockThreshold {
					conn.Do("sadd", "banned_user", login)
				}
			}
		}
	}
}
