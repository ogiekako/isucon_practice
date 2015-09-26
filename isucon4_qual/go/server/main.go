package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/draftcode/isucon_misc/grizzly"
	_ "github.com/go-sql-driver/mysql"
	ss "github.com/martini-contrib/sessions"
	"log"
	"net/http"
	"runtime"
	"strconv"
	"time"
)

var DB *sql.DB
var (
	histIndex *grizzly.KeyedHistogramMetric
)
var (
	UserLockThreshold int
	IPBanThreshold    int
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Local",
		getEnv("ISU4_DB_USER", "root"),
		getEnv("ISU4_DB_PASSWORD", ""),
		getEnv("ISU4_DB_HOST", "localhost"),
		getEnv("ISU4_DB_PORT", "3306"),
		getEnv("ISU4_DB_NAME", "isu4_qualifier"),
	)

	var err error

	DB, err = sql.Open("mysql", dsn)
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
}

func Main() {

	store := ss.NewCookieStore([]byte("secret-isucon"))

	m := http.NewServeMux()
	addResourceHandlers(m)

	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, "isucon")
		if err != nil {
			log.Fatalln(err)
		}
		notice := ""
		if flash, ok := session.Values["notice"]; ok {
			notice = flash.(string)
		}
		NewTemplate(w).index(notice)
	})

	m.HandleFunc("/init", func(w http.ResponseWriter, r *http.Request) {
		prepare()
		fmt.Fprintf(w, "done\n")
	})

	m.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		user, err := attemptLogin(r)

		notice := ""
		if err != nil || user == nil {
			switch err {
			case ErrBannedIP:
				notice = "You're banned."
			case ErrLockedUser:
				notice = "This account is locked."
			default:
				notice = "Wrong username or password"
			}

			session, err := store.Get(r, "isucon")
			if err != nil {
				log.Fatalln(err)
			}
			session.Values["notice"] = notice
			store.Save(r, w, session)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		session, err := store.Get(r, "isucon")
		if err != nil {
			log.Fatalln(err)
		}
		session.Values["user_id"] = user.Login
		store.Save(r, w, session)
		http.Redirect(w, r, "/mypage", http.StatusFound)
	})

	m.HandleFunc("/mypage", func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, "isucon")
		if err != nil {
			log.Fatalln(err)
		}
		var login string
		if v, ok := session.Values["user_id"]; ok {
			login = v.(string)
		}
		currentUser := getUser(login)

		if currentUser == nil {
			session.Values["notice"] = "You must be logged in"
			store.Save(r, w, session)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		store.Save(r, w, session)
		NewTemplate(w).mypage(getLastLogin(currentUser.Login))
	})

	m.HandleFunc("/report", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		bs, err := json.Marshal(map[string][]string{
			"banned_ips":   bannedIPs(),
			"locked_users": lockedUsers(),
		})
		if err != nil {
			log.Fatalln(err)
		}
		w.Write(bs)
	})

	http.Handle("/", grizzly.WrappedServeMux(m, grizzly.KeyedHistogram("http/")))

	log.Fatal(http.ListenAndServe(":80", nil))
}

func prepare() {
	initDB()
	rows, err := DB.Query("select ip, login, succeeded, created_at from login_log order by id")
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

		CreateLoginLog(succeeded, remoteAddr, login)
		if succeeded {
			UpdateLastLogin(remoteAddr, login, createdAt)
		}
	}
	rows2, err := DB.Query("select login, password_hash, salt from users")
	if err != nil {
		log.Fatalln(err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var user User
		err := rows2.Scan(&user.Login, &user.PasswordHash, &user.Salt)
		if err != nil {
			log.Fatalln(err)
		}
		AddUser(&user)
	}

}
