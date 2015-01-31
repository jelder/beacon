package main

import (
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/mux"
	"github.com/mholt/binding"
	// "io/ioutil"
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
	mget, err := redis.Values(conn.Do("MGET", "visits_"+objectId, "uniques_"+objectId, "hits_"+objectId))
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
	req.ParseForm()

	if len(req.Form["id"]) < 1 {
		http.Error(w, "Must pass id parameter (at least once)", http.StatusBadRequest)
		return
	}

	response, err := GetMulti(req.Form["id"])
	if err != nil {
		fmt.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	js, _ := json.MarshalIndent(response, "", "  ")
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func GetMulti(ids []string) (tj TrackJson, err error) {
	conn := redisPool.Get()
	defer conn.Close()

	script_args := redis.Args{}.Add(len(ids)).AddFlat(ids)
	// var visits, uniques int64
	script_result, err := redis.Values(multi_script.Do(conn, script_args...))
	if err != nil {
		return tj, err
	}
	_, err = redis.Scan(script_result, &tj.Visits, &tj.Uniques)
	if err != nil {
		return tj, err
	}
	return tj, err
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
