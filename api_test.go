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

var _ = Describe("Book", func() {
	AfterEach(resetRedis)
	BeforeEach(func() {
		conn := RedisPool.Get()
		defer conn.Close()

		object := "foo"
		user := "jelder"
		for i := 0; i < 10; i++ {
			conn.Send("PFADD", "hll_"+object, user)
			conn.Send("INCR", "hits_"+object)
		}
	})

	Describe("Reading multiple objects at once", func() {
		var result TrackJson
		BeforeEach(func() {
			result, _ = GetMulti([]string{"foo"})
		})

		It("should find one unique", func() {
			Expect(result.Uniques).To(Equal(int64(1)))
		})
		It("should find ten visits", func() {
			Expect(result.Visits).To(Equal(int64(10)))
		})
	})
})
