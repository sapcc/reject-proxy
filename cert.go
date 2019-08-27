package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"os"
	"time"

	"github.com/elazarl/goproxy"
)

func createCA() error {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1653),
		Subject: pkix.Name{
			Organization:       []string{"SAP SE"},
			OrganizationalUnit: []string{"CCloud Services MITM proxy"},
			Country:            []string{"DE"},
			Province:           []string{"Berlin"},
			Locality:           []string{"Berlin"},
			StreetAddress:      []string{"Rosenthaler Str. 30"},
			PostalCode:         []string{"10178"},
		},
		NotBefore:             time.Now().AddDate(0, 0, -1),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}
	keyOut, err := os.OpenFile(certDir+"/ca.key", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	err = pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	if err != nil {
		return err
	}
	keyOut.Close()

	pub := &priv.PublicKey
	caCert, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, priv)
	if err != nil {
		return err
	}
	certOut, err := os.Create(certDir + "/ca.crt")
	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: caCert})
	if err != nil {
		return err
	}
	certOut.Close()

	return nil
}

func loadCA() ([]byte, []byte, error) {
	certPEMBlock, err := ioutil.ReadFile(certDir + "/ca.crt")
	if err != nil {
		return nil, nil, err
	}

	keyPEMBlock, err := ioutil.ReadFile(certDir + "/ca.key")
	if err != nil {
		return nil, nil, err
	}

	return certPEMBlock, keyPEMBlock, nil
}

func setCA(caCert, caKey []byte) error {
	goproxyCa, err := tls.X509KeyPair(caCert, caKey)
	if err != nil {
		return err
	}
	if goproxyCa.Leaf, err = x509.ParseCertificate(goproxyCa.Certificate[0]); err != nil {
		return err
	}
	goproxy.GoproxyCa = goproxyCa
	goproxy.OkConnect = &goproxy.ConnectAction{Action: goproxy.ConnectAccept, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	goproxy.MitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	goproxy.HTTPMitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectHTTPMitm, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	goproxy.RejectConnect = &goproxy.ConnectAction{Action: goproxy.ConnectReject, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	return nil
}
