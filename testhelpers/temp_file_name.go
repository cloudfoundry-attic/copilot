package testhelpers

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/gomega"
)

func TempFileName() string {
	f, err := ioutil.TempFile("", "test-config")
	Expect(err).NotTo(HaveOccurred())
	Expect(f.Close()).To(Succeed())
	path := f.Name()
	Expect(os.Remove(path)).To(Succeed())
	return path
}
