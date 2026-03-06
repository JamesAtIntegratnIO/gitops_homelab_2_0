package main

// CertificateSpec represents a cert-manager Certificate resource spec.
type CertificateSpec struct {
	IsCA           bool             `json:"isCA,omitempty"`
	CommonName     string           `json:"commonName"`
	SecretName     string           `json:"secretName"`
	DNSNames       []string         `json:"dnsNames,omitempty"`
	IPAddresses    []string         `json:"ipAddresses,omitempty"`
	Usages         []string         `json:"usages,omitempty"`
	PrivateKey     *PrivateKeySpec  `json:"privateKey,omitempty"`
	IssuerRef      IssuerRef        `json:"issuerRef"`
	SecretTemplate *SecretTemplate  `json:"secretTemplate,omitempty"`
}

// PrivateKeySpec defines the private key algorithm and size for a certificate.
type PrivateKeySpec struct {
	Algorithm string `json:"algorithm"`
	Size      int    `json:"size"`
}

// IssuerRef identifies the cert-manager issuer for a certificate.
type IssuerRef struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Group string `json:"group"`
}

// SecretTemplate provides labels for the generated TLS secret.
type SecretTemplate struct {
	Labels map[string]string `json:"labels,omitempty"`
}

// IssuerSpec represents a cert-manager Issuer resource spec.
type IssuerSpec struct {
	SelfSigned *SelfSignedIssuer `json:"selfSigned,omitempty"`
	CA         *CAIssuer         `json:"ca,omitempty"`
}

// SelfSignedIssuer marks the issuer as self-signed.
type SelfSignedIssuer struct{}

// CAIssuer references a CA secret for signing.
type CAIssuer struct {
	SecretName string `json:"secretName"`
}
