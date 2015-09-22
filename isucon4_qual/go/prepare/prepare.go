package main

import (
	"github.com/garyburd/redigo/redis"
	_ "github.com/go-sql-driver/mysql"
	"github.com/ogiekako/isucon_practice/isucon4_qual/go/server"
	"log"
	"time"
)

func main() {
	conn, err := redis.Dial("tcp", ":6379")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	_, err = conn.Do("flushdb")
	if err != nil {
		log.Fatalln(err)
	}

	rows, err := server.DB.Query("select ip, login, succeeded, created_at from login_log order by id")
	if err != nil {
		log.Fatalln(err)
	}
	defer rows.Close()
	for rows.Next() {
		var remoteAddr, login string
		var succeeded bool
		var createdAt time.Time
		err = rows.Scan(&remoteAddr, &login, &succeeded, &createdAt)
		if err != nil {
			log.Fatalln(err)
		}

		server.CreateRedisLoginLog(succeeded, remoteAddr, login)
		if succeeded {
			server.UpdateLastLogin(remoteAddr, login, createdAt)
		}
	}
}
