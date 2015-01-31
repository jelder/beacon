package main

import (
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/dchest/uniuri"
	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/mux"
	"github.com/phyber/negroni-gzip/gzip"
	"github.com/rs/cors"
	"net/http"
	"runtime"
	"time"
)

const (
	cookieMaxAge = 60 * 60 * 60 * 24 * 30
)

var (
	redisPool    *redis.Pool
	events       chan Event
	multi_script = redis.NewScript(-1, fmt.Sprintf("%s", mustReadFile("assets/multi.lua")))
	beacon_png   = mustReadFile("assets/beacon.png")
)

type Event struct {
	Object string
	User   string
}

func (event *Event) Track(conn redis.Conn) {
	fmt.Print("Tracking ", event.User, " on ", event.Object)

	// http://godoc.org/github.com/garyburd/redigo/redis#hdr-Pipelining
	conn.Send("MULTI")

	// Track the number of unique visitors in a HyperLogLog
	// http://redis.io/commands/pfadd
	conn.Send("PFADD", "hll_"+event.Object, event.User)

	// Track the total number of visits in a simple key (stringy)
	// http://redis.io/commands/incr
	conn.Send("INCR", "hits_"+event.Object)

	_, err := conn.Do("EXEC")
	if err != nil {
		fmt.Print(err)
	}
}

func Tracker() {
	conn := redisPool.Get()
	defer conn.Close()

	for {
		event := <-events
		event.Track(conn)
	}
}

func init() {
	redisPool = redisSetup(redisConfig())
	events = make(chan Event, runtime.NumCPU()*100)
}

func main() {
	fmt.Println("Beacon running on", fmt.Sprintf("%d", runtime.NumCPU()), "CPUs")
	runtime.GOMAXPROCS(runtime.NumCPU())

	go Tracker()

	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "https://www.github.com/jelder/beacon", 302)
	})
	r.HandleFunc("/{objectId}.png", beaconHandler)
	r.HandleFunc("/api/v1/{objectId}", apiHandler).Methods("GET")
	r.HandleFunc("/api/v1/_multi", apiMultiHandler).Methods("POST")
	r.HandleFunc("/api/v1/{objectId}", apiWriteHandler).Methods("POST").Queries("key", ENV["SECRET_KEY"])

	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./assets/")))

	n := negroni.Classic()
	n.Use(gzip.Gzip(gzip.DefaultCompression))
	n.Use(cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
	}))
	n.UseHandler(r)
	n.Run(listenAddress())
}

func beaconHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	objectId := vars["objectId"]
	events <- Event{objectId, uid(w, req)}
	w.Header().Set("Content-Type", "image/png")
	w.Write(beacon_png)
}

func uid(w http.ResponseWriter, req *http.Request) string {
	cookie, err := req.Cookie("uid")
	if err != nil {
		switch err {
		case http.ErrNoCookie:
			uid := fmt.Sprintf("%s", uniuri.New())
			now := time.Now()
			new_cookie := &http.Cookie{Name: "uid", Value: uid, MaxAge: cookieMaxAge, Expires: now.Add(cookieMaxAge)}
			fmt.Print("Setting new cookie ", new_cookie)
			http.SetCookie(w, new_cookie)
			return uid
		default:
			fmt.Println(err)
		}
	}
	return cookie.Value
}
