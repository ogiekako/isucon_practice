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
)

var DB *sql.DB
var hist *grizzly.KeyedHistogramMetric
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

	m.Use(martini.Static("../public"))
	m.Use(render.Renderer(render.Options{
		Layout: "layout",
	}))

	m.Get("/", func(r render.Render, session sessions.Session) {
		s := grizzly.KeyedStopwatch(hist, "/")
		r.HTML(200, "index", map[string]string{"Flash": getFlash(session, "notice")})
		s.Close()
	})

	m.Post("/login", func(req *http.Request, r render.Render, session sessions.Session) {
		s := grizzly.KeyedStopwatch(hist, "/login")
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
			r.Redirect("/")
			return
		}

		session.Set("user_id", user.Login)
		r.Redirect("/mypage")
		s.Close()
	})

	m.Get("/mypage", func(r render.Render, session sessions.Session) {
		s := grizzly.KeyedStopwatch(hist, "/mypage")
		currentUser := getCurrentUser(session.Get("user_id"))

		if currentUser == nil {
			session.Set("notice", "You must be logged in")
			r.Redirect("/")
			return
		}

		currentUser.getLastLogin()
		r.HTML(200, "mypage", currentUser)
		s.Close()
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
