package client_test

import (
	"math/rand"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/mudler/edgevpn/api/client"
)

var testInstance = os.Getenv("TEST_INSTANCE")

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

var _ = Describe("Client", func() {
	c := NewClient(WithHost(testInstance))

	// Start the test suite only if we have some machines connected
	BeforeSuite(func() {
		Eventually(func() (int, error) {
			m, err := c.Machines()
			return len(m), err
		}, 100*time.Second, 1*time.Second).Should(BeNumerically(">=", 0))
	})

	Context("Operates blockchain", func() {
		var testBucket string

		AfterEach(func() {
			Eventually(c.GetBuckets, 100*time.Second, 1*time.Second).Should(ContainElement(testBucket))
			err := c.DeleteBucket(testBucket)
			Expect(err).ToNot(HaveOccurred())
			Eventually(c.GetBuckets, 100*time.Second, 1*time.Second).ShouldNot(ContainElement(testBucket))
		})

		BeforeEach(func() {
			testBucket = randStringBytes(10)
		})

		It("Puts string data", func() {
			err := c.Put(testBucket, "foo", "bar")
			Expect(err).ToNot(HaveOccurred())

			Eventually(c.GetBuckets, 100*time.Second, 1*time.Second).Should(ContainElement(testBucket))
			Eventually(func() ([]string, error) { return c.GetBucketKeys(testBucket) }, 100*time.Second, 1*time.Second).Should(ContainElement("foo"))

			Eventually(func() (string, error) {
				resp, err := c.GetBucketKey(testBucket, "foo")
				if err == nil {
					var r string
					resp.Unmarshal(&r)
					return r, nil
				}
				return "", err
			}, 100*time.Second, 1*time.Second).Should(Equal("bar"))

			m, err := c.Ledger()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(m) > 0).To(BeTrue())
		})

		It("Puts random data", func() {
			err := c.Put(testBucket, "foo2", struct{ Foo string }{Foo: "bar"})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() (string, error) {
				resp, err := c.GetBucketKey(testBucket, "foo2")
				if err == nil {
					var r struct{ Foo string }
					resp.Unmarshal(&r)
					return r.Foo, nil
				}

				return "", err
			}, 100*time.Second, 1*time.Second).Should(Equal("bar"))
		})
	})
})
