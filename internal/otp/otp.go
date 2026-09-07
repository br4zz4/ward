package otp

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// IsOTP returns true when val looks like a TOTP secret: an otpauth:// URI, or a
// bare base32 string of a typical TOTP secret length (16, 32 or 64 chars).
func IsOTP(val string) bool {
	if strings.HasPrefix(val, "otpauth://") {
		return true
	}
	switch len(val) {
	case 16, 32, 64:
	default:
		return false
	}
	for i := 0; i < len(val); i++ {
		c := val[i]
		if (c < 'A' || c > 'Z') && (c < '2' || c > '7') {
			return false
		}
	}
	return true
}

// GenerateCode produces the TOTP code for val at time t. otpauth:// URIs are
// parsed via pquerna/otp, respecting the URI's algorithm, digits and period;
// bare base32 secrets use the defaults (SHA1, 6 digits, 30s). Invalid input
// returns an error.
func GenerateCode(val string, t time.Time) (string, error) {
	if strings.HasPrefix(val, "otpauth://") {
		key, err := otp.NewKeyFromURL(val)
		if err != nil {
			return "", fmt.Errorf("parsing otpauth URI: %w", err)
		}
		alg, digits, period, err := uriParams(val)
		if err != nil {
			return "", err
		}
		return totp.GenerateCodeCustom(key.Secret(), t, totp.ValidateOpts{
			Period:    uint(period),
			Digits:    digits,
			Algorithm: alg,
		})
	}
	return totp.GenerateCode(val, t)
}

// VerboseInfo describes the OTP parameters of val at time t. otpauth:// URIs
// show issuer and account; bare secrets are annotated with (default).
func VerboseInfo(val string, t time.Time) string {
	var sb strings.Builder
	markDefault := true
	algName, digits, period := "SHA1", 6, 30
	issuer, account := "", ""
	if strings.HasPrefix(val, "otpauth://") {
		if key, err := otp.NewKeyFromURL(val); err == nil {
			alg, d, p, pErr := uriParams(val)
			if pErr == nil {
				algName, digits, period = alg.String(), int(d), p
			}
			markDefault = false
			issuer = key.Issuer()
			account = key.AccountName()
		}
	}
	if markDefault {
		fmt.Fprintf(&sb, "Algorithm: %s (default)\n", algName)
		fmt.Fprintf(&sb, "Digits: %d (default)\n", digits)
		fmt.Fprintf(&sb, "Period: %ds (default)\n", period)
	} else {
		if issuer != "" {
			fmt.Fprintf(&sb, "Issuer: %s\n", issuer)
		}
		if account != "" {
			fmt.Fprintf(&sb, "Account: %s\n", account)
		}
		fmt.Fprintf(&sb, "Algorithm: %s\n", algName)
		fmt.Fprintf(&sb, "Digits: %d\n", digits)
		fmt.Fprintf(&sb, "Period: %ds\n", period)
	}
	remaining := period - int(t.Unix()%int64(period))
	fmt.Fprintf(&sb, "Remaining: %ds", remaining)
	return sb.String()
}

// uriParams extracts algorithm, digits and period from an otpauth:// URI,
// applying the standard defaults when a parameter is absent.
func uriParams(val string) (alg otp.Algorithm, digits otp.Digits, period int, err error) {
	u, err := url.Parse(val)
	if err != nil {
		return otp.AlgorithmSHA1, otp.DigitsSix, 30, fmt.Errorf("parsing otpauth URI: %w", err)
	}
	q := u.Query()

	alg = otp.AlgorithmSHA1
	switch strings.ToUpper(q.Get("algorithm")) {
	case "":
	case "SHA1":
	case "SHA256":
		alg = otp.AlgorithmSHA256
	case "SHA512":
		alg = otp.AlgorithmSHA512
	case "MD5":
		alg = otp.AlgorithmMD5
	default:
		return otp.AlgorithmSHA1, otp.DigitsSix, 30, fmt.Errorf("unsupported algorithm %q in otpauth URI", q.Get("algorithm"))
	}

	digits = otp.DigitsSix
	if d := q.Get("digits"); d != "" {
		n, e := strconv.Atoi(d)
		if e != nil || (n != 6 && n != 8) {
			return otp.AlgorithmSHA1, otp.DigitsSix, 30, fmt.Errorf("invalid digits %q in otpauth URI", d)
		}
		digits = otp.Digits(n)
	}

	period = 30
	if p := q.Get("period"); p != "" {
		n, e := strconv.Atoi(p)
		if e != nil || n <= 0 {
			return otp.AlgorithmSHA1, otp.DigitsSix, 30, fmt.Errorf("invalid period %q in otpauth URI", p)
		}
		period = n
	}
	return alg, digits, period, nil
}