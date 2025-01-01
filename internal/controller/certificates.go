package controller

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"time"

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CertInfo struct {
	Name          string
	Namespace     string
	NotBefore     time.Time
	NotAfter      time.Time
	DaysRemaining int
	SerialNumber  string
	IsExpired     bool
}

type CertController struct {
	clientset *kubernetes.Clientset
}

func NewCertController(clientset *kubernetes.Clientset) *CertController {
	return &CertController{
		clientset: clientset,
	}
}

// GetTLSCertificates retrieves all TLS certificates from secrets
func (c *CertController) GetTLSCertificates() ([]CertInfo, error) {
	secrets, err := c.clientset.CoreV1().Secrets("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %v", err)
	}

	var certInfos []CertInfo
	for _, secret := range secrets.Items {
		if secret.Type != corev1.SecretTypeTLS {
			continue
		}

		certData, ok := secret.Data["tls.crt"]
		if !ok {
			continue
		}

		block, _ := pem.Decode(certData)
		if block == nil {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}

		daysRemaining := int(time.Until(cert.NotAfter).Hours() / 24)

		certInfos = append(certInfos, CertInfo{
			Name:          secret.Name,
			Namespace:     secret.Namespace,
			NotBefore:     cert.NotBefore,
			NotAfter:      cert.NotAfter,
			DaysRemaining: daysRemaining,
			SerialNumber:  cert.SerialNumber.String(),
			IsExpired:     time.Now().After(cert.NotAfter),
		})
	}

	return certInfos, nil
}

// RenewCertificate creates a new CSR and handles the renewal process
func (c *CertController) RenewCertificate(namespace, name string) error {
	secret, err := c.clientset.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret: %v", err)
	}

	// Generate new private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %v", err)
	}

	// Create CSR template
	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   fmt.Sprintf("system:node:%s", name),
			Organization: []string{"system:nodes"},
		},
	}

	// Create CSR
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create CSR: %v", err)
	}

	// Encode CSR to PEM
	csrPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	})

	// Create CertificateSigningRequest
	csrName := fmt.Sprintf("%s-%s-%d", namespace, name, time.Now().Unix())
	csr := &certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: csrName,
		},
		Spec: certificatesv1.CertificateSigningRequestSpec{
			Request:           csrPEM,
			SignerName:        "kubernetes.io/kube-apiserver-client",
			ExpirationSeconds: int32Ptr(365 * 24 * 60 * 60), // 1 year
			Usages: []certificatesv1.KeyUsage{
				certificatesv1.UsageClientAuth,
			},
		},
	}

	// Submit CSR
	csr, err = c.clientset.CertificatesV1().CertificateSigningRequests().Create(context.TODO(), csr, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create CSR: %v", err)
	}

	// Approve CSR
	approval := &certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: csrName,
		},
		Status: certificatesv1.CertificateSigningRequestStatus{
			Conditions: []certificatesv1.CertificateSigningRequestCondition{
				{
					Type:    certificatesv1.CertificateApproved,
					Status:  corev1.ConditionTrue,
					Reason:  "AutoApproved",
					Message: "Automatically approved by certificate controller",
				},
			},
		},
	}

	_, err = c.clientset.CertificatesV1().CertificateSigningRequests().UpdateApproval(context.TODO(), csrName, approval, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to approve CSR: %v", err)
	}

	// Wait for certificate to be issued
	var certificate []byte
	for i := 0; i < 10; i++ {
		time.Sleep(time.Second)
		csr, err = c.clientset.CertificatesV1().CertificateSigningRequests().Get(context.TODO(), csrName, metav1.GetOptions{})
		if err != nil {
			continue
		}
		if csr.Status.Certificate != nil {
			certificate = csr.Status.Certificate
			break
		}
	}

	if certificate == nil {
		return fmt.Errorf("timeout waiting for certificate to be issued")
	}

	// Update secret with new certificate
	secret.Data["tls.crt"] = certificate
	secret.Data["tls.key"] = encodePrivateKeyToPEM(privateKey)

	_, err = c.clientset.CoreV1().Secrets(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update secret: %v", err)
	}

	return nil
}

// GetKubeconfigCertInfo gets information about the current kubeconfig certificate
func (c *CertController) GetKubeconfigCertInfo(certData string) (*CertInfo, error) {
	decoded, err := base64.StdEncoding.DecodeString(certData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode certificate: %v", err)
	}

	block, _ := pem.Decode(decoded)
	if block == nil {
		return nil, fmt.Errorf("failed to parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %v", err)
	}

	daysRemaining := int(time.Until(cert.NotAfter).Hours() / 24)

	return &CertInfo{
		Name:          "kubeconfig",
		NotBefore:     cert.NotBefore,
		NotAfter:      cert.NotAfter,
		DaysRemaining: daysRemaining,
		SerialNumber:  cert.SerialNumber.String(),
		IsExpired:     time.Now().After(cert.NotAfter),
	}, nil
}

// Helper function to encode private key to PEM
func encodePrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
}

func int32Ptr(i int32) *int32 {
	return &i
}
