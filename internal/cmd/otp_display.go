package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/br4zz4/ward/internal/otp"
	"github.com/spf13/cobra"
)

// otpValue returns the value to show for a secret: a generated OTP code when
// the value is a TOTP secret and raw is not set, otherwise the value itself.
// Generation errors are reported on stderr and fall back to the original value.
// With verbose, OTP metadata is printed to stderr before the code.
func otpValue(value string, now time.Time, raw, verbose bool) string {
	if raw || !otp.IsOTP(value) {
		return value
	}
	code, err := otp.GenerateCode(value, now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ward: otp: %v\n", err)
		return value
	}
	if verbose {
		fmt.Fprintln(os.Stderr, otp.VerboseInfo(value, now))
	}
	return code
}

// flagRaw reports the persistent --raw flag.
func flagRaw(c *cobra.Command) bool {
	v, _ := c.Flags().GetBool("raw")
	return v
}

// flagVerbose reports the persistent -v/--verbose flag.
func flagVerbose(c *cobra.Command) bool {
	v, _ := c.Flags().GetBool("verbose")
	return v
}