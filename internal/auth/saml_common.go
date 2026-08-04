package auth

import (
	"net/url"
	"strings"
)

func extractSAMLFromPostData(postData string) string {
	if postData == "" {
		return ""
	}

	values, err := url.ParseQuery(postData)
	if err == nil {
		if saml := values.Get("SAMLResponse"); saml != "" {
			return saml
		}
	}

	const key = "SAMLResponse="
	idx := strings.Index(postData, key)
	if idx < 0 {
		return ""
	}
	saml := postData[idx+len(key):]
	if amp := strings.IndexByte(saml, '&'); amp >= 0 {
		saml = saml[:amp]
	}
	decoded, err := url.QueryUnescape(saml)
	if err == nil && decoded != "" {
		return decoded
	}
	return saml
}
