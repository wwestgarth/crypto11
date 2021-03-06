// Copyright 2018 Thales e-Security, Inc
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to
// the following conditions:
//
// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
// LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
// OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
// WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package crypto11

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"testing"
	"time"

	"github.com/miekg/pkcs11"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCertificate(t *testing.T) {
	skipTest(t, skipTestCert)

	ctx, err := ConfigureFromFile("config")
	require.NoError(t, err)

	defer func() {
		require.NoError(t, ctx.Close())
	}()

	id := randomBytes()
	label := randomBytes()

	cert := generateRandomCert(t)

	err = ctx.ImportCertificateWithLabel(id, label, cert)
	require.NoError(t, err)

	cert2, err := ctx.FindCertificate(nil, label, nil)
	require.NoError(t, err)
	require.NotNil(t, cert2)

	assert.Equal(t, cert.Signature, cert2.Signature)

	cert2, err = ctx.FindCertificate(nil, []byte("test2"), nil)
	require.NoError(t, err)
	assert.Nil(t, cert2)

	cert2, err = ctx.FindCertificate(nil, nil, cert.SerialNumber)
	require.NoError(t, err)
	require.NotNil(t, cert2)

	assert.Equal(t, cert.Signature, cert2.Signature)
}

// Test that provided attributes override default values
func TestCertificateAttributes(t *testing.T) {
	skipTest(t, skipTestCert)

	ctx, err := ConfigureFromFile("config")
	require.NoError(t, err)

	defer func() {
		require.NoError(t, ctx.Close())
	}()

	cert := generateRandomCert(t)

	// We import this with a different serial number, to test this is obeyed
	ourSerial := new(big.Int)
	ourSerial.Add(cert.SerialNumber, big.NewInt(1))

	derSerial, err := asn1.Marshal(ourSerial)
	require.NoError(t, err)

	template := NewAttributeSet()
	err = template.Set(CkaSerialNumber, derSerial)
	require.NoError(t, err)

	err = ctx.ImportCertificateWithAttributes(template, cert)
	require.NoError(t, err)

	// Try to find with old serial
	c, err := ctx.FindCertificate(nil, nil, cert.SerialNumber)
	assert.Nil(t, c)

	// Find with new serial
	c, err = ctx.FindCertificate(nil, nil, ourSerial)
	assert.NotNil(t, c)
}

func TestCertificateRequiredArgs(t *testing.T) {
	skipTest(t, skipTestCert)

	ctx, err := ConfigureFromFile("config")
	require.NoError(t, err)

	defer func() {
		require.NoError(t, ctx.Close())
	}()

	cert := generateRandomCert(t)

	val := randomBytes()

	err = ctx.ImportCertificateWithLabel(nil, val, cert)
	require.Error(t, err)

	err = ctx.ImportCertificateWithLabel(val, nil, cert)
	require.Error(t, err)

	err = ctx.ImportCertificateWithLabel(val, val, nil)
	require.Error(t, err)
}

func TestTrustedCertificate(t *testing.T) {
	skipTest(t, skipTestCert)
	skipTest(t, skipTestSecurityOfficer)

	// This is also checks that the config allows logging in as CKU_SO
	cfg, err := getConfig("config")
	require.NoError(t, err)
	cfg.LoginAsSecurityOfficer = true

	ctx, err := Configure(cfg)
	require.NoError(t, err)

	cert := generateRandomCert(t)

	template := NewAttributeSet()
	err = template.Set(CkaTrusted, true)
	require.NoError(t, err)

	err = ctx.ImportCertificateWithAttributes(template, cert)
	require.NoError(t, err)

}

func TestTrustedCertificateNotSO(t *testing.T) {

	// This is also checks that the config allows logging in as CKU_SO
	cfg, err := getConfig("config")
	require.NoError(t, err)
	cfg.LoginAsSecurityOfficer = false

	ctx, err := Configure(cfg)
	require.NoError(t, err)

	cert := generateRandomCert(t)

	template := NewAttributeSet()
	err = template.Set(CkaTrusted, true)
	require.NoError(t, err)

	err = ctx.ImportCertificateWithAttributes(template, cert)
	require.Error(t, err)

	p11Err, ok := err.(pkcs11.Error)
	require.True(t, ok)

	assert.Equal(t, pkcs11.Error(pkcs11.CKR_USER_NOT_LOGGED_IN), p11Err)

}

func generateRandomCert(t *testing.T) *x509.Certificate {
	serial, err := rand.Int(rand.Reader, big.NewInt(20000))
	require.NoError(t, err)

	ca := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: "Foo",
		},
		SerialNumber:          serial,
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	key, err := rsa.GenerateKey(rand.Reader, 4096)
	require.NoError(t, err)

	csr := &key.PublicKey
	certBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, csr, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certBytes)
	require.NoError(t, err)

	return cert
}
