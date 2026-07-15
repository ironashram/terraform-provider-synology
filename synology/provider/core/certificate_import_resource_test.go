package core

import "testing"

func TestValidateCertificatePEM(t *testing.T) {
	valid := `-----BEGIN CERTIFICATE-----
MIIDDzCCAfegAwIBAgIUfnd9FPvqavo7BY8jUkBFEZqScnQwDQYJKoZIhvcNAQEL
BQAwFzEVMBMGA1UEAwwMdGVzdC5leGFtcGxlMB4XDTI2MDQwNTE5MjQyMloXDTI2
MDQwNjE5MjQyMlowFzEVMBMGA1UEAwwMdGVzdC5leGFtcGxlMIIBIjANBgkqhkiG
9w0BAQEFAAOCAQ8AMIIBCgKCAQEAv792UbaMgvGS3ezDGZNPdwskVjAph5HMbdaF
8T2OAmuZR8iRug8V3m8jiE98j1qSPBF63dJFmaBCesajoGe+n4tKV6iM28kKd1eI
dkIAa4j8n6/1A1J10mYXw6ZCGBZV/Z2TDGoYWD+5vi2Box0NNnPfBeMNYDP3/ijA
uDQuttYPsNMTKlP3c2nR/ilI1o+dszBzj1bG5z54wltw2zaeOVBVX4c2k6n1bn+l
CDkbuuV4mUjjAk43AQ5vlOVg+uQ8yONMNH/8VjQOZySovmeWdsENy4hreQZ2Y0DR
rZFq2lBElUfQmvbGgmvbUGzEU+tubIX7Z4xQ3Qbc3D1fBmwj4wIDAQABo1MwUTAd
BgNVHQ4EFgQU2KcbKQExOgZAjo8baU0uhqPKHDEwHwYDVR0jBBgwFoAU2KcbKQEx
OgZAjo8baU0uhqPKHDEwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOC
AQEAksmuJw3TdY5JWxa1CNmG07/tGlIDKDdoFgO0wNpxpEXxa0BeCzR2Ci5uF5fy
ualOZtKWCecQGx/wszszIpINvPtmFg9mT9EBN5HDly8/IEDrrtl1sKa0X+eZ/BF6
egHgk0ytWsG25+WDhsFm85mFnihiqmig1J/DAw4aLikSnzoUBSSXqUZAcwTDU/02
J4UeeVCwIGQhQxw+bQLozxEXU+1zkvXqbeKdPsGauUPxaMkdAGIAnFEOycxr8NEz
wmPLm/ARcCIAWOzWf3+wN5MFS1oXI3nmWGpO1Nkg7WayfsyIJo3X65eddWehvddC
dlN9Mecnv0H2AcxQxrfb9JQz/g==
-----END CERTIFICATE-----
`

	if err := validateCertificatePEM(valid, "certificate"); err != nil {
		t.Fatalf("expected valid certificate PEM, got error: %v", err)
	}
	if err := validateCertificatePEM("not-a-cert", "certificate"); err == nil {
		t.Fatal("expected invalid certificate PEM to fail")
	}
}

