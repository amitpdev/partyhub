package ai

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"
)

type identity struct {
	Subject         string          `json:"sub"`
	Email           string          `json:"email"`
	Issuer          string          `json:"iss"`
	Audience        json.RawMessage `json:"aud"`
	AuthorizedParty string          `json:"azp"`
	Expires         int64           `json:"exp"`
	Issued          int64           `json:"iat"`
	Nonce           string          `json:"nonce"`
}

func (c *chatGPT) validateIdentity(ctx context.Context, token, clientID, nonce string) (identity, error) {
	invalid := errors.New("ChatGPT identity could not be verified. Try signing in again.")
	var claims identity
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(token) > 32768 {
		return claims, invalid
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	h, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(h, &header) != nil || header.Alg != "RS256" || header.Kid == "" {
		return claims, invalid
	}
	data, err := c.http(ctx, "GET", authIssuer+"/.well-known/jwks.json", nil, "")
	if err != nil {
		return claims, invalid
	}
	var jwks struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			Use string `json:"use"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if json.Unmarshal(data, &jwks) != nil {
		return claims, invalid
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return claims, invalid
	}
	verified := false
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	for _, key := range jwks.Keys {
		if key.Kid != header.Kid || key.Kty != "RSA" || (key.Use != "" && key.Use != "sig") || (key.Alg != "" && key.Alg != "RS256") {
			continue
		}
		n, e1 := base64.RawURLEncoding.DecodeString(key.N)
		e, e2 := base64.RawURLEncoding.DecodeString(key.E)
		if e1 != nil || e2 != nil || len(e) > 4 || len(e) == 0 || len(n) < 256 || len(n) > 1024 {
			continue
		}
		exponent := 0
		for _, b := range e {
			exponent = exponent*256 + int(b)
		}
		if exponent < 3 || exponent%2 == 0 {
			continue
		}
		if rsa.VerifyPKCS1v15(&rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}, crypto.SHA256, sum[:], signature) == nil {
			verified = true
			break
		}
	}
	if !verified {
		return claims, invalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || json.Unmarshal(payload, &claims) != nil {
		return claims, invalid
	}
	var audience string
	audOK := json.Unmarshal(claims.Audience, &audience) == nil && audience == clientID
	if !audOK {
		var audiences []string
		if json.Unmarshal(claims.Audience, &audiences) == nil {
			if len(audiences) > 1 && claims.AuthorizedParty != clientID {
				return claims, invalid
			}
			for _, a := range audiences {
				if a == clientID {
					audOK = true
				}
			}
		}
	}
	now := time.Now().Unix()
	if (claims.AuthorizedParty != "" && claims.AuthorizedParty != clientID) || !audOK || claims.Issuer != authIssuer || claims.Subject == "" || claims.Nonce != nonce || claims.Expires <= now || claims.Issued > now+60 {
		return claims, invalid
	}
	return claims, nil
}
