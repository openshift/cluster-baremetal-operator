/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provisioning

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"time"
)

type TlsCertificate struct {
	privateKey  string
	certificate string
}

func generateRandomPassword() (string, error) {
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789")
	length := 16
	buf := make([]rune, length)
	numChars := big.NewInt(int64(len(chars)))
	for i := range buf {
		c, err := rand.Int(rand.Reader, numChars)
		if err != nil {
			return "", err
		}
		buf[i] = chars[c.Uint64()]
	}
	return string(buf), nil
}

func generateSerialNumber() (*big.Int, error) {
	max := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, max)
}

func generateTlsCertificate() (TlsCertificate, error) {
	pkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return TlsCertificate{}, err
	}

	sn, err := generateSerialNumber()
	if err != nil {
		return TlsCertificate{}, err
	}

	notBefore := time.Now()
	// NOTE(dtantsur): is 10 years enough? let's assume so.
	notAfter := notBefore.Add(10 * 365 * 24 * time.Hour)

	// TODO(dtantsur): add IPAddresses with provisioning IPs
	cert := x509.Certificate{
		SerialNumber:          sn,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &cert, &cert, &pkey.PublicKey, pkey)
	if err != nil {
		return TlsCertificate{}, err
	}

	var buf bytes.Buffer
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return TlsCertificate{}, err
	}
	pemStr := buf.String()
	buf.Truncate(0)

	pkeyBytes, err := x509.MarshalPKCS8PrivateKey(pkey)
	if err != nil {
		return TlsCertificate{}, err
	}
	if err := pem.Encode(&buf, &pem.Block{Type: "PRIVATE KEY", Bytes: pkeyBytes}); err != nil {
		return TlsCertificate{}, err
	}
	keyStr := buf.String()

	return TlsCertificate{
		privateKey:  keyStr,
		certificate: pemStr,
	}, nil
}
