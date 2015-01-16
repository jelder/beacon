package main

import (
	"encoding/json"
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/dchest/uniuri"
	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/mux"
	"github.com/mholt/binding"
	"github.com/phyber/negroni-gzip/gzip"
	"github.com/rs/cors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"time"
)

const cookieMaxAge = 60 * 60 * 60 * 24 * 30

var (
	redisPool    = redisSetup(redisConfig())
	beacon_png   = mustReadFile("assets/beacon.png")
	multi_script = redis.NewScript(-1, fmt.Sprintf("%s", mustReadFile("assets/multi.lua")))
	events       = make(chan Event, runtime.NumCPU()*100)
	version      string
)

func mustReadFile(path string) []byte {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return b
}

type Event struct {
	Object string
	User   string
}

type TrackJson struct {
	Visits  int64 `json:"visits"`
	Uniques int64 `json:"uniques"`
}

func (trackJson *TrackJson) FieldMap() binding.FieldMap {
	return binding.FieldMap{
		&trackJson.Visits:  "visits",
		&trackJson.Uniques: "uniques",
	}
}

func uid(w http.ResponseWriter, req *http.Request) string {
	cookie, err := req.Cookie("uid")
	if err != nil {
		switch err {
		case http.ErrNoCookie:
			uid := fmt.Sprintf("%s", uniuri.New())
			now := time.Now()
			new_cookie := &http.Cookie{Name: "uid", Value: uid, MaxAge: cookieMaxAge, Expires: now.Add(cookieMaxAge)}
			log.Print("Setting new cookie ", new_cookie)
			http.SetCookie(w, new_cookie)
			return uid
		default:
			log.Fatal(err)
			return ""
		}
	}
	return cookie.Value
}

func track() {
	conn := redisPool.Get()
	defer conn.Close()

	for {
		event := <-events
		log.Print("Tracking ", event.User, " on ", event.Object)

		// http://godoc.org/github.com/garyburd/redigo/redis#hdr-Pipelining
		conn.Send("MULTI")

		// Track the number of unique visitors in a HyperLogLog
		// http://redis.io/commands/pfadd
		conn.Send("PFADD", "hll_"+event.Object, event.User)

		// Track the total number of visits in a simple key (stringy)
		// http://redis.io/commands/incr
		conn.Send("INCR", "str_"+event.Object)

		_, err := conn.Do("EXEC")
		if err != nil {
			log.Print(err)
		}
	}
}

func beaconHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	objectId := vars["objectId"]
	events <- Event{objectId, uid(w, req)}
	w.Header().Set("Content-Type", "image/png")
	w.Write(beacon_png)
}

func apiHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	objectId := vars["objectId"]
	conn := redisPool.Get()
	defer conn.Close()

	uniques, err := redis.Int64(conn.Do("PFCOUNT", "hll_"+objectId))
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var migrated_visits, migrated_uniques, visits int64
	mget, err := redis.Values(conn.Do("MGET", "visits_"+objectId, "uniques_"+objectId, "str_"+objectId))
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := redis.Scan(mget, &migrated_visits, &migrated_uniques, &visits); err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	visits += migrated_visits
	uniques += migrated_uniques

	apiResponse := TrackJson{Visits: visits, Uniques: uniques}
	js, _ := json.MarshalIndent(apiResponse, "", "  ")
	w.Header().Set("Server", "Beacon "+version)
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func apiMultiHandler(w http.ResponseWriter, req *http.Request) {
	conn := redisPool.Get()
	defer conn.Close()
	body, _ := ioutil.ReadAll(req.Body)
	fmt.Printf("%s\n", body)

	// TODO read from POST body
	test := []string{"foo", "bar", "baz", "blah"}

	keys := variadicScriptArgs(test)
	visits, err := redis.Int64(multi_script.Do(conn, keys...))
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// TODO also query unique
	apiResponse := TrackJson{Visits: visits, Uniques: 1}
	js, _ := json.MarshalIndent(apiResponse, "", "  ")
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func variadicScriptArgs(args []string) []interface{} {
	scriptArgs := make([]interface{}, len(args)+1)
	scriptArgs[0] = len(args) // First argument is length
	for i, v := range args {
		scriptArgs[i+1] = interface{}(v)
	}
	return scriptArgs
}

func apiWriteHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	objectId := vars["objectId"]
	trackJson := new(TrackJson)
	if binding.Bind(req, trackJson).Handle(w) {
		return
	}
	fmt.Sprintf("%q\n", trackJson)
	conn := redisPool.Get()
	defer conn.Close()
	_, err := conn.Do("MSET", "uniques_"+objectId, trackJson.Uniques, "visits_"+objectId, trackJson.Visits)
	if err != nil {
		log.Print(err)
	}
}

func listenAddress() string {
	string := os.Getenv("PORT")
	if string == "" {
		return ":8080"
	} else {
		return ":" + string
	}
}

func redisConfig() (string, string) {
	redis_provider := os.Getenv("REDIS_PROVIDER")
	if redis_provider == "" {
		redis_provider = "OPENREDIS_URL"
	}
	string := os.Getenv(redis_provider)
	if string != "" {
		url, err := url.Parse(string)
		password := ""
		if err != nil {
			panic(err)
		}
		if url.User != nil {
			password, _ = url.User.Password()
		}
		return url.Host, password
	} else {
		return "127.0.0.1:6379", ""
	}
}

func redisSetup(server, password string) *redis.Pool {
	log.Print("Connecting to Redis on ", server, password)
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", server)
			if err != nil {
				return nil, err
			}
			if password != "" {
				if _, err := c.Do("AUTH", password); err != nil {
					c.Close()
					return nil, err
				}
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

func main() {
	log.Print("Beacon " + version + " running on " + fmt.Sprintf("%d", runtime.NumCPU()) + "CPUs")
	runtime.GOMAXPROCS(runtime.NumCPU())

	go track()

	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "https://www.github.com/jelder/beacon", 302)
	})
	r.HandleFunc("/{objectId}.png", beaconHandler)
	r.HandleFunc("/api/v1/{objectId}", apiHandler).Methods("GET")
	r.HandleFunc("/api/v1/_multi", apiMultiHandler).Methods("POST")
	r.HandleFunc("/api/v1/{objectId}", apiWriteHandler).Methods("POST").Queries("key", os.Getenv("SECRET_KEY"))

	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./assets/")))

	n := negroni.Classic()
	n.Use(gzip.Gzip(gzip.DefaultCompression))
	n.Use(cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
	}))
	n.UseHandler(r)
	n.Run(listenAddress())
}