func TestValidatePrivateKeyPEM(t *testing.T) {
	valid := `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIBH8e1G7wYx2y5m9y9wW4slfW44Z8vF+0t1dIhU5c2UdoAoGCCqGSM49
AwEHoUQDQgAEewRpvVV9buwC6SAgT35B5iQ2JpLetS6Ta8XSihHe3WTSDcXjEJb8
ATwlsQFh9ybejQPh0/sWtb0PYF/41y/0Q==
-----END EC PRIVATE KEY-----
`

	if err := validatePrivateKeyPEM(valid); err == nil {
		t.Fatal("expected malformed EC private key PEM to fail")
	}

	valid = `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQD6cwyoyS9MhUlC
v1srO2Qd6hw2yYB9H9n1tFoZT3zh0+BTtPlqvGjufH6G+jD/adJzi10BGSAdoo6g
QBaIj++ImQxGc1dQc5sKXc5teLoI0lp4rWuieKkyV7bgidh+NROm4tW7x1YgnPZX
ygJyI072QtdgQXl3k5ufADG7n2AFD+a83H8XTur2qxGn8pY/+bexdFv+DE5jBqFa
2RglN6E466+vWXTjhBMWrMUR3pvN8MPmMXxMvmXrKE+g6u40qCMgfHdCqkfNNpJB
AbIYW/W2PASi6DPd7OJbRRqtD9h5pz50jdK5Zk90un0nLBKBPXn1HULICwhf66A1
xeqmoeZaAgMBAAECggEAQicP3+GO6zkRgZNpmjQe7YQDdyCjTiMQuuLHfoalGoVY
LRNvKcJsteVEh9UpAJZciV06P88eaJEqn3Ejj6inUeJ8V+RaHcRUW2KIiMzFxLpy
58F3RrgPf63eNbUsVTNff7kwh28ykVfoCENKz7dxyzKDn5XxhxL7sRKqzZo4PMBV
S5aXoaZySUdkGFUTkOcJCIZy9FHn5Vf3L7hIwrKyYVJZZzKzbwQ6vurIWBLL8GMD
ZhDJW60Trw7O3cu6UytzszbmWzxubUoil58x2oyS9MhUlCv1srO2Qd6hw2yYB9H9
n1tFoZT3zh0+BTtPlqvGjufH6G+jD/adJzi10BGSAQKBgQD+0svBLnBxG29+rsoX
2m6q+UpBAkLTlaX2UjsFvdcGaVbL50DJBSSTXobHxPPH/FkOFxHlkRQAt2bQVWwt
sY9ycYzaO+F6DRKCVh0b07XHtcwPa5RWPLXnw0lPwQKBgQD8LF8A3+yQUwpsOSJy
cmBHQYaWV7k/akdUSHhDuD1ynjUTduVdJN9WewtG/XAIN5eVw1sM+dAf5BgU984R
HVil2PASi6DPd7OJbRRqtD9h5pz50jdK5Zk90un0nLBKBPXn1HULICwhf66A1xeq
moeZaFQKBgQC+6zkRgZNpmjDoE7YQDdyCjTiMQuuLHfoalGoVYLRNvKcJsteVEh9
UpAJZciV06P88eaJEqn3Ejj6inUeJ8V+RaHcRUW2KIiMzFxLpy58F3RrgPf63eNb
UsVTNff7kwh28ykVfoCENKz7dxyzKDn5XxhxL7sRKqzZo4PMBVS5aXoQKBgQCf4q
CMgfHdCqkfNNpJBAbIYW/W2PASi6DPd7OJbRRqtD9h5pz50jdK5Zk90un0nLBKBP
Xn1HULICwhf66A1xeqmoeZa2RglN6E466+vWXTjhBMWrMUR3pvN8MPmMXxMvmXrK
E+g6u40qCMgfHdCqkfNNpJBAbIYW/W2PAQKBgQDC5sKXc5teLoI0lp4rWuieKkyV
7bgidh+NROm4tW7x1YgnPZXygJyI072QtdgQXl3k5ufADG7n2AFD+a83H8XTur2q
xGn8pY/+bexdFv+DE5jBqFa2RglN6E466+vWXTjhBMWrMUR3pvN8MPmMXxMvmXrK
E+g6u40qCMgfHdCqkQ==
-----END PRIVATE KEY-----
`
	if err := validatePrivateKeyPEM(valid); err == nil {
		t.Fatal("expected malformed PKCS#8 private key PEM to fail")
	}

	valid = `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC/v3ZRtoyC8ZLd
7MMZk093CyRWMCmHkcxt1oXxPY4Ca5lHyJG6DxXebyOIT3yPWpI8EXrd0kWZoEJ6
xqOgZ76fi0pXqIzbyQp3V4h2QgBriPyfr/UDUnXSZhfDpkIYFlX9nZMMahhYP7m+
LYGjHQ02c98F4w1gM/f+KMC4NC621g+w0xMqU/dzadH+KUjWj52zMHOPVsbnPnjC
W3DbNp45UFVfhzaTqfVuf6UIORu65XiZSOMCTjcBDm+U5WD65DzI40w0f/xWNA5n
JKi+Z5Z2wQ3LiGt5BnZjQNGtkWraUESVR9Ca9saCa9tQbMRT625shftnjFDdBtzc
PV8GbCPjAgMBAAECggEAEbf3jwV3ZoI8OBWw0aQzK6Tz7qL0s3pdkajJJ8mwXbjj
qSZ3kOHj+3H5rpbpw7Vy3eofmG/dzpxoiD/izufHTabpb8A7g/PH689C5Oqkb0tx
TLBNy8jK6m5Us9ehM+iceZseA3+qUD1TRKef2xrMJcP/T+PzUHh86heJ93ua9XoZ
bRf5BGsCA01EfIBJt3uXJPVEk2Z55zNfc3M1Zrrkd8okm+qE3z86SCQmT44iEra3
tPiQb0fwUbHNcMRqe8AA8YSUXA3dqJ9xZ9SE2jwtCsSpqZ7k4nFMaqN5ArBvRNqN
1pfy8hubsZB0GOzoqE017Lm86zgfbu9xbKPONRwuAQKBgQDnO8surVvzYoUUb7ZQ
igOMykgADH8bvDVXjHLph12JLxEXmrH5L25fGUlAVk/bjkabGuBgz8+i/34WPSyE
/YyL5JoovQpUAPLtrU+zBWzNxPg94rbb4xYRJS5lNXqBsBFcksqUwKsvB0XRXtum
dfJFpiROuiEPQdwLlEnxhB7lwwKBgQDUSQI5L1srBx3mvUD+u+vUA8aRlKMG6Yti
h0vqq2xkZfOkUY0svwdxf40IN02e0/wZiIrGLWfv69Syg0S6KjcHEAPm5/p9HDma
KYSdLEEnxE/lSYOBU2UdYexkxMtqIAX6j5pAgBAqXpEAapd5oYebVchpK68eIylz
GGKRUSpHYQKBgQDIUK5Vw2yyzZhH+fbQkp88qkfxcuHyXvs+2rb5w4CuRQ3jium+
2u4ciEVC7QLFSt2zpHbYp25S4E6UaW5Vz2igD+vUet+loiTQ7aDrjzmQkKAUzIBo
wLLvK2yj1M5J5wNDVQ8WCkrBtOUw2aIi9G5rE+DEKs5U71L23QGprjEuDQKBgEoI
zEW1Rk5TRRJbnnc4gp6GUpIjDFg0yu+pz8gf0MWS6M29w0Z/uNDUcxMSdneV5q3g
+MT0wPLjhGJddXKXlmlYJIQ7Exje5xfksuM9s9tyk4qbgMlxlCoTJKZgG7D/ShaA
ToOAJiMgp+FFS16X/vslh6dmHMSd7q69KmMTs3MBAoGAfnd5pwjhTFAqAZV9FkFC
szO4gXc2mr4K/dZkaGWA+QdS8vkPOZDNV/tWCSvqnSy1zbF9we+EfBnYF03hru91
GtQmnpT+WIqKP7llr1UQ1jNVNJTT4aepfeukGbtqBRM4OmJHc/juuQmHcho2YrGP
odt6jiqmHz3Z3AWu1lELKFw=
-----END PRIVATE KEY-----
`
	if err := validatePrivateKeyPEM(valid); err != nil {
		t.Fatalf("expected valid private key PEM, got error: %v", err)
	}
}

func TestFileNameFromPath(t *testing.T) {
	if got := fileNameFromPath("/docker/certbot/live/example.com/cert.pem"); got != "cert.pem" {
		t.Fatalf("unexpected file name: %q", got)
	}
}
