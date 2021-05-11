package provisioning

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateRandomPassword(t *testing.T) {
	pwd1, err := generateRandomPassword()
	if err != nil {
		t.Errorf("Unexpected error while generating random password: %s", err)
	}
	if pwd1 == "" {
		t.Errorf("Expected a valid string but got null")
	}
	pwd2, err := generateRandomPassword()
	if err != nil {
		t.Errorf("Unexpected error while re-generating random password: %s", err)
	} else {
		assert.False(t, pwd1 == pwd2, "regenerated random password should not match pervious one")
	}
}

func TestGenerateTlsCertificate(t *testing.T) {
	cert, err := generateTlsCertificate("")
	if err != nil {
		t.Errorf("Unexpected error while generating a certificate: %s", err)
	} else {
		assert.NotEqual(t, cert.certificate, "", "empty certificate")
		assert.NotEqual(t, cert.privateKey, "", "empty private key")
	}

	expired, err := isTlsCertificateExpired(cert.certificate)
	if err != nil {
		t.Errorf("Unexpected error while checking a certificate: %s", err)
	} else {
		assert.False(t, expired, "new certificate is already expired")
	}
}

func TestGenerateTlsCertificateWithHost(t *testing.T) {
	cert, err := generateTlsCertificate("127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error while generating a certificate: %s", err)
	} else {
		assert.NotEqual(t, cert.certificate, "", "empty certificate")
		assert.NotEqual(t, cert.privateKey, "", "empty private key")
	}

	expired, err := isTlsCertificateExpired(cert.certificate)
	if err != nil {
		t.Errorf("Unexpected error while checking a certificate: %s", err)
	} else {
		assert.False(t, expired, "new certificate is already expired")
	}
}
