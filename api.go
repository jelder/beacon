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

type TrackJSON struct {
	Visits  int64 `json:"visits"`
	Uniques int64 `json:"uniques"`
}

func (TrackJSON *TrackJSON) FieldMap() binding.FieldMap {
	return binding.FieldMap{
		&TrackJSON.Visits:  "visits",
		&TrackJSON.Uniques: "uniques",
	}
}

func apiHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	objectID := vars["objectID"]
	conn := RedisPool.Get()
	defer conn.Close()

	uniques, err := redis.Int64(conn.Do("PFCOUNT", "hll_"+objectID))
	if err != nil {
		fmt.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var migratedVisits, migratedUniques, visits int64
	mget, err := redis.Values(conn.Do("MGET", "visits_"+objectID, "uniques_"+objectID, "hits_"+objectID))
	if err != nil {
		fmt.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := redis.Scan(mget, &migratedVisits, &migratedUniques, &visits); err != nil {
		fmt.Print(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	visits += migratedVisits
	uniques += migratedUniques

	apiResponse := TrackJSON{Visits: visits, Uniques: uniques}
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

func GetMulti(ids []string) (tj TrackJSON, err error) {
	conn := RedisPool.Get()
	defer conn.Close()

	scriptArgs := redis.Args{}.Add(len(ids)).AddFlat(ids)
	// var visits, uniques int64
	scriptResult, err := redis.Values(multiScript.Do(conn, scriptArgs...))
	if err != nil {
		return tj, err
	}
	_, err = redis.Scan(scriptResult, &tj.Visits, &tj.Uniques)
	if err != nil {
		return tj, err
	}
	return tj, err
}

func apiWriteHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	objectID := vars["objectID"]
	TrackJSON := new(TrackJSON)
	if binding.Bind(req, TrackJSON).Handle(w) {
		return
	}
	fmt.Sprintf("%q\n", TrackJSON)
	conn := RedisPool.Get()
	defer conn.Close()
	_, err := conn.Do("MSET", "uniques_"+objectID, TrackJSON.Uniques, "visits_"+objectID, TrackJSON.Visits)
	if err != nil {
		fmt.Print(err)
	}
}
