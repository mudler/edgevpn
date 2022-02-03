// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package client_test

import (
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/mudler/edgevpn/api/client"
)

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
