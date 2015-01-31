package main

import (
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/mux"
	"github.com/mholt/binding"
	"io/ioutil"
	"net/http"
)

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

func apiHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	objectId := vars["objectId"]
	conn := redisPool.Get()
	defer conn.Close()

	uniques, err := redis.Int64(conn.Do("PFCOUNT", "hll_"+objectId))
	if err != nil {
		fmt.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var migrated_visits, migrated_uniques, visits int64
	mget, err := redis.Values(conn.Do("MGET", "visits_"+objectId, "uniques_"+objectId, "str_"+objectId))
	if err != nil {
		fmt.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := redis.Scan(mget, &migrated_visits, &migrated_uniques, &visits); err != nil {
		fmt.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	visits += migrated_visits
	uniques += migrated_uniques

	apiResponse := TrackJson{Visits: visits, Uniques: uniques}
	js, _ := json.MarshalIndent(apiResponse, "", "  ")
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
		fmt.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// TODO also query unique
	apiResponse := TrackJson{Visits: visits, Uniques: 1}
	js, _ := json.MarshalIndent(apiResponse, "", "  ")
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
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
		fmt.Print(err)
	}
}
