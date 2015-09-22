package server

import (
	"database/sql"
	_ "encoding/json"
	"fmt"
	"github.com/draftcode/isucon_misc/grizzly"
	"github.com/go-martini/martini"
	_ "github.com/go-sql-driver/mysql"
	"github.com/martini-contrib/render"
	"github.com/martini-contrib/sessions"
	"log"
	"net/http"
	"strconv"
	"time"
)

var DB *sql.DB
var (
	hist      *grizzly.KeyedHistogramMetric
	histIndex *grizzly.KeyedHistogramMetric
)
var (
	UserLockThreshold int
	IPBanThreshold    int
)

func init() {
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
	hist = grizzly.KeyedHistogram("http/")

	m := martini.Classic()
	store := sessions.NewCookieStore([]byte("secret-isucon"))
	m.Use(sessions.Sessions("isucon_go_session", store))

	m.Use(render.Renderer(render.Options{
		Layout: "layout",
	}))
	m.Use(martini.Static("../public"))

	m.Get("/", func(w http.ResponseWriter, session sessions.Session) {
		s := grizzly.KeyedStopwatch(hist, "/")
		defer s.Close()
		flash := getFlash(session, "notice")
		NewTemplate(w).index(flash)
	})

	m.Get("/init", func(w http.ResponseWriter) {
		prepare()
		fmt.Fprintf(w, "done\n")
	})

	m.Post("/login", func(req *http.Request, w http.ResponseWriter, session sessions.Session) {
		s := grizzly.KeyedStopwatch(hist, "/login")
		defer s.Close()
		user, err := attemptLogin(req)

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

			session.Set("notice", notice)
			http.Redirect(w, req, "/", http.StatusFound)
			return
		}

		session.Set("user_id", user.Login)
		http.Redirect(w, req, "/mypage", http.StatusFound)
	})

	m.Get("/mypage", func(r *http.Request, w http.ResponseWriter, session sessions.Session) {
		s := grizzly.KeyedStopwatch(hist, "/mypage")
		defer s.Close()
		currentUser := getUser(session.Get("user_id").(string))

		if currentUser == nil {
			session.Set("notice", "You must be logged in")
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		NewTemplate(w).mypage(getLastLogin(currentUser.Login))
	})

	m.Get("/report", func(r render.Render) {
		s := grizzly.KeyedStopwatch(hist, "/report")
		ips := bannedIPs()
		users := lockedUsers()
		r.JSON(200, map[string][]string{
			"banned_ips":   ips,
			"locked_users": users,
		})
		s.Close()
	})

	http.Handle("/", m)

	log.Fatal(http.ListenAndServe(":8080", nil))
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
