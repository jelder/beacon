package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/dchest/uniuri"
	"github.com/garyburd/redigo/redis"
	"github.com/phyber/negroni-gzip/gzip"
	"github.com/rs/cors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type ApiResponse struct {
	Visits  int64 `json:"visits"`
	Uniques int64 `json:"uniques"`
}

const cookieMaxAge = 60 * 60 * 60 * 24 * 30

var (
	pool *redis.Pool
	png  = mustReadFile("assets/beacon.png")
)

func mustReadFile(path string) []byte {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return b
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

func track(objectId string, uid string) {
	log.Print("Tracking ", uid, " on ", objectId)
	conn := pool.Get()
	defer conn.Close()

	// http://godoc.org/github.com/garyburd/redigo/redis#hdr-Pipelining
	conn.Send("MULTI")

	// Track the number of unique visitors in a HyperLogLog
	// http://redis.io/commands/pfadd
	conn.Send("PFADD", "hll_"+objectId, uid)

	// Track the total number of visits in a simple key (stringy)
	// http://redis.io/commands/incr
	conn.Send("INCR", "str_"+objectId)

	_, err := conn.Do("EXEC")
	if err != nil {
		log.Print(err)
	}
}

func beaconHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Println(req.Method, req.URL)
	query, _ := url.ParseQuery(req.URL.RawQuery)
	objectId := query.Get("id")
	if objectId != "" {
		go track(objectId, uid(w, req))
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}

func apiObjectId(path string) (string, error) {
	path = strings.TrimSuffix(path, "/")
	elements := strings.SplitN(path, "/", 3)
	if len(elements) != 3 {
		return "", errors.New("Object Id not found in path")
	}
	return elements[2], nil
}

func apiHandler(w http.ResponseWriter, req *http.Request) {
	objectId, err := apiObjectId(req.URL.Path)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	log.Print("Object ID: ", objectId)
	conn := pool.Get()
	defer conn.Close()
	visits, _ := redis.Int64(conn.Do("GET", "str_"+objectId))
	uniques, _ := redis.Int64(conn.Do("PFCOUNT", "hll_"+objectId))
	apiResponse := ApiResponse{Visits: visits, Uniques: uniques}
	js, err := json.Marshal(apiResponse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
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

func newPool(server, password string) *redis.Pool {
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
	redisServer, redisPassword := redisConfig()
	log.Print("Connecting to Redis on ", redisServer, redisPassword)
	pool = newPool(redisServer, redisPassword)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "https://www.github.com/jelder/beacon", 302)
	})
	mux.HandleFunc("/beacon.png", beaconHandler)
	mux.HandleFunc("/api/", apiHandler)

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
	})

	n := negroni.Classic()
	n.Use(gzip.Gzip(gzip.DefaultCompression))
	n.Use(c)
	n.UseHandler(mux)
	n.Run(listenAddress())
}
