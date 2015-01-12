package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dchest/uniuri"
	"github.com/garyburd/redigo/redis"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

type ApiResponse struct {
	Visits  int64 `json:"visits"`
	Uniques int64 `json:"uniques"`
}

const cookieMaxAge = 60 * 60 * 60 * 24 * 30

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
	http.ServeFile(w, req, path.Join("images", "beacon.png"))
}

func indexHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Println(req.Method, req.URL)
	if req.URL.Path == "/" {
		http.ServeFile(w, req, path.Join("index.html"))
	} else {
		http.NotFound(w, req)
	}
}

func apiHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Println(req.Method, req.URL)
	switch req.Method {
	case "GET":
		apiGetHandler(w, req)
	case "POST":
		apiPostHandler(w, req)
	default:
		http.Error(w, "Expected GET or POST", http.StatusMethodNotAllowed)
	}
}

func apiObjectId(path string) (string, error) {
	path = strings.TrimSuffix(path, "/")
	elements := strings.SplitN(path, "/", 3)
	if len(elements) != 3 {
		return "", errors.New("Object Id not found in path")
	}
	return elements[2], nil
}

func apiGetHandler(w http.ResponseWriter, req *http.Request) {
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

// TODO support for backfilling values
func apiPostHandler(w http.ResponseWriter, req *http.Request) {
	objectId, err := apiObjectId(req.URL.Path)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
	}
	log.Print("Object ID: ", objectId)
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

var (
	pool *redis.Pool
)

func main() {
	redisServer, redisPassword := redisConfig()
	log.Print(redisServer, redisPassword)
	pool = newPool(redisServer, redisPassword)

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/beacon.png", beaconHandler)
	http.HandleFunc("/api/", apiHandler)

	log.Print("Listening on ", listenAddress())
	err := http.ListenAndServe(listenAddress(), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
