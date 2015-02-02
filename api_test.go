package main_test

import (
	. "github.com/jelder/beacon"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestEnv(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API")
	BeforeSuite(resetRedis)
	AfterSuite(resetRedis)
}

func resetRedis() {
	conn := RedisPool.Get()
	defer conn.Close()
	conn.Do("FLUSHALL")
}

func trackSomeEvents() {
	conn := RedisPool.Get()
	defer conn.Close()
	events := []Event{
		{Object: "foo", User: "jelder"},
		{Object: "foo", User: "cmbt"},
		{Object: "bar", User: "jelder"},
		{Object: "bar", User: "cmbt"},
	}

	for i := 0; i < 10; i++ {
		for _, event := range events {
			event.Track(conn)
		}
	}
}

var _ = Describe("API", func() {
	BeforeEach(trackSomeEvents)
	AfterEach(resetRedis)

	Describe("api/v1/_multi", func() {
		var result TrackJson

		BeforeEach(func() {
			result, _ = GetMulti([]string{"foo", "bar"})
		})

		It("should find our uniques", func() {
			Expect(result.Uniques).To(Equal(int64(2)))
		})

		It("should find our visits", func() {
			Expect(result.Visits).To(Equal(int64(40)))
		})
	})
})
