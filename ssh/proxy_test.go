//go:build proxy

package ssh

import (
	"os"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestWithProxy(t *testing.T) {
	g := NewWithT(t)
	kh, err := ScanHostKey(os.Getenv("SSH_HOST"), 5*time.Second, []string{"ssh-rsa"}, false)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(kh)).To(ContainSubstring("ssh-rsa"))
}
